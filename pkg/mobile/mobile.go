//go:build android

// Package mobile bridges the POEM engine to an in-process native presenter on
// Android. The C++ presenter (android_engine) hosts the process; the Go
// engine runs as a c-shared library inside it. Both sides speak the same
// POEM byte protocol used by the Windows host — same envelope and 4-byte
// length framing. Both native targets use the shared hosted in-memory runtime;
// this package adds Android exports and logcat integration.
//
// Call batching matters here: a cgo call costs far more than a Go call, so
// the presenter reads whole framed messages (one blocking PoemHostRead loop
// on a dedicated presenter thread) and writes one EventBatch per frame,
// rather than chatting per draw command.
package mobile

/*
#cgo LDFLAGS: -llog
#include <android/log.h>
#include <stdlib.h>

static void poemLog(const char* msg) {
    __android_log_write(ANDROID_LOG_INFO, "poem-go", msg);
}
*/
import "C"

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/mulavdm/poem/pkg/hosted"
	"github.com/mulavdm/poem/pkg/render"
)

// redirectStdio points fds 1 and 2 at a pipe drained into logcat: the engine
// logs progress with fmt.Printf and the Go runtime writes panics to stderr,
// both of which an APK otherwise discards.
func redirectStdio() {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	syscall.Dup3(int(w.Fd()), 1, 0)
	syscall.Dup3(int(w.Fd()), 2, 0)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logcat("%s", scanner.Text())
		}
	}()
}

// logcatWriter routes the Go engine's log output to Android logcat under the
// poem-go tag — stderr is a black hole inside an APK, so without this a Go
// panic or engine error is invisible.
type logcatWriter struct{}

func (logcatWriter) Write(p []byte) (int, error) {
	msg := C.CString(string(p))
	C.poemLog(msg)
	C.free(unsafe.Pointer(msg))
	return len(p), nil
}

func logcat(format string, args ...any) {
	w := logcatWriter{}
	w.Write([]byte(fmt.Sprintf(format, args...)))
}

var (
	startOnce sync.Once
	runtime   hosted.Runtime
)

// Start launches the engine against the in-process transport. It returns
// immediately; the engine loop runs on its own goroutines for the lifetime of
// the process. Width/height are the initial surface dimensions in physical
// pixels. Safe to call once; later calls are ignored (an Android activity can
// be re-created while the process lives on).
func Start(config render.AppConfig, width, height int) {
	startOnce.Do(func() {
		log.SetOutput(logcatWriter{})
		redirectStdio()
		logcat("engine starting %dx%d", width, height)
		if err := runtime.Start(config, width, height); err != nil {
			logcat("engine failed to start: %v", err)
		}
	})
}

// PoemHostRead copies up to capacity bytes of the engine→presenter stream
// into buf, blocking until data is available. Returns the byte count, or -1
// once the engine side closed. The presenter calls this from a dedicated
// transport thread and applies the standard 4-byte length framing itself.
//
//export PoemHostRead
func PoemHostRead(buf unsafe.Pointer, capacity C.int) C.int {
	if capacity <= 0 {
		return -1
	}
	dst := unsafe.Slice((*byte)(buf), int(capacity))
	n, err := runtime.Read(dst)
	if n > 0 {
		return C.int(n)
	}
	if err != nil {
		return -1
	}
	return 0
}

// PoemHostWrite feeds length bytes of presenter→engine stream (framed
// EventBatch messages) to the engine. Returns bytes consumed or -1 when the
// engine side is gone.
//
//export PoemHostWrite
func PoemHostWrite(buf unsafe.Pointer, length C.int) C.int {
	if length <= 0 {
		return -1
	}
	src := unsafe.Slice((*byte)(buf), int(length))
	n, err := runtime.Write(src)
	if err != nil {
		return -1
	}
	return C.int(n)
}
