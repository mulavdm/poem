//go:build windows

package render

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mulavdm/poem/internal/win32"
)

const (
	pipeNameGoToSidecar = `\\.\pipe\poem_ipc_go_to_sidecar`
	pipeNameSidecarToGo = `\\.\pipe\poem_ipc_sidecar_to_go`
)

// connectPresenter spawns the Windows native presentation sidecar and binds
// the two named pipes Run drives it through. The returned cleanup closes the
// pipes and reaps the sidecar process (with a bounded wait) so orphaned
// sidecars cannot hold the pipes or DLLs across a rebuild.
func connectPresenter(config AppConfig) (toPresenter, fromPresenter io.ReadWriteCloser, cleanup func()) {
	pipeNameGoToSidecarUTF16, _ := syscall.UTF16PtrFromString(pipeNameGoToSidecar)
	pipeHandleGoToSidecar, err := win32.CreateNamedPipe(
		pipeNameGoToSidecarUTF16,
		win32.PIPE_ACCESS_DUPLEX,
		win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
		win32.PIPE_UNLIMITED_INSTANCES,
		1024*1024, // 1MB Output buffer
		1024*1024, // 1MB Input buffer
		0,
		0,
	)
	if err != nil || pipeHandleGoToSidecar == 0 {
		panic(fmt.Sprintf("orchestrator failed to create Go->sidecar IPC pipe: %v", err))
	}

	pipeNameSidecarToGoUTF16, _ := syscall.UTF16PtrFromString(pipeNameSidecarToGo)
	pipeHandleSidecarToGo, err := win32.CreateNamedPipe(
		pipeNameSidecarToGoUTF16,
		win32.PIPE_ACCESS_DUPLEX,
		win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
		win32.PIPE_UNLIMITED_INSTANCES,
		1024*1024,
		1024*1024,
		0,
		0,
	)
	if err != nil || pipeHandleSidecarToGo == 0 {
		win32.CloseHandle(pipeHandleGoToSidecar)
		panic(fmt.Sprintf("orchestrator failed to create sidecar->Go IPC pipe: %v", err))
	}

	fmt.Println("IPC named pipes created. Awaiting native presentation sidecar connection...")

	sidecarPath, err := resolveSidecarPath()
	if err != nil {
		win32.CloseHandle(pipeHandleGoToSidecar)
		win32.CloseHandle(pipeHandleSidecarToGo)
		panic(fmt.Sprintf("failed to locate native presentation sidecar: %v", err))
	}
	fmt.Printf("Spawning native presentation sidecar: %s\n", sidecarPath)
	sidecarCmd := exec.Command(sidecarPath, config.Title)
	sidecarCmd.Dir = filepath.Dir(sidecarPath)

	sidecarCmd.Stdout = os.Stdout
	sidecarCmd.Stderr = os.Stderr
	if err := sidecarCmd.Start(); err != nil {
		win32.CloseHandle(pipeHandleGoToSidecar)
		win32.CloseHandle(pipeHandleSidecarToGo)
		panic(fmt.Sprintf("failed to spawn native presentation sidecar: %v", err))
	}

	connected1, err := win32.ConnectNamedPipe(pipeHandleGoToSidecar, 0)
	if err != nil || !connected1 {
		panic(fmt.Sprintf("failed to lock client connection on Go->sidecar named pipe: %v", err))
	}
	connected2, err := win32.ConnectNamedPipe(pipeHandleSidecarToGo, 0)
	if err != nil || !connected2 {
		panic(fmt.Sprintf("failed to lock client connection on sidecar->Go named pipe: %v", err))
	}
	fmt.Println("IPC handshake synchronized. Native presentation core bound successfully.")

	toPresenter = &pipeReadWriteCloser{handle: pipeHandleGoToSidecar}
	fromPresenter = &pipeReadWriteCloser{handle: pipeHandleSidecarToGo}
	cleanup = func() {
		if sidecarCmd.Process != nil {
			fmt.Println("Terminating native presentation sidecar...")
			_ = sidecarCmd.Process.Kill()
			done := make(chan struct{})
			go func() {
				_ = sidecarCmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				fmt.Println("Timed out waiting for native presentation sidecar to exit")
			}
		}
		win32.CloseHandle(pipeHandleGoToSidecar)
		win32.CloseHandle(pipeHandleSidecarToGo)
	}
	return toPresenter, fromPresenter, cleanup
}

// Named Pipe wrapper implementing io.ReadWriteCloser
type pipeReadWriteCloser struct {
	handle uintptr
}

func (p *pipeReadWriteCloser) Read(b []byte) (int, error) {
	var read uint32
	// Use Win32 ReadFile API under the hood
	err := syscall.ReadFile(syscall.Handle(p.handle), b, &read, nil)
	if err != nil {
		if err == syscall.ERROR_BROKEN_PIPE {
			return 0, io.EOF
		}
		return 0, err
	}
	return int(read), nil
}

func (p *pipeReadWriteCloser) Write(b []byte) (int, error) {
	var written uint32
	err := syscall.WriteFile(syscall.Handle(p.handle), b, &written, nil)
	if err != nil {
		return 0, err
	}
	return int(written), nil
}

func (p *pipeReadWriteCloser) Close() error {
	win32.DisconnectNamedPipe(p.handle)
	win32.CloseHandle(p.handle)
	return nil
}
