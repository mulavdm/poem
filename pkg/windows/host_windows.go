//go:build windows

package windows

/*
#include <stdint.h>
*/
import "C"

import (
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/mulavdm/poem/pkg/hosted"
	"github.com/mulavdm/poem/pkg/render"
)

var registration struct {
	sync.RWMutex
	set      bool
	config   render.AppConfig
	metadata []byte
	runtime  hosted.Runtime
}

// Register binds one application to the process-global native host ABI.
func Register(config render.AppConfig, metadata Metadata) error {
	encoded, err := encodeMetadata(metadata)
	if err != nil {
		return err
	}
	registration.Lock()
	defer registration.Unlock()
	if registration.set {
		return hosted.ErrStarted
	}
	registration.set = true
	registration.config = config
	registration.metadata = encoded
	return nil
}

// MustRegister is Register for application init functions.
func MustRegister(config render.AppConfig, metadata Metadata) {
	if err := Register(config, metadata); err != nil {
		panic(err)
	}
}

//export PoemWindowsABIVersion
func PoemWindowsABIVersion() C.uint32_t { return C.uint32_t(ABIVersion) }

//export PoemWindowsMetadata
func PoemWindowsMetadata(buffer unsafe.Pointer, capacity C.int32_t) C.int32_t {
	registration.RLock()
	metadata, set := registration.metadata, registration.set
	registration.RUnlock()
	if !set || len(metadata) == 0 {
		return -1
	}
	required := C.int32_t(len(metadata))
	if buffer == nil || capacity < required {
		return required
	}
	copy(unsafe.Slice((*byte)(buffer), int(capacity)), metadata)
	return required
}

//export PoemWindowsStart
func PoemWindowsStart(width, height C.int32_t) C.int32_t {
	registration.RLock()
	config, set := registration.config, registration.set
	registration.RUnlock()
	if !set {
		return -1
	}
	if err := registration.runtime.Start(config, int(width), int(height)); err != nil {
		return -2
	}
	return 0
}

//export PoemHostRead
func PoemHostRead(buffer unsafe.Pointer, capacity C.int32_t) C.int32_t {
	if buffer == nil || capacity <= 0 {
		return -1
	}
	n, err := registration.runtime.Read(unsafe.Slice((*byte)(buffer), int(capacity)))
	if err != nil {
		if err == io.EOF {
			return -1
		}
		return -1
	}
	return C.int32_t(n)
}

//export PoemHostWrite
func PoemHostWrite(buffer unsafe.Pointer, length C.int32_t) C.int32_t {
	if buffer == nil || length <= 0 {
		return -1
	}
	n, err := registration.runtime.Write(unsafe.Slice((*byte)(buffer), int(length)))
	if err != nil {
		return -1
	}
	return C.int32_t(n)
}

//export PoemWindowsStop
func PoemWindowsStop() {
	registration.runtime.Stop()
	_, _ = registration.runtime.Wait(2 * time.Second)
}
