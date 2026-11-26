package audit

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Event struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Outcome   string         `json:"outcome"`
	Target    string         `json:"target,omitempty"`
	Repo      string         `json:"repo,omitempty"`
	Path      string         `json:"path,omitempty"`
	ClientIP  string         `json:"clientIp,omitempty"`
	Message   string         `json:"message,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type Filter struct {
	From    *time.Time
	To      *time.Time
	Action  string
	Outcome string
	Keyword string
	Page    int
	Size    int
}

type Result struct {
	Items    []Event `json:"items"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

type Store struct {
	dir string
	mu  sync.Mutex
}

func NewStore(dir string) *Store {
	return &Store{dir: filepath.Clean(dir)}
}

func (s *Store) Append(event Event) error {
	if event.ID == "" {
		event.ID = randomID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Outcome == "" {
		event.Outcome = "success"
	}
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	name := "audit-" + event.Timestamp.UTC().Format("2006-01-02") + ".jsonl"
	file, err := os.OpenFile(filepath.Join(s.dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (s *Store) Query(filter Filter) (Result, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Size <= 0 {
		filter.Size = 50
	}
	if filter.Size > 200 {
		filter.Size = 200
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Items: []Event{}, Page: filter.Page, PageSize: filter.Size}, nil
		}
		return Result{}, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "audit-") && strings.HasSuffix(entry.Name(), ".jsonl") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	keep := filter.Page * filter.Size
	total := 0
	items := make([]Event, 0, keep)
	for _, name := range names {
		file, err := os.Open(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64<<10), 4<<20)
		for scanner.Scan() {
			var event Event
			if json.Unmarshal(scanner.Bytes(), &event) != nil || !matches(event, filter) {
				continue
			}
			total++
			items = append(items, event)
			if len(items) > keep {
				items = items[len(items)-keep:]
			}
		}
		_ = file.Close()
	}
	start := (filter.Page - 1) * filter.Size
	end := start + filter.Size
	if start > len(items) {
		start = len(items)
	}
	if end > len(items) {
		end = len(items)
	}
	pageItems := append([]Event(nil), items[start:end]...)
	for i, j := 0, len(pageItems)-1; i < j; i, j = i+1, j-1 {
		pageItems[i], pageItems[j] = pageItems[j], pageItems[i]
	}
	return Result{Items: pageItems, Total: total, Page: filter.Page, PageSize: filter.Size}, nil
}

func matches(event Event, filter Filter) bool {
	if filter.From != nil && event.Timestamp.Before(*filter.From) {
		return false
	}
	if filter.To != nil && event.Timestamp.After(*filter.To) {
		return false
	}
	if filter.Action != "" && event.Action != filter.Action {
		return false
	}
	if filter.Outcome != "" && event.Outcome != filter.Outcome {
		return false
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Keyword)); keyword != "" {
		haystack := strings.ToLower(strings.Join([]string{
			event.Actor, event.Action, event.Outcome, event.Target, event.Repo, event.Path, event.Message,
		}, "\n"))
		if !strings.Contains(haystack, keyword) {
			return false
		}
	}
	return true
}

func randomID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}
