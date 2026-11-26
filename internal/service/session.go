package service

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	Token     string
	CSRF      string
	Username  string
	CreatedAt time.Time
	LastSeen  time.Time
}

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]Session
	idle     time.Duration
	absolute time.Duration
}

func NewSessionManager(idle, absolute time.Duration) *SessionManager {
	return &SessionManager{sessions: make(map[string]Session), idle: idle, absolute: absolute}
}

func (m *SessionManager) Create(username string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	now := time.Now()
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	session := Session{
		Token: token, CSRF: csrf, Username: username,
		CreatedAt: now, LastSeen: now,
	}
	m.sessions[session.Token] = session
	return session, nil
}

func (m *SessionManager) Get(token string) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	session, ok := m.sessions[token]
	if !ok {
		return Session{}, false
	}
	if now.Sub(session.LastSeen) > m.idle || now.Sub(session.CreatedAt) > m.absolute {
		delete(m.sessions, token)
		return Session{}, false
	}
	session.LastSeen = now
	m.sessions[token] = session
	return session, true
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *SessionManager) cleanupLocked(now time.Time) {
	for token, session := range m.sessions {
		if now.Sub(session.LastSeen) > m.idle || now.Sub(session.CreatedAt) > m.absolute {
			delete(m.sessions, token)
		}
	}
}

type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]limitEntry
	max     int
	window  time.Duration
}

type limitEntry struct {
	count int
	first time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: make(map[string]limitEntry), max: 5, window: 5 * time.Minute}
}

func (l *loginLimiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok {
		return true
	}
	if time.Since(entry.first) > l.window {
		delete(l.entries, key)
		return true
	}
	return entry.count < l.max
}

func (l *loginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.first.IsZero() || time.Since(entry.first) > l.window {
		entry = limitEntry{first: time.Now()}
	}
	entry.count++
	l.entries[key] = entry
}

func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func randomToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
