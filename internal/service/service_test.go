package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"svnpanel/internal/cfg"
)

func TestLoginRateLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	if err := os.WriteFile(passwdPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authzPath, []byte("[groups]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := &cfg.Config{
		Admin: cfg.AdminConfig{Username: "admin", Password: "correct-password"},
		SVN: cfg.SVNConfig{
			RepoRoot: dir, PasswdFile: passwdPath, AuthzFile: authzPath,
			SvnlookPath: "svnlook", SvnservePath: "svnserve", SvnservePort: 3690,
		},
		Audit:   cfg.AuditConfig{Dir: filepath.Join(dir, "audit")},
		Backup:  cfg.BackupConfig{Dir: filepath.Join(dir, "backups"), Keep: 2},
		Session: cfg.SessionConfig{IdleMinutes: 30, AbsoluteHours: 8},
	}
	svc := New(config)
	for i := 0; i < 5; i++ {
		if _, _, err := svc.Login("admin", "wrong-password", "127.0.0.1"); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("attempt %d: expected unauthorized, got %v", i+1, err)
		}
	}
	if _, _, err := svc.Login("admin", "wrong-password", "127.0.0.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func TestListUsersAndGroupsPreserveFileOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	if err := os.WriteFile(
		passwdPath,
		[]byte("zeta = zeta-password\nalpha = alpha-password\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		authzPath,
		[]byte("[groups]\nzeta_group = zeta\nalpha_group = alpha\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	config := &cfg.Config{
		Admin: cfg.AdminConfig{Username: "admin", Password: "correct-password"},
		SVN: cfg.SVNConfig{
			RepoRoot: dir, PasswdFile: passwdPath, AuthzFile: authzPath,
			SvnlookPath: "svnlook", SvnservePath: "svnserve", SvnservePort: 3690,
		},
		Audit:   cfg.AuditConfig{Dir: filepath.Join(dir, "audit")},
		Backup:  cfg.BackupConfig{Dir: filepath.Join(dir, "backups"), Keep: 2},
		Session: cfg.SessionConfig{IdleMinutes: 30, AbsoluteHours: 8},
	}
	svc := New(config)
	users, err := svc.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users.Items) != 2 || users.Items[0].Name != "zeta" || users.Items[1].Name != "alpha" {
		t.Fatalf("user order was not preserved: %+v", users.Items)
	}
	groups, err := svc.ListGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups.Items) != 2 ||
		groups.Items[0].Name != "zeta_group" ||
		groups.Items[1].Name != "alpha_group" {
		t.Fatalf("group order was not preserved: %+v", groups.Items)
	}
}

func TestReadSvnserveConfigInfoMatchesPanelFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	configPath := filepath.Join(dir, "svnserve.conf")
	if err := os.WriteFile(passwdPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authzPath, []byte("[groups]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"[general]",
		"anon-access = none",
		"auth-access = write",
		"password-db = passwd",
		"authz-db = authz",
		"realm = svn",
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	info := readSvnserveConfigInfo(configPath, passwdPath, authzPath)
	if !info.Exists || !info.Readable {
		t.Fatalf("config should be readable: %+v", info)
	}
	if !info.PasswordMatches || !info.AuthzMatches {
		t.Fatalf("auth paths should match panel files: %+v", info)
	}
	if info.Realm != "svn" || info.AnonAccess != "none" || info.AuthAccess != "write" {
		t.Fatalf("unexpected parsed config: %+v", info)
	}
	if len(info.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", info.Warnings)
	}

	if err := os.WriteFile(configPath, []byte(
		"[general]\npassword-db = other-passwd\nauthz-db = authz\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	mismatched := readSvnserveConfigInfo(configPath, passwdPath, authzPath)
	if mismatched.PasswordMatches || !mismatched.AuthzMatches {
		t.Fatalf("unexpected match state: %+v", mismatched)
	}
	if !strings.Contains(strings.Join(mismatched.Warnings, "\n"), "password-db 与面板的 passwd 不一致") {
		t.Fatalf("missing mismatch warning: %v", mismatched.Warnings)
	}
}

func TestReadSvnserveConfigInfoReportsMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	info := readSvnserveConfigInfo(
		filepath.Join(dir, "svnserve.conf"),
		filepath.Join(dir, "passwd"),
		filepath.Join(dir, "authz"),
	)
	if info.Exists || info.Readable {
		t.Fatalf("missing config must not be readable: %+v", info)
	}
	if !strings.Contains(strings.Join(info.Warnings, "\n"), "不存在") {
		t.Fatalf("missing warning: %v", info.Warnings)
	}
}
