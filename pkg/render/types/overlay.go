package types

import (
	"image"
	"strings"
	"sync"
)

type OverlayEntry struct {
	ID                 string
	Component          Component
	Modal              bool
	DismissOnEscape    bool
	DismissOnFocusLoss bool
	RestoreFocusID     string
	OnDismiss          func(*ApplicationState)
}

// OverlayManager owns transient z-ordered UI such as menus, popovers, dialogs,
// combo boxes, and toasts. Snapshots keep iteration independent of mutations.
type OverlayManager struct {
	mu      sync.RWMutex
	entries []OverlayEntry
}

func NewOverlayManager() *OverlayManager { return &OverlayManager{} }

func (m *OverlayManager) Open(entry OverlayEntry) {
	if m == nil || entry.ID == "" || entry.Component == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.entries {
		if m.entries[index].ID == entry.ID {
			m.entries[index] = entry
			return
		}
	}
	m.entries = append(m.entries, entry)
}

func (m *OverlayManager) Close(id string) (OverlayEntry, bool) {
	if m == nil {
		return OverlayEntry{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := len(m.entries) - 1; index >= 0; index-- {
		if m.entries[index].ID == id {
			entry := m.entries[index]
			m.entries = append(m.entries[:index], m.entries[index+1:]...)
			return entry, true
		}
	}
	return OverlayEntry{}, false
}

func (m *OverlayManager) DismissTop() (OverlayEntry, bool) {
	if m == nil {
		return OverlayEntry{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := len(m.entries) - 1; index >= 0; index-- {
		if m.entries[index].DismissOnEscape {
			entry := m.entries[index]
			m.entries = append(m.entries[:index], m.entries[index+1:]...)
			return entry, true
		}
	}
	return OverlayEntry{}, false
}

// DismissFocusLoss removes every anchored overlay from top to bottom while
// leaving informational and modal overlays intact.
func (m *OverlayManager) DismissFocusLoss() []OverlayEntry {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	dismissed := make([]OverlayEntry, 0)
	for index := len(m.entries) - 1; index >= 0; index-- {
		if m.entries[index].DismissOnFocusLoss {
			dismissed = append(dismissed, m.entries[index])
		}
	}
	if len(dismissed) == 0 {
		return nil
	}
	kept := make([]OverlayEntry, 0, len(m.entries)-len(dismissed))
	for _, entry := range m.entries {
		if !entry.DismissOnFocusLoss {
			kept = append(kept, entry)
		}
	}
	m.entries = kept
	return dismissed
}

func (m *OverlayManager) Snapshot() []OverlayEntry {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]OverlayEntry, len(m.entries))
	copy(out, m.entries)
	return out
}

func (m *OverlayManager) HasModal() bool {
	for _, entry := range m.Snapshot() {
		if entry.Modal {
			return true
		}
	}
	return false
}

func (s *ApplicationState) OpenOverlay(id string, component Component, modal, dismissOnEscape bool) {
	s.openOverlay(id, component, modal, dismissOnEscape, modal, false, nil)
}

// OpenModal opens a modal overlay that owns keyboard focus, traps Tab
// navigation to its own subtree, and closes on Escape by default.
func (s *ApplicationState) OpenModal(id string, component Component, onDismiss func(*ApplicationState)) {
	s.openOverlay(id, component, true, true, true, false, onDismiss)
}

// OpenAnchoredOverlay opens a non-modal popup whose lifetime is tied to its
// current focus owner, such as a combo-box list or calendar.
func (s *ApplicationState) OpenAnchoredOverlay(id string, component Component, dismissOnEscape bool) {
	s.OpenAnchoredOverlayWithDismiss(id, component, dismissOnEscape, nil)
}

func (s *ApplicationState) OpenAnchoredOverlayWithDismiss(id string, component Component, dismissOnEscape bool, onDismiss func(*ApplicationState)) {
	s.openOverlay(id, component, false, dismissOnEscape, false, true, onDismiss)
}

// OpenFocusedOverlay opens a non-modal overlay that owns keyboard focus while
// it is visible, such as a menu. Informational overlays should use OpenOverlay.
func (s *ApplicationState) OpenFocusedOverlay(id string, component Component, dismissOnEscape bool) {
	s.openOverlay(id, component, false, dismissOnEscape, true, true, nil)
}

func (s *ApplicationState) openOverlay(id string, component Component, modal, dismissOnEscape, focus, dismissOnFocusLoss bool, onDismiss func(*ApplicationState)) {
	if id == "" || component == nil {
		return
	}
	if s.Overlays == nil {
		s.Overlays = NewOverlayManager()
	}
	restore := s.FocusedID
	s.Overlays.Open(OverlayEntry{ID: id, Component: component, Modal: modal, DismissOnEscape: dismissOnEscape, DismissOnFocusLoss: dismissOnFocusLoss, RestoreFocusID: restore, OnDismiss: onDismiss})
	if focus {
		s.FocusedID = ""
		component.Walk(func(child Component) {
			if s.FocusedID == "" && child.Focusable() {
				s.FocusedID = child.ID()
			}
		})
	}
}

func (s *ApplicationState) DismissFocusLossOverlays() int {
	return s.dismissFocusLossOverlays("", nil)
}

// DismissFocusLossOverlaysForTarget preserves an anchored overlay when focus
// remains on its owner or one of its semantic descendants.
func (s *ApplicationState) DismissFocusLossOverlaysForTarget(targetID string) int {
	return s.dismissFocusLossOverlays(targetID, nil)
}

// DismissFocusLossOverlaysForPointer additionally preserves the topmost
// anchored layer containing the pointer and all of its anchored ancestors.
func (s *ApplicationState) DismissFocusLossOverlaysForPointer(targetID string, point image.Point) int {
	return s.dismissFocusLossOverlays(targetID, &point)
}

func semanticTargetWithin(targetID, rootID string) bool {
	return targetID != "" && rootID != "" && (targetID == rootID || strings.HasPrefix(targetID, rootID+"/"))
}

func (s *ApplicationState) dismissFocusLossOverlays(targetID string, point *image.Point) int {
	if s == nil || s.Overlays == nil {
		return 0
	}
	entries := s.Overlays.Snapshot()
	dismissed := 0
	preserveAncestors := false
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		if !entry.DismissOnFocusLoss {
			continue
		}
		inside := preserveAncestors || semanticTargetWithin(targetID, entry.RestoreFocusID) || semanticTargetWithin(targetID, entry.Component.ID())
		if point != nil && point.In(entry.Component.Bounds()) {
			inside = true
		}
		if inside {
			preserveAncestors = true
			continue
		}
		if s.CloseOverlay(entry.ID) {
			dismissed++
		}
	}
	return dismissed
}

func (s *ApplicationState) CloseOverlay(id string) bool {
	if s.Overlays == nil {
		return false
	}
	entry, ok := s.Overlays.Close(id)
	if ok {
		s.FocusedID = entry.RestoreFocusID
		if entry.OnDismiss != nil {
			entry.OnDismiss(s)
		}
	}
	return ok
}

func (s *ApplicationState) DismissTopOverlay() (OverlayEntry, bool) {
	if s == nil || s.Overlays == nil {
		return OverlayEntry{}, false
	}
	entry, ok := s.Overlays.DismissTop()
	if ok {
		s.FocusedID = entry.RestoreFocusID
		if entry.OnDismiss != nil {
			entry.OnDismiss(s)
		}
	}
	return entry, ok
}
