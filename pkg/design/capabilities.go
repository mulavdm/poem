package design

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// CapabilityUpdate is the transport-safe presenter capability snapshot.
type CapabilityUpdate struct {
	Pointer        PointerPrecision `json:"pointer"`
	HoverAvailable bool             `json:"hover"`
	Keyboard       bool             `json:"keyboard"`
	Touch          bool             `json:"touch"`
	Trackpad       bool             `json:"trackpad"`
	Stylus         bool             `json:"stylus"`
	Density        Density          `json:"density"`
	TextScale      float32          `json:"textScale"`
	ReducedMotion  bool             `json:"reducedMotion"`
	HighContrast   bool             `json:"highContrast"`
}

// EncodeCapabilityUpdate validates and serializes a capability event.
func EncodeCapabilityUpdate(update CapabilityUpdate) (string, error) {
	if err := update.Validate(); err != nil {
		return "", err
	}
	value, err := json.Marshal(update)
	return string(value), err
}

// DecodeCapabilityUpdate strictly parses an untrusted presenter update.
func DecodeCapabilityUpdate(value string) (CapabilityUpdate, error) {
	var update CapabilityUpdate
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		return CapabilityUpdate{}, fmt.Errorf("decode capabilities: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return CapabilityUpdate{}, fmt.Errorf("decode capabilities: trailing data")
	}
	return update, update.Validate()
}

// Validate rejects enum and text-scale values outside the public contract.
func (update CapabilityUpdate) Validate() error {
	if update.Pointer > PointerFine || update.Density > DensityCompact {
		return fmt.Errorf("invalid capability enum")
	}
	if update.TextScale != 0 && (update.TextScale < 0.5 || update.TextScale > 4) {
		return fmt.Errorf("text scale outside 0.5-4")
	}
	return nil
}

// Input returns the normalized input-capability subset.
func (update CapabilityUpdate) Input() InputCapabilities {
	return InputCapabilities{Pointer: update.Pointer, HoverAvailable: update.HoverAvailable, Keyboard: update.Keyboard,
		Touch: update.Touch, Trackpad: update.Trackpad, Stylus: update.Stylus}
}
