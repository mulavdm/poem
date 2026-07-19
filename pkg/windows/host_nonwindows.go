//go:build !windows

package windows

import (
	"errors"

	"github.com/mulavdm/poem/pkg/render"
)

// Register reports that the Windows native host is unavailable.
func Register(render.AppConfig, Metadata) error {
	return errors.New("Windows native host is unavailable")
}

// MustRegister panics because the Windows native host is unavailable.
func MustRegister(render.AppConfig, Metadata) { panic("Windows native host is unavailable") }
