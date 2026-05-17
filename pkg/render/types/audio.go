package types

import (
	"time"
	"unsafe"

	"go_native_gpu_gui/internal/win32"
)

// PlayHover chimes the ultra-short mouse hover tick asynchronously if audio is enabled and cooled down.
func (s *ApplicationState) PlayHover() {
	if s.AudioEnabled && len(s.AudioHoverBuffer) > 0 {
		now := time.Now()
		if now.Sub(s.LastHoverTime) < 50*time.Millisecond {
			return
		}
		s.LastHoverTime = now
		go win32.PlaySound(
			uintptr(unsafe.Pointer(&s.AudioHoverBuffer[0])),
			0,
			win32.SND_MEMORY|win32.SND_ASYNC|win32.SND_NODEFAULT,
		)
	}
}

// PlayClick chimes the high-fidelity click dual-sine wave asynchronously if audio is enabled and cooled down.
func (s *ApplicationState) PlayClick() {
	if s.AudioEnabled && len(s.AudioClickBuffer) > 0 {
		now := time.Now()
		if now.Sub(s.LastClickTime) < 150*time.Millisecond {
			return
		}
		s.LastClickTime = now
		go win32.PlaySound(
			uintptr(unsafe.Pointer(&s.AudioClickBuffer[0])),
			0,
			win32.SND_MEMORY|win32.SND_ASYNC|win32.SND_NODEFAULT,
		)
	}
}

// PlaySuccess chimes the ascending sci-fi arpeggio melody asynchronously if audio is enabled and cooled down.
func (s *ApplicationState) PlaySuccess() {
	if s.AudioEnabled && len(s.AudioSavedBuffer) > 0 {
		now := time.Now()
		if now.Sub(s.LastSuccessTime) < 500*time.Millisecond {
			return
		}
		s.LastSuccessTime = now
		go win32.PlaySound(
			uintptr(unsafe.Pointer(&s.AudioSavedBuffer[0])),
			0,
			win32.SND_MEMORY|win32.SND_ASYNC|win32.SND_NODEFAULT,
		)
	}
}
