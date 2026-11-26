package svncfg

import (
	"context"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseTreeOutput(t *testing.T) {
	t.Parallel()
	output := []byte("/\n/branches/\n/trunk/\n/trunk/docs/\n/trunk/README.txt\n")
	nodes, err := parseTreeOutput(output, "/", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Path != "/branches" || nodes[1].Path != "/trunk" {
		t.Fatalf("unexpected root nodes: %+v", nodes)
	}
	children, err := parseTreeOutput(output, "/trunk", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Path != "/trunk/docs" {
		t.Fatalf("unexpected child nodes: %+v", children)
	}
	if _, err := parseTreeOutput([]byte("/a/\n/b/\n/c/\n"), "/", 2); err == nil {
		t.Fatal("expected output limit error")
	}
}

func TestScannerWithLocalSvnlook(t *testing.T) {
	svnadminPath, err := exec.LookPath("svnadmin")
	if err != nil {
		t.Skip("svnadmin 不可用")
	}
	svnPath, err := exec.LookPath("svn")
	if err != nil {
		t.Skip("svn 不可用")
	}
	root := t.TempDir()
	repoPath := filepath.Join(root, "demo")
	if output, err := exec.Command(svnadminPath, "create", repoPath).CombinedOutput(); err != nil {
		t.Fatalf("svnadmin create: %v: %s", err, output)
	}
	urlPath := filepath.ToSlash(repoPath)
	if runtime.GOOS == "windows" && len(urlPath) > 1 && urlPath[1] == ':' {
		urlPath = "/" + urlPath
	}
	repoURL := (&url.URL{Scheme: "file", Path: urlPath}).String()
	if output, err := exec.Command(svnPath, "mkdir", "-m", "init", repoURL+"/trunk").CombinedOutput(); err != nil {
		t.Fatalf("svn mkdir: %v: %s", err, output)
	}
	if output, err := exec.Command(svnPath, "mkdir", "-m", "dirs", repoURL+"/trunk/src", repoURL+"/trunk/docs").CombinedOutput(); err != nil {
		t.Fatalf("svn mkdir children: %v: %s", err, output)
	}
	scanner := Scanner{
		RepoRoot: root, SvnlookPath: "svnlook",
		Timeout: 5 * time.Second, MaxOutput: 1 << 20, MaxPathCount: 100,
	}
	repos, err := scanner.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "demo" || repos[0].Revision < 2 {
		t.Fatalf("unexpected repositories: %+v", repos)
	}
	children, err := scanner.Tree(context.Background(), "demo", "/trunk")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 || children[0].Path != "/trunk/docs" || children[1].Path != "/trunk/src" {
		t.Fatalf("unexpected tree: %+v", children)
	}
}
