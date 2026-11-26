package cfg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	c.applyPathMap()
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("配置文件权限过宽，请执行 chmod 600 %s", path)
		}
	}
	return &c, nil
}

func (c *Config) applyPathMap() {
	if len(c.PathMap) == 0 {
		return
	}
	type mapping struct {
		from string
		to   string
	}
	mappings := make([]mapping, 0, len(c.PathMap))
	for from, to := range c.PathMap {
		from = strings.TrimSuffix(strings.ReplaceAll(strings.TrimSpace(from), "\\", "/"), "/")
		to = strings.TrimSuffix(strings.ReplaceAll(strings.TrimSpace(to), "\\", "/"), "/")
		if from != "" && to != "" {
			mappings = append(mappings, mapping{from: from, to: to})
		}
	}
	sort.Slice(mappings, func(i, j int) bool { return len(mappings[i].from) > len(mappings[j].from) })
	remap := func(value string) string {
		normalized := strings.ReplaceAll(filepath.Clean(value), "\\", "/")
		for _, item := range mappings {
			if normalized == item.from {
				return filepath.FromSlash(item.to)
			}
			if strings.HasPrefix(normalized, item.from+"/") {
				suffix := strings.TrimPrefix(normalized, item.from)
				return filepath.FromSlash(item.to + suffix)
			}
		}
		return value
	}
	c.SVN.RepoRoot = remap(c.SVN.RepoRoot)
	c.SVN.PasswdFile = remap(c.SVN.PasswdFile)
	c.SVN.AuthzFile = remap(c.SVN.AuthzFile)
	c.Audit.Dir = remap(c.Audit.Dir)
	c.Backup.Dir = remap(c.Backup.Dir)
}

func (c *Config) applyDefaults() {
	if c.Global.Title == "" {
		c.Global.Title = "SVN 权限面板"
	}
	if c.Global.RunMode == "" {
		c.Global.RunMode = "release"
	}
	if c.HTTP.Host == "" {
		c.HTTP.Host = "127.0.0.1"
	}
	if c.HTTP.Port == 0 {
		c.HTTP.Port = 8080
	}
	if c.HTTP.ReadTimeout <= 0 {
		c.HTTP.ReadTimeout = 15
	}
	if c.HTTP.WriteTimeout <= 0 {
		c.HTTP.WriteTimeout = 30
	}
	if c.HTTP.IdleTimeout <= 0 {
		c.HTTP.IdleTimeout = 60
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		c.HTTP.ShutdownTimeout = 15
	}
	if c.HTTP.MaxContentLength <= 0 {
		c.HTTP.MaxContentLength = 1 << 20
	}
	if len(c.HTTP.TrustedProxies) == 0 {
		c.HTTP.TrustedProxies = []string{"127.0.0.1", "::1"}
	}
	if c.SVN.SvnlookPath == "" {
		c.SVN.SvnlookPath = "svnlook"
	}
	if c.SVN.CommandTimeoutSec <= 0 {
		c.SVN.CommandTimeoutSec = 5
	}
	if c.SVN.MaxCommandOutputMiB <= 0 {
		c.SVN.MaxCommandOutputMiB = 4
	}
	if c.SVN.MaxPathsPerLevel <= 0 {
		c.SVN.MaxPathsPerLevel = 2000
	}
	if c.Audit.Dir == "" {
		c.Audit.Dir = "logs/audit"
	}
	if c.Backup.Dir == "" {
		c.Backup.Dir = "data/backups"
	}
	if c.Backup.Keep <= 0 {
		c.Backup.Keep = 50
	}
	if c.Session.IdleMinutes <= 0 {
		c.Session.IdleMinutes = 30
	}
	if c.Session.AbsoluteHours <= 0 {
		c.Session.AbsoluteHours = 8
	}
}

func (c *Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Admin.Username) == "" {
		errs = append(errs, errors.New("Admin.Username 不能为空"))
	}
	if strings.TrimSpace(c.Admin.Password) == "" {
		errs = append(errs, errors.New("Admin.Password 不能为空"))
	}
	if strings.TrimSpace(c.SVN.RepoRoot) == "" {
		errs = append(errs, errors.New("SVN.RepoRoot 不能为空"))
	}
	if strings.TrimSpace(c.SVN.PasswdFile) == "" {
		errs = append(errs, errors.New("SVN.PasswdFile 不能为空"))
	}
	if strings.TrimSpace(c.SVN.AuthzFile) == "" {
		errs = append(errs, errors.New("SVN.AuthzFile 不能为空"))
	}
	if strings.TrimSpace(c.Audit.Dir) == "" {
		errs = append(errs, errors.New("Audit.Dir 不能为空"))
	}
	if strings.TrimSpace(c.Backup.Dir) == "" {
		errs = append(errs, errors.New("Backup.Dir 不能为空"))
	}
	if runtime.GOOS == "windows" {
		paths := map[string]string{
			"SVN.RepoRoot": c.SVN.RepoRoot, "SVN.PasswdFile": c.SVN.PasswdFile,
			"SVN.AuthzFile": c.SVN.AuthzFile, "Audit.Dir": c.Audit.Dir, "Backup.Dir": c.Backup.Dir,
		}
		for name, value := range paths {
			if strings.HasPrefix(value, string(filepath.Separator)) && filepath.VolumeName(value) == "" {
				errs = append(errs, fmt.Errorf("%s=%q 是无效的 Windows 根路径；请改为 D:/...、相对路径，或配置 [PathMap]", name, value))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *Config) EnsureRuntimeDirs() error {
	for _, dir := range []string{c.Audit.Dir, c.Backup.Dir} {
		if err := os.MkdirAll(filepath.Clean(dir), 0o700); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}
	return nil
}
