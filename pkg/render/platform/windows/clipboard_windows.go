//go:build windows

// Package windows implements Windows platform services behind POEM's portable
// capability interfaces.
package windows

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodeText    = 13
	gmemMoveable     = 0x0002
	maxClipboardByte = 16 << 20
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	procGlobalFree       = kernel32.NewProc("GlobalFree")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	procGlobalSize       = kernel32.NewProc("GlobalSize")
)

type Clipboard struct{}

func NewClipboard() *Clipboard { return &Clipboard{} }

func openClipboard(ctx context.Context) error {
	for attempt := 0; attempt < 8; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		opened, _, callErr := procOpenClipboard.Call(0)
		if opened != 0 {
			return nil
		}
		if attempt == 7 {
			if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
				return fmt.Errorf("open clipboard: %w", callErr)
			}
			return errors.New("open clipboard: clipboard is busy")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return errors.New("open clipboard failed")
}

func (c *Clipboard) ReadText(ctx context.Context) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(ctx); err != nil {
		return "", err
	}
	defer procCloseClipboard.Call()

	handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
			return "", fmt.Errorf("get clipboard text: %w", callErr)
		}
		return "", nil
	}
	size, _, _ := procGlobalSize.Call(handle)
	if size == 0 {
		return "", nil
	}
	if size > maxClipboardByte {
		return "", fmt.Errorf("clipboard text exceeds %d bytes", maxClipboardByte)
	}
	pointer, _, lockErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return "", fmt.Errorf("lock clipboard text: %w", lockErr)
	}
	defer procGlobalUnlock.Call(handle)
	units := int(size / 2)
	buffer := unsafe.Slice((*uint16)(unsafe.Pointer(pointer)), units)
	length := 0
	for length < len(buffer) && buffer[length] != 0 {
		length++
	}
	return syscall.UTF16ToString(buffer[:length]), nil
}

func (c *Clipboard) WriteText(ctx context.Context, value string) error {
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return fmt.Errorf("encode clipboard text: %w", err)
	}
	byteSize := len(encoded) * 2
	if byteSize > maxClipboardByte {
		return fmt.Errorf("clipboard text exceeds %d bytes", maxClipboardByte)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(ctx); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if emptied, _, callErr := procEmptyClipboard.Call(); emptied == 0 {
		return fmt.Errorf("empty clipboard: %w", callErr)
	}
	handle, _, allocErr := procGlobalAlloc.Call(gmemMoveable, uintptr(byteSize))
	if handle == 0 {
		return fmt.Errorf("allocate clipboard text: %w", allocErr)
	}
	transferred := false
	defer func() {
		if !transferred {
			procGlobalFree.Call(handle)
		}
	}()
	pointer, _, lockErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return fmt.Errorf("lock clipboard allocation: %w", lockErr)
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(pointer)), len(encoded)), encoded)
	procGlobalUnlock.Call(handle)
	result, _, setErr := procSetClipboardData.Call(cfUnicodeText, handle)
	if result == 0 {
		return fmt.Errorf("set clipboard text: %w", setErr)
	}
	transferred = true
	return nil
}
