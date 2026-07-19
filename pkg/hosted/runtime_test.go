package hosted

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mulavdm/poem/pkg/render"
)

func testRuntime(run func(render.AppConfig, io.ReadWriteCloser, io.ReadWriteCloser)) *Runtime {
	return &Runtime{run: run}
}

func TestRuntimeRejectsInvalidAndRepeatedStart(t *testing.T) {
	var runtime Runtime
	if err := runtime.Start(render.AppConfig{}, 0, 1); err == nil {
		t.Fatal("invalid dimensions accepted")
	}
	if err := runtime.Start(render.AppConfig{}, 64, 64); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Start(render.AppConfig{}, 64, 64); !errors.Is(err, ErrStarted) {
		t.Fatalf("second start = %v, want ErrStarted", err)
	}
	runtime.Stop()
	if stopped, _ := runtime.Wait(2 * time.Second); !stopped {
		t.Fatal("runtime did not stop")
	}
}

func TestRuntimeReadWriteBeforeStart(t *testing.T) {
	var runtime Runtime
	if _, err := runtime.Read(make([]byte, 1)); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Read error = %v", err)
	}
	if _, err := runtime.Write([]byte{1}); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Write error = %v", err)
	}
}

func TestRuntimeReportsEOFAndSanitizedPanic(t *testing.T) {
	normal := testRuntime(func(render.AppConfig, io.ReadWriteCloser, io.ReadWriteCloser) {})
	if err := normal.Start(render.AppConfig{}, 320, 240); err != nil {
		t.Fatal(err)
	}
	if stopped, err := normal.Wait(time.Second); !stopped || err != nil {
		t.Fatalf("normal Wait = %v, %v", stopped, err)
	}
	if _, err := normal.Read(make([]byte, 1)); !errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, io.EOF) {
		t.Fatalf("Read after exit = %v", err)
	}

	panicking := testRuntime(func(render.AppConfig, io.ReadWriteCloser, io.ReadWriteCloser) {
		panic("secret panic detail")
	})
	if err := panicking.Start(render.AppConfig{}, 320, 240); err != nil {
		t.Fatal(err)
	}
	stopped, err := panicking.Wait(time.Second)
	if !stopped || err == nil || err.Error() != "hosted engine panic" {
		t.Fatalf("panic Wait = %v, %v", stopped, err)
	}
}

func TestRuntimeConcurrentStopUnblocksIO(t *testing.T) {
	runtime := testRuntime(func(_ render.AppConfig, _ io.ReadWriteCloser, events io.ReadWriteCloser) {
		_, _ = events.Read(make([]byte, 1))
	})
	if err := runtime.Start(render.AppConfig{}, 320, 240); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			runtime.Stop()
		}()
	}
	group.Wait()
	if stopped, err := runtime.Wait(time.Second); !stopped || err != nil {
		t.Fatalf("Wait after Stop = %v, %v", stopped, err)
	}
}

func TestRuntimeWaitTimeout(t *testing.T) {
	release := make(chan struct{})
	runtime := testRuntime(func(render.AppConfig, io.ReadWriteCloser, io.ReadWriteCloser) { <-release })
	if err := runtime.Start(render.AppConfig{}, 320, 240); err != nil {
		t.Fatal(err)
	}
	if stopped, err := runtime.Wait(time.Millisecond); stopped || err != nil {
		t.Fatalf("Wait before release = %v, %v", stopped, err)
	}
	close(release)
	if stopped, err := runtime.Wait(time.Second); !stopped || err != nil {
		t.Fatalf("Wait after release = %v, %v", stopped, err)
	}
}

func TestSplitConnPartialIO(t *testing.T) {
	reader, writer := io.Pipe()
	conn := splitConn{reader: reader, writer: writer}
	done := make(chan []byte, 1)
	go func() {
		buffer := make([]byte, 3)
		first, _ := conn.Read(buffer[:2])
		second, _ := conn.Read(buffer[first:])
		done <- buffer[:first+second]
	}()
	if n, err := conn.Write([]byte{1, 2, 3}); err != nil || n != 3 {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got := <-done; len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("partial read = %v", got)
	}
}
