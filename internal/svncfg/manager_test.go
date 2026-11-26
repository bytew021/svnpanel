package svncfg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestManagerApplyCreatesBackupsAndRejectsStaleRevision(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	backupDir := filepath.Join(dir, "backups")
	if err := os.WriteFile(passwdPath, []byte("# comment\nalice = old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authzPath, []byte("[groups]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(passwdPath, 0o600)
		_ = os.Chmod(authzPath, 0o600)
	}
	manager := NewManager(passwdPath, authzPath, backupDir, 3)
	state, err := manager.Read()
	if err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Apply(context.Background(), state.Revisions, func(passwd *PasswdDoc, _ *AuthzDoc) error {
		return passwd.SetPassword("alice", "new-password")
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revisions == state.Revisions {
		t.Fatal("revision did not change")
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("backup was not created")
	}
	if _, err := manager.Apply(context.Background(), state.Revisions, func(passwd *PasswdDoc, _ *AuthzDoc) error {
		return passwd.SetPassword("alice", "other-password")
	}, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestManagerRollsBackWhenPostWriteHookFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	original := []byte("# keep\nalice = old-password\n")
	if err := os.WriteFile(passwdPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authzPath, []byte("[groups]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(passwdPath, authzPath, filepath.Join(dir, "backups"), 3)
	state, err := manager.Read()
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Apply(context.Background(), state.Revisions, func(passwd *PasswdDoc, _ *AuthzDoc) error {
		return passwd.SetPassword("alice", "new-password")
	}, func(string) error {
		return errors.New("audit unavailable")
	})
	if err == nil {
		t.Fatal("expected post-write failure")
	}
	got, err := os.ReadFile(passwdPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("file was not rolled back: %q", got)
	}
}

func TestManagerPrunesOldBackups(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	if err := os.WriteFile(passwdPath, []byte("alice = password-one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authzPath, []byte("[groups]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(dir, "backups")
	manager := NewManager(passwdPath, authzPath, backupDir, 2)
	state, err := manager.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"password-two", "password-three", "password-four"} {
		state, err = manager.Apply(context.Background(), state.Revisions, func(passwd *PasswdDoc, _ *AuthzDoc) error {
			return passwd.SetPassword("alice", password)
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	directories := 0
	for _, entry := range entries {
		if entry.IsDir() {
			directories++
		}
	}
	if directories != 2 {
		t.Fatalf("expected 2 backup directories, got %d", directories)
	}
}
