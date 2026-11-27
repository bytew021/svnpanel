package svncfg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

type Revisions struct {
	Passwd string `json:"passwd"`
	Authz  string `json:"authz"`
}

type RawFileKind string

const (
	RawFilePasswd    RawFileKind = "passwd"
	RawFileAuthz     RawFileKind = "authz"
	backupTimeLayout             = "20060102T150405.000000000"
)

func ParseRawFileKind(value string) (RawFileKind, error) {
	switch RawFileKind(value) {
	case RawFilePasswd:
		return RawFilePasswd, nil
	case RawFileAuthz:
		return RawFileAuthz, nil
	default:
		return "", fmt.Errorf("%w: 不支持的文件 %s", ErrValidation, value)
	}
}

type State struct {
	Passwd    *PasswdDoc
	Authz     *AuthzDoc
	Revisions Revisions
	RawPasswd []byte
	RawAuthz  []byte
}

type SnapshotFileInfo struct {
	Exists bool  `json:"exists"`
	Size   int64 `json:"size"`
}

type SnapshotInfo struct {
	ID        string           `json:"id"`
	CreatedAt string           `json:"createdAt"`
	Passwd    SnapshotFileInfo `json:"passwd"`
	Authz     SnapshotFileInfo `json:"authz"`
}

type SnapshotContent struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Passwd    []byte `json:"-"`
	Authz     []byte `json:"-"`
}

type Manager struct {
	passwdPath string
	authzPath  string
	backupDir  string
	keep       int

	mu       sync.Mutex
	fileLock *flock.Flock
}

func NewManager(passwdPath, authzPath, backupDir string, keep int) *Manager {
	if keep <= 0 {
		keep = 50
	}
	return &Manager{
		passwdPath: filepath.Clean(passwdPath),
		authzPath:  filepath.Clean(authzPath),
		backupDir:  filepath.Clean(backupDir),
		keep:       keep,
		fileLock:   flock.New(filepath.Join(filepath.Clean(backupDir), ".svnpanel.lock")),
	}
}

func (m *Manager) Validate() error {
	if err := os.MkdirAll(m.backupDir, 0o700); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}
	for _, file := range []string{m.passwdPath, m.authzPath} {
		info, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("检查配置文件 %s 失败: %w", file, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s 不是普通文件", file)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("%s 权限过宽，请执行 chmod 600 %s", file, file)
		}
	}
	return nil
}

func (m *Manager) Read() (*State, error) {
	passwdData, err := os.ReadFile(m.passwdPath)
	if err != nil {
		return nil, fmt.Errorf("读取 passwd 失败: %w", err)
	}
	authzData, err := os.ReadFile(m.authzPath)
	if err != nil {
		return nil, fmt.Errorf("读取 authz 失败: %w", err)
	}
	return parseState(passwdData, authzData)
}

func parseState(passwdData, authzData []byte) (*State, error) {
	passwd, err := ParsePasswd(passwdData)
	if err != nil {
		return nil, fmt.Errorf("解析 passwd 失败: %w", err)
	}
	authz, err := ParseAuthz(authzData)
	if err != nil {
		return nil, fmt.Errorf("解析 authz 失败: %w", err)
	}
	return &State{
		Passwd: passwd, Authz: authz,
		Revisions: Revisions{Passwd: Revision(passwdData), Authz: Revision(authzData)},
		RawPasswd: passwdData, RawAuthz: authzData,
	}, nil
}

func (m *Manager) Apply(
	ctx context.Context,
	expected Revisions,
	mutate func(*PasswdDoc, *AuthzDoc) error,
	afterWrite func(backupDir string) error,
) (*State, error) {
	if err := os.MkdirAll(m.backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}
	if err := m.lock(ctx); err != nil {
		return nil, err
	}
	defer m.unlock()

	passwdData, err := os.ReadFile(m.passwdPath)
	if err != nil {
		return nil, fmt.Errorf("读取 passwd 失败: %w", err)
	}
	authzData, err := os.ReadFile(m.authzPath)
	if err != nil {
		return nil, fmt.Errorf("读取 authz 失败: %w", err)
	}
	currentRevisions := Revisions{Passwd: Revision(passwdData), Authz: Revision(authzData)}
	if expected != currentRevisions {
		return nil, fmt.Errorf("%w: 期望 %+v，当前 %+v", ErrConflict, expected, currentRevisions)
	}
	state, err := parseState(passwdData, authzData)
	if err != nil {
		return nil, err
	}
	if err := mutate(state.Passwd, state.Authz); err != nil {
		return nil, err
	}
	newPasswd := state.Passwd.Doc.Bytes()
	newAuthz := state.Authz.Doc.Bytes()
	passwdChanged := !equalBytes(passwdData, newPasswd)
	authzChanged := !equalBytes(authzData, newAuthz)
	if !passwdChanged && !authzChanged {
		return state, nil
	}

	// Validate the complete generated files before touching the originals.
	if passwdChanged {
		if _, err := ParsePasswd(newPasswd); err != nil {
			return nil, fmt.Errorf("生成后的 passwd 校验失败: %w", err)
		}
	}
	if authzChanged {
		if _, err := ParseAuthz(newAuthz); err != nil {
			return nil, fmt.Errorf("生成后的 authz 校验失败: %w", err)
		}
	}
	passwdInfo, err := os.Stat(m.passwdPath)
	if err != nil {
		return nil, err
	}
	authzInfo, err := os.Stat(m.authzPath)
	if err != nil {
		return nil, err
	}
	backupDir, err := m.createBackup(passwdData, authzData)
	if err != nil {
		return nil, err
	}

	rollback := func() {
		if passwdChanged {
			_ = writeAtomic(m.passwdPath, passwdData, passwdInfo.Mode().Perm())
		}
		if authzChanged {
			_ = writeAtomic(m.authzPath, authzData, authzInfo.Mode().Perm())
		}
	}
	if passwdChanged {
		if err := writeAtomic(m.passwdPath, newPasswd, passwdInfo.Mode().Perm()); err != nil {
			rollback()
			return nil, fmt.Errorf("写入 passwd 失败: %w", err)
		}
	}
	if authzChanged {
		if err := writeAtomic(m.authzPath, newAuthz, authzInfo.Mode().Perm()); err != nil {
			rollback()
			return nil, fmt.Errorf("写入 authz 失败: %w", err)
		}
	}
	if afterWrite != nil {
		if err := afterWrite(backupDir); err != nil {
			rollback()
			return nil, fmt.Errorf("发布后审计失败，已回滚: %w", err)
		}
	}
	m.pruneBackups()
	updated, err := parseState(newPasswd, newAuthz)
	if err != nil {
		rollback()
		return nil, err
	}
	return updated, nil
}

func (m *Manager) ApplyRaw(
	ctx context.Context,
	target RawFileKind,
	expected Revisions,
	content []byte,
	afterWrite func(backupDir string) error,
) (*State, error) {
	if err := os.MkdirAll(m.backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}
	if err := m.lock(ctx); err != nil {
		return nil, err
	}
	defer m.unlock()

	passwdData, err := os.ReadFile(m.passwdPath)
	if err != nil {
		return nil, fmt.Errorf("读取 passwd 失败: %w", err)
	}
	authzData, err := os.ReadFile(m.authzPath)
	if err != nil {
		return nil, fmt.Errorf("读取 authz 失败: %w", err)
	}
	currentRevisions := Revisions{Passwd: Revision(passwdData), Authz: Revision(authzData)}
	if expected != currentRevisions {
		return nil, fmt.Errorf("%w: 期望 %+v，当前 %+v", ErrConflict, expected, currentRevisions)
	}

	newPasswd := passwdData
	newAuthz := authzData
	var targetPath string
	switch target {
	case RawFilePasswd:
		newPasswd = append([]byte(nil), content...)
		targetPath = m.passwdPath
	case RawFileAuthz:
		newAuthz = append([]byte(nil), content...)
		targetPath = m.authzPath
	default:
		return nil, fmt.Errorf("%w: 不支持的文件 %s", ErrValidation, target)
	}
	if equalBytes(passwdData, newPasswd) && equalBytes(authzData, newAuthz) {
		return parseState(passwdData, authzData)
	}
	if _, err := parseState(newPasswd, newAuthz); err != nil {
		return nil, err
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}
	backupDir, err := m.createBackup(passwdData, authzData)
	if err != nil {
		return nil, err
	}
	original := authzData
	replacement := newAuthz
	if target == RawFilePasswd {
		original = passwdData
		replacement = newPasswd
	}
	if err := writeAtomic(targetPath, replacement, info.Mode().Perm()); err != nil {
		return nil, fmt.Errorf("写入 %s 失败: %w", target, err)
	}
	if afterWrite != nil {
		if err := afterWrite(backupDir); err != nil {
			_ = writeAtomic(targetPath, original, info.Mode().Perm())
			return nil, fmt.Errorf("发布后审计失败，已回滚: %w", err)
		}
	}
	m.pruneBackups()
	updated, err := parseState(newPasswd, newAuthz)
	if err != nil {
		_ = writeAtomic(targetPath, original, info.Mode().Perm())
		return nil, err
	}
	return updated, nil
}

func (m *Manager) ListSnapshots() ([]SnapshotInfo, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotInfo{}, nil
		}
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}
	snapshots := make([]SnapshotInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		created, err := time.Parse(backupTimeLayout, entry.Name())
		if err != nil {
			continue
		}
		passwdPath := filepath.Join(m.backupDir, entry.Name(), "passwd")
		authzPath := filepath.Join(m.backupDir, entry.Name(), "authz")
		item := SnapshotInfo{
			ID: entry.Name(), CreatedAt: created.UTC().Format(time.RFC3339Nano),
			Passwd: SnapshotFileInfo{},
			Authz:  SnapshotFileInfo{},
		}
		if stat, statErr := os.Stat(passwdPath); statErr == nil && stat.Mode().IsRegular() {
			item.Passwd = SnapshotFileInfo{Exists: true, Size: stat.Size()}
		}
		if stat, statErr := os.Stat(authzPath); statErr == nil && stat.Mode().IsRegular() {
			item.Authz = SnapshotFileInfo{Exists: true, Size: stat.Size()}
		}
		snapshots = append(snapshots, item)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].ID > snapshots[j].ID })
	return snapshots, nil
}

func (m *Manager) ReadSnapshot(id string) (*SnapshotContent, error) {
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	dir := filepath.Join(m.backupDir, id)
	passwdData, err := os.ReadFile(filepath.Join(dir, "passwd"))
	if err != nil {
		return nil, fmt.Errorf("读取快照 passwd 失败: %w", err)
	}
	authzData, err := os.ReadFile(filepath.Join(dir, "authz"))
	if err != nil {
		return nil, fmt.Errorf("读取快照 authz 失败: %w", err)
	}
	if _, err := parseState(passwdData, authzData); err != nil {
		return nil, fmt.Errorf("快照内容校验失败: %w", err)
	}
	created, _ := time.Parse(backupTimeLayout, id)
	return &SnapshotContent{
		ID: id, CreatedAt: created.UTC().Format(time.RFC3339Nano),
		Passwd: passwdData, Authz: authzData,
	}, nil
}

func (m *Manager) RestoreSnapshot(
	ctx context.Context,
	id string,
	expected Revisions,
	afterWrite func(preRestoreBackup string) error,
) (*State, error) {
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	if err := m.lock(ctx); err != nil {
		return nil, err
	}
	defer m.unlock()

	passwdData, err := os.ReadFile(m.passwdPath)
	if err != nil {
		return nil, fmt.Errorf("读取 passwd 失败: %w", err)
	}
	authzData, err := os.ReadFile(m.authzPath)
	if err != nil {
		return nil, fmt.Errorf("读取 authz 失败: %w", err)
	}
	currentRevisions := Revisions{Passwd: Revision(passwdData), Authz: Revision(authzData)}
	if expected != currentRevisions {
		return nil, fmt.Errorf("%w: 期望 %+v，当前 %+v", ErrConflict, expected, currentRevisions)
	}
	snapshot, err := m.ReadSnapshot(id)
	if err != nil {
		return nil, err
	}
	if _, err := parseState(snapshot.Passwd, snapshot.Authz); err != nil {
		return nil, err
	}
	passwdInfo, err := os.Stat(m.passwdPath)
	if err != nil {
		return nil, err
	}
	authzInfo, err := os.Stat(m.authzPath)
	if err != nil {
		return nil, err
	}
	preRestoreBackup, err := m.createBackup(passwdData, authzData)
	if err != nil {
		return nil, err
	}
	rollback := func() {
		_ = writeAtomic(m.passwdPath, passwdData, passwdInfo.Mode().Perm())
		_ = writeAtomic(m.authzPath, authzData, authzInfo.Mode().Perm())
	}
	if err := writeAtomic(m.passwdPath, snapshot.Passwd, passwdInfo.Mode().Perm()); err != nil {
		rollback()
		return nil, fmt.Errorf("恢复快照 passwd 失败: %w", err)
	}
	if err := writeAtomic(m.authzPath, snapshot.Authz, authzInfo.Mode().Perm()); err != nil {
		rollback()
		return nil, fmt.Errorf("恢复快照 authz 失败: %w", err)
	}
	if afterWrite != nil {
		if err := afterWrite(preRestoreBackup); err != nil {
			rollback()
			return nil, fmt.Errorf("发布后审计失败，已回滚: %w", err)
		}
	}
	m.pruneBackups()
	updated, err := parseState(snapshot.Passwd, snapshot.Authz)
	if err != nil {
		rollback()
		return nil, err
	}
	return updated, nil
}

func validateSnapshotID(id string) error {
	if _, err := time.Parse(backupTimeLayout, id); err != nil || filepath.Base(id) != id {
		return fmt.Errorf("%w: 无效的快照 %s", ErrValidation, id)
	}
	return nil
}

func (m *Manager) lock(ctx context.Context) error {
	m.mu.Lock()
	if _, err := m.fileLock.TryLockContext(ctx, 100*time.Millisecond); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("获取配置文件锁失败: %w", err)
	}
	return nil
}

func (m *Manager) unlock() {
	_ = m.fileLock.Unlock()
	m.mu.Unlock()
}

func (m *Manager) createBackup(passwdData, authzData []byte) (string, error) {
	if err := os.MkdirAll(m.backupDir, 0o700); err != nil {
		return "", err
	}
	name := time.Now().UTC().Format(backupTimeLayout)
	dir := filepath.Join(m.backupDir, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", err
	}
	for file, data := range map[string][]byte{
		"passwd": passwdData,
		"authz":  authzData,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func (m *Manager) pruneBackups() {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := time.Parse(backupTimeLayout, entry.Name()); err != nil {
				continue
			}
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	for len(dirs) > m.keep {
		target := filepath.Join(m.backupDir, dirs[0])
		if isContained(m.backupDir, target) {
			_ = os.RemoveAll(target)
		}
		dirs = dirs[1:]
	}
}

func isContained(root, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !startsWithParent(rel)
}

func startsWithParent(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}

func writeAtomic(target string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(target)
	temp, err := os.CreateTemp(dir, ".svnpanel-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(mode.Perm()); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, target); err != nil {
		return err
	}
	cleanup = false
	if runtime.GOOS != "windows" {
		if dirHandle, openErr := os.Open(dir); openErr == nil {
			_ = dirHandle.Sync()
			_ = dirHandle.Close()
		}
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
