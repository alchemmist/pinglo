package pinglo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
)

type Item struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Cwd       string    `json:"cwd"`
	Command   string    `json:"command"`
	Status    Status    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Order     int       `json:"order"`
}

type Request struct {
	Action   string `json:"action"`
	Cwd      string `json:"cwd,omitempty"`
	Command  string `json:"command,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Items []Item `json:"items,omitempty"`
}

type Manager struct {
	mu       sync.Mutex
	items    map[string]*Item
	nextID   int
	order    int
	onChange func()
}

func NewManager(onChange func()) *Manager {
	return &Manager{items: make(map[string]*Item), onChange: onChange}
}

func BuildKey(cwd, command string) string {
	return strings.TrimSpace(cwd) + "\x00" + strings.TrimSpace(command)
}

func (m *Manager) Start(cwd, command string) *Item {
	m.mu.Lock()
	key := BuildKey(cwd, command)
	now := time.Now()
	item, ok := m.items[key]
	if ok {
		item.Status = StatusRunning
		item.UpdatedAt = now
		result := clone(item)
		m.mu.Unlock()
		m.trigger()
		return result
	}

	m.nextID++
	m.order++
	item = &Item{
		ID:        fmt.Sprintf("dot-%d", m.nextID),
		Key:       key,
		Cwd:       cwd,
		Command:   command,
		Status:    StatusRunning,
		StartedAt: now,
		UpdatedAt: now,
		Order:     m.order,
	}
	m.items[key] = item
	result := clone(item)
	m.mu.Unlock()
	m.trigger()
	return result
}

func (m *Manager) Finish(cwd, command string, exitCode int) *Item {
	m.mu.Lock()
	key := BuildKey(cwd, command)
	now := time.Now()
	status := StatusFailed
	if exitCode == 0 {
		status = StatusSuccess
	}

	item, ok := m.items[key]
	if !ok {
		m.nextID++
		m.order++
		item = &Item{
			ID:        fmt.Sprintf("dot-%d", m.nextID),
			Key:       key,
			Cwd:       cwd,
			Command:   command,
			StartedAt: now,
			Order:     m.order,
		}
		m.items[key] = item
	}

	item.Status = status
	item.UpdatedAt = now
	if item.StartedAt.IsZero() {
		item.StartedAt = now
	}
	result := clone(item)
	m.mu.Unlock()
	m.trigger()
	return result
}

func (m *Manager) Clear() {
	m.mu.Lock()
	m.items = make(map[string]*Item)
	m.order = 0
	m.mu.Unlock()
	m.trigger()
}

func (m *Manager) List() []Item {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]Item, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, *clone(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Order == items[j].Order {
			return items[i].UpdatedAt.Before(items[j].UpdatedAt)
		}
		return items[i].Order < items[j].Order
	})
	return items
}

func clone(item *Item) *Item {
	cp := *item
	return &cp
}

func (m *Manager) trigger() {
	if m.onChange != nil {
		m.onChange()
	}
}

func DefaultSocketPath() string {
	if path := strings.TrimSpace(os.Getenv("PINGLO_SOCKET")); path != "" {
		return path
	}
	runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if runtimeDir != "" {
		return filepath.Join(runtimeDir, "pinglo.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("pinglo-%d.sock", os.Getuid()))
}
