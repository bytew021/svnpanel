package cfg

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	content := []byte(`
[Admin]
Username = "admin"
Password = "password"
[SVN]
RepoRoot = "/srv/svn/repos"
PasswdFile = "/srv/svn/etc/passwd"
AuthzFile = "/srv/svn/etc/authz"

[Metadata]
Departments = ["legacy-department"]
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, 0o600)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTP.Host != "127.0.0.1" || config.HTTP.Port != 8080 {
		t.Fatalf("unexpected http defaults: %+v", config.HTTP)
	}
	if config.Backup.Keep != 50 || config.Session.IdleMinutes != 30 {
		t.Fatalf("unexpected runtime defaults: %+v %+v", config.Backup, config.Session)
	}
	if config.SVN.SvnservePath != "svnserve" || config.SVN.SvnservePort != 3690 {
		t.Fatalf("unexpected svnserve defaults: %+v", config.SVN)
	}
}

func TestLoadAppliesLongestPathMap(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	content := []byte(`
[Admin]
Username = "admin"
Password = "password"
[SVN]
RepoRoot = "/data/svn/repos"
PasswdFile = "/data/svn/repos/project/conf/passwd"
AuthzFile = "/data/svn/repos/project/conf/authz"
[Audit]
Dir = "/var/lib/svnpanel/audit"
[Backup]
Dir = "/var/lib/svnpanel/backups"
[PathMap]
"/data/svn" = "D:/svn-dev"
"/data/svn/repos/project/conf" = "D:/project-conf"
"/var/lib/svnpanel" = "D:/svnpanel-data"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, 0o600)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(config.SVN.PasswdFile) != filepath.Clean(filepath.FromSlash("D:/project-conf/passwd")) {
		t.Fatalf("longest mapping was not used: %s", config.SVN.PasswdFile)
	}
	if filepath.Clean(config.SVN.RepoRoot) != filepath.Clean(filepath.FromSlash("D:/svn-dev/repos")) {
		t.Fatalf("repo mapping failed: %s", config.SVN.RepoRoot)
	}
}
