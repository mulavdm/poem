//go:build windows

package win32

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreateNamedPipe     = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe    = kernel32.NewProc("ConnectNamedPipe")
	procDisconnectNamedPipe = kernel32.NewProc("DisconnectNamedPipe")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
)

const (
	PIPE_ACCESS_DUPLEX       = 0x00000003
	PIPE_TYPE_BYTE           = 0x00000000
	PIPE_READMODE_BYTE       = 0x00000000
	PIPE_WAIT                = 0x00000000
	PIPE_UNLIMITED_INSTANCES = 255
)

func CreateNamedPipe(name *uint16, openMode, pipeMode, maxInstances, outBufferSize, inBufferSize, defaultTimeOut uintptr, securityAttributes uintptr) (uintptr, error) {
	ret, _, err := procCreateNamedPipe.Call(
		uintptr(unsafe.Pointer(name)),
		openMode,
		pipeMode,
		maxInstances,
		outBufferSize,
		inBufferSize,
		defaultTimeOut,
		securityAttributes,
	)
	if ret == ^uintptr(0) { // INVALID_HANDLE_VALUE (-1)
		return 0, err
	}
	return ret, nil
}

func ConnectNamedPipe(handle uintptr, overlapped uintptr) (bool, error) {
	ret, _, err := procConnectNamedPipe.Call(handle, overlapped)
	if ret == 0 {
		// If ERROR_PIPE_CONNECTED (535) is returned, the client is already connected.
		if errno, ok := err.(syscall.Errno); ok && errno == 535 {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func DisconnectNamedPipe(handle uintptr) bool {
	ret, _, _ := procDisconnectNamedPipe.Call(handle)
	return ret != 0
}

func CloseHandle(handle uintptr) bool {
	ret, _, _ := procCloseHandle.Call(handle)
	return ret != 0
}
