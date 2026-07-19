// Package hosted runs the POEM Go engine inside a native presenter process.
package hosted

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/mulavdm/poem/pkg/render"
)

var (
	// ErrStarted reports that a process-global hosted engine was already started.
	ErrStarted = errors.New("hosted engine already started")
	// ErrNotStarted reports an operation before Start.
	ErrNotStarted = errors.New("hosted engine is not started")
)

// Runtime owns the in-memory protocol transport and engine lifecycle.
type Runtime struct {
	mu sync.RWMutex

	started bool
	stopped bool
	err     error
	done    chan struct{}

	presenterReader *io.PipeReader
	engineWriter    *io.PipeWriter
	engineReader    *io.PipeReader
	presenterWriter *io.PipeWriter
	run             func(render.AppConfig, io.ReadWriteCloser, io.ReadWriteCloser)
}

// Start launches one hosted engine and returns immediately.
func (r *Runtime) Start(config render.AppConfig, width, height int) error {
	if width <= 0 || height <= 0 {
		return errors.New("hosted engine dimensions must be positive")
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return ErrStarted
	}
	r.started = true
	r.done = make(chan struct{})
	r.presenterReader, r.engineWriter = io.Pipe()
	r.engineReader, r.presenterWriter = io.Pipe()
	toPresenter := splitConn{reader: r.presenterReader, writer: r.engineWriter}
	fromPresenter := splitConn{reader: r.engineReader, writer: r.presenterWriter}
	run := r.run
	if run == nil {
		run = render.RunHosted
	}
	r.mu.Unlock()

	config.Width, config.Height = width, height
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				r.mu.Lock()
				r.err = errors.New("hosted engine panic")
				r.mu.Unlock()
			}
			r.closePipes()
			r.mu.Lock()
			r.stopped = true
			close(r.done)
			r.mu.Unlock()
		}()
		run(config, toPresenter, fromPresenter)
	}()
	return nil
}

// Read copies engine-to-presenter protocol bytes, blocking until available.
func (r *Runtime) Read(buffer []byte) (int, error) {
	r.mu.RLock()
	reader, started := r.presenterReader, r.started
	r.mu.RUnlock()
	if !started || reader == nil {
		return 0, ErrNotStarted
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	return reader.Read(buffer)
}

// Write feeds presenter-to-engine protocol bytes.
func (r *Runtime) Write(buffer []byte) (int, error) {
	r.mu.RLock()
	writer, started := r.presenterWriter, r.started
	r.mu.RUnlock()
	if !started || writer == nil {
		return 0, ErrNotStarted
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	return writer.Write(buffer)
}

// Stop cooperatively disconnects the presenter and unblocks all transport IO.
func (r *Runtime) Stop() {
	r.closePipes()
}

// Wait waits for engine shutdown and returns its sanitized terminal error.
func (r *Runtime) Wait(timeout time.Duration) (bool, error) {
	r.mu.RLock()
	done, started := r.done, r.started
	r.mu.RUnlock()
	if !started || done == nil {
		return false, ErrNotStarted
	}
	if timeout <= 0 {
		<-done
	} else {
		select {
		case <-done:
		case <-time.After(timeout):
			return false, nil
		}
	}
	r.mu.RLock()
	err := r.err
	r.mu.RUnlock()
	return true, err
}

func (r *Runtime) closePipes() {
	r.mu.RLock()
	readFromEngine, writeToEngine := r.presenterReader, r.presenterWriter
	engineRead, engineWrite := r.engineReader, r.engineWriter
	r.mu.RUnlock()
	if writeToEngine != nil {
		_ = writeToEngine.Close()
	}
	if engineRead != nil {
		_ = engineRead.Close()
	}
	if engineWrite != nil {
		_ = engineWrite.Close()
	}
	if readFromEngine != nil {
		_ = readFromEngine.Close()
	}
}

type splitConn struct {
	reader io.Reader
	writer io.Writer
}

func (c splitConn) Read(buffer []byte) (int, error)  { return c.reader.Read(buffer) }
func (c splitConn) Write(buffer []byte) (int, error) { return c.writer.Write(buffer) }
func (splitConn) Close() error                       { return nil }
