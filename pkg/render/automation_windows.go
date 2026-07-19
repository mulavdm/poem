//go:build windows

package render

import (
	"fmt"
	"io"
	"syscall"

	"github.com/mulavdm/poem/internal/win32"
)

type automationPipeConnection struct{ handle uintptr }

func (p *automationPipeConnection) Read(buffer []byte) (int, error) {
	var read uint32
	if err := syscall.ReadFile(syscall.Handle(p.handle), buffer, &read, nil); err != nil {
		if err == syscall.ERROR_BROKEN_PIPE {
			return 0, io.EOF
		}
		return 0, err
	}
	return int(read), nil
}

func (p *automationPipeConnection) Write(buffer []byte) (int, error) {
	var written uint32
	if err := syscall.WriteFile(syscall.Handle(p.handle), buffer, &written, nil); err != nil {
		return 0, err
	}
	return int(written), nil
}

func (p *automationPipeConnection) Close() error {
	win32.DisconnectNamedPipe(p.handle)
	win32.CloseHandle(p.handle)
	return nil
}

// startPipeAutomationServer serves the automation protocol over a Windows
// named pipe. Pipe mode is Windows-only; other platforms use HTTP mode.
func startPipeAutomationServer(cfg AutomationConfig) {
	go func() {
		for {
			pipeNameUTF16, _ := syscall.UTF16PtrFromString(cfg.PipeName)
			handle, err := win32.CreateNamedPipe(
				pipeNameUTF16,
				win32.PIPE_ACCESS_DUPLEX,
				win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
				win32.PIPE_UNLIMITED_INSTANCES,
				1024*1024,
				1024*1024,
				0,
				0,
			)
			if err != nil || handle == 0 {
				fmt.Printf("POEM automation pipe create failed: %v\n", err)
				return
			}

			connected, err := win32.ConnectNamedPipe(handle, 0)
			if err != nil || !connected {
				win32.CloseHandle(handle)
				continue
			}

			conn := &automationPipeConnection{handle: handle}
			handleAutomationConnection(conn, cfg)
			_ = conn.Close()
		}
	}()
}
