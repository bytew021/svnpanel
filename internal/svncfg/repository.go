package svncfg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Repository struct {
	Name     string `json:"name"`
	Path     string `json:"-"`
	Revision int64  `json:"revision"`
}

type PathNode struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Scanner struct {
	RepoRoot     string
	SvnlookPath  string
	Timeout      time.Duration
	MaxOutput    int64
	MaxPathCount int
}

func (s Scanner) Check(ctx context.Context) error {
	output, err := s.run(ctx, "--version")
	if err != nil {
		return fmt.Errorf("svnlook 不可用: %w", err)
	}
	if len(bytes.TrimSpace(output)) == 0 {
		return fmt.Errorf("svnlook --version 未返回内容")
	}
	return nil
}

func (s Scanner) Discover(ctx context.Context) ([]Repository, error) {
	entries, err := os.ReadDir(s.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("读取仓库根目录失败: %w", err)
	}
	repos := make([]Repository, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		repoPath := filepath.Join(s.RepoRoot, entry.Name())
		output, err := s.run(ctx, "youngest", repoPath)
		if err != nil {
			continue
		}
		var revision int64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &revision); err != nil {
			continue
		}
		repos = append(repos, Repository{Name: entry.Name(), Path: repoPath, Revision: revision})
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
	return repos, nil
}

func (s Scanner) Tree(ctx context.Context, repoName, parent string) ([]PathNode, error) {
	if err := ValidateRepoPath(parent); err != nil {
		return nil, err
	}
	parent = NormalizePath(parent)
	if s.MaxPathCount <= 0 {
		s.MaxPathCount = 2000
	}
	repos, err := s.Discover(ctx)
	if err != nil {
		return nil, err
	}
	var repo *Repository
	for i := range repos {
		if repos[i].Name == repoName {
			repo = &repos[i]
			break
		}
	}
	if repo == nil {
		return nil, fmt.Errorf("%w: 仓库 %s", ErrNotFound, repoName)
	}
	output, err := s.run(ctx, "tree", "--full-paths", "--non-recursive", repo.Path, parent)
	if err != nil {
		return nil, err
	}
	return parseTreeOutput(output, parent, s.MaxPathCount)
}

func parseTreeOutput(output []byte, parent string, maxPaths int) ([]PathNode, error) {
	parent = NormalizePath(parent)
	if maxPaths <= 0 {
		maxPaths = 2000
	}
	nodes := make([]PathNode, 0)
	seen := make(map[string]bool)
	for _, raw := range strings.Split(string(output), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasSuffix(line, "/") {
			continue
		}
		itemPath := NormalizePath(strings.TrimSuffix(line, "/"))
		if itemPath == parent || seen[itemPath] {
			continue
		}
		if path.Dir(itemPath) != parent {
			continue
		}
		seen[itemPath] = true
		nodes = append(nodes, PathNode{Name: path.Base(itemPath), Path: itemPath})
		if len(nodes) > maxPaths {
			return nil, fmt.Errorf("目录项超过限制 %d", maxPaths)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
	return nodes, nil
}

func (s Scanner) run(ctx context.Context, args ...string) ([]byte, error) {
	if s.Timeout <= 0 {
		s.Timeout = 5 * time.Second
	}
	if s.MaxOutput <= 0 {
		s.MaxOutput = 4 << 20
	}
	commandCtx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, s.SvnlookPath, args...)
	var stdout limitedBuffer
	stdout.limit = s.MaxOutput
	var stderr limitedBuffer
	stderr.limit = 64 << 10
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if commandCtx.Err() != nil {
			return nil, fmt.Errorf("svnlook 执行超时")
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("svnlook 执行失败: %s", message)
	}
	if stdout.exceeded {
		return nil, fmt.Errorf("svnlook 输出超过 %d 字节", s.MaxOutput)
	}
	return bytes.Clone(stdout.Bytes()), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int64
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	remaining := b.limit - int64(b.Len())
	if remaining <= 0 {
		b.exceeded = true
		return originalLen, nil
	}
	if int64(len(p)) > remaining {
		b.exceeded = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return originalLen, nil
}
