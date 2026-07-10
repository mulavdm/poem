package theme

import "sync"

// Manager publishes immutable theme snapshots and supports live switching.
type Manager struct {
	mu       sync.RWMutex
	current  Theme
	revision uint64
}

func NewManager(initial Theme) *Manager {
	if initial.Validate() != nil {
		initial = ModernDark()
	}
	return &Manager{current: initial, revision: 1}
}

func (m *Manager) Current() (Theme, uint64) {
	if m == nil {
		return ModernDark(), 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current, m.revision
}

func (m *Manager) Set(next Theme) error {
	if err := next.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	m.current = next
	m.revision++
	m.mu.Unlock()
	return nil
}

func (m *Manager) Update(name string, apply func(*Theme)) error {
	current, _ := m.Current()
	return m.Set(current.With(name, apply))
}
