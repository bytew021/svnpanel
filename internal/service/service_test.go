package service

import (
	"errors"
	"os"
	"path/filepath"
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
