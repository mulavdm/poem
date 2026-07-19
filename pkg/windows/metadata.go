// Package windows exposes POEM's application registration contract for the
// single-process native Windows host.
package windows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

const (
	// ABIVersion is the native host/export-table compatibility version.
	ABIVersion    uint32 = 1
	minWindowSize        = 160
	maxWindowSize        = 16384
)

var identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{2,127}$`)

// Metadata describes an application before its Go engine starts.
type Metadata struct {
	// Identity is the stable application/package identity.
	Identity string `json:"identity"`
	// Title is the user-visible native window title.
	Title string `json:"title"`
	// Width is the preferred initial logical client width.
	Width int `json:"width"`
	// Height is the preferred initial logical client height.
	Height int `json:"height"`
}

// Validate checks the native host trust-boundary contract.
func (m Metadata) Validate() error {
	if !identityPattern.MatchString(m.Identity) {
		return fmt.Errorf("invalid Windows application identity")
	}
	if m.Title == "" || len(m.Title) > 256 {
		return fmt.Errorf("invalid Windows application title")
	}
	if m.Width < minWindowSize || m.Width > maxWindowSize || m.Height < minWindowSize || m.Height > maxWindowSize {
		return fmt.Errorf("Windows application dimensions outside %d-%d", minWindowSize, maxWindowSize)
	}
	return nil
}

func encodeMetadata(m Metadata) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(m); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}
