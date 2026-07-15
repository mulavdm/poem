//go:build windows

package render

import (
	"fmt"
	"syscall"

	"github.com/mulavdm/poem/internal/win32"
)

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

			conn := &pipeReadWriteCloser{handle: handle}
			handleAutomationConnection(conn, cfg)
			_ = conn.Close()
		}
	}()
}
