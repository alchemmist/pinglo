package pinglo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	Color     string    `json:"color,omitempty"`
	Tooltip   string    `json:"tooltip,omitempty"`
}

type Request struct {
	Action   string      `json:"action"`
	Cwd      string      `json:"cwd,omitempty"`
	Command  string      `json:"command,omitempty"`
	ExitCode *int        `json:"exit_code,omitempty"`
	Dot      *DotRequest `json:"dot,omitempty"`
}

type DotRequest struct {
	ID      string `json:"id,omitempty"`
	Color   string `json:"color,omitempty"`
	Tooltip string `json:"tooltip,omitempty"`
	Status  Status `json:"status,omitempty"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Items []Item `json:"items,omitempty"`
}

type Manager struct {
	mu        sync.Mutex
	items     map[string]*Item
	nextID    int
	order     int
	onChange  func()
	stateFile string
}

func NewManager(onChange func(), stateFile string) *Manager {
	m := &Manager{items: make(map[string]*Item), onChange: onChange, stateFile: stateFile}
	if err := m.LoadState(); err != nil {
		log.Printf("pinglo: failed to load state: %v", err)
	}
	return m
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
	m.persist()
	m.trigger()
	return result
}

func (m *Manager) Clear() {
	m.mu.Lock()
	m.items = make(map[string]*Item)
	m.order = 0
	m.mu.Unlock()
	m.persist()
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

func (m *Manager) SetDot(id, color, tooltip string, status Status) *Item {
	if strings.TrimSpace(id) == "" {
		m.nextID++
		id = fmt.Sprintf("dot-%d", m.nextID)
	}
	if status == "" {
		status = StatusRunning
	}

	m.mu.Lock()
	now := time.Now()
	item, ok := m.items[id]
	if !ok {
		m.order++
		item = &Item{
			ID:        id,
			Key:       id,
			Status:    status,
			StartedAt: now,
			Order:     m.order,
		}
		m.items[id] = item
	}
	item.Status = status
	if color != "" {
		item.Color = color
	}
	if tooltip != "" {
		item.Tooltip = tooltip
	}
	item.UpdatedAt = now
	if item.StartedAt.IsZero() {
		item.StartedAt = now
	}
	result := clone(item)
	m.mu.Unlock()
	m.persist()
	m.trigger()
	return result
}

func (m *Manager) RemoveDot(id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	m.mu.Lock()
	if _, ok := m.items[id]; !ok {
		m.mu.Unlock()
		return false
	}
	delete(m.items, id)
	m.mu.Unlock()
	m.persist()
	m.trigger()
	return true
}

func (m *Manager) LoadState() error {
	if strings.TrimSpace(m.stateFile) == "" {
		return nil
	}
	data, err := os.ReadFile(m.stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var dump struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(data, &dump); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]*Item, len(dump.Items))
	m.nextID = 0
	m.order = 0
	for _, item := range dump.Items {
		cp := clone(&item)
		m.items[cp.Key] = cp
		if n := parseDotNumericID(cp.ID); n > m.nextID {
			m.nextID = n
		}
		if cp.Order > m.order {
			m.order = cp.Order
		}
	}
	return nil
}

func (m *Manager) persist() {
	if strings.TrimSpace(m.stateFile) == "" {
		return
	}
	items := m.List()
	dump := struct {
		Items []Item `json:"items"`
	}{
		Items: items,
	}
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		log.Printf("pinglo: failed to marshal state: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.stateFile), 0o755); err != nil {
		log.Printf("pinglo: failed to create state dir: %v", err)
		return
	}
	tmp := m.stateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("pinglo: failed to write state: %v", err)
		return
	}
	if err := os.Rename(tmp, m.stateFile); err != nil {
		log.Printf("pinglo: failed to finalize state file: %v", err)
	}
}

func parseDotNumericID(id string) int {
	const prefix = "dot-"
	if !strings.HasPrefix(id, prefix) {
		return 0
	}
	num, err := strconv.Atoi(id[len(prefix):])
	if err != nil {
		return 0
	}
	return num
}

func DefaultStatePath() string {
	if path := strings.TrimSpace(os.Getenv("PINGLO_STATE_FILE")); path != "" {
		return path
	}
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	if dataHome == "" {
		dataHome = os.TempDir()
	}
	return filepath.Join(dataHome, "pinglo", "state.json")
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
