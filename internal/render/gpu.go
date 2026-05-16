//go:build gpu

package render

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"go_native_gpu_gui/internal/win32"
)

type GPUEngine struct{}

func New(hdc uintptr) (UIRenderer, error) {
	fmt.Println("🚀 Factory: Spawning Hardware-Accelerated GPU Engine")
	
	pfd := win32.PIXELFORMATDESCRIPTOR{Flags: 4 | 32, PixelType: 0, ColorBits: 32, DepthBits: 24}
	pfd.Size = uint16(unsafe.Sizeof(pfd))
	pfd.Version = 1
	
	pf, err := win32.ChoosePixelFormat(hdc, &pfd)
	if err != nil {
		return nil, fmt.Errorf("failed to choose pixel format: %v", err)
	}
	
	err = win32.SetPixelFormat(hdc, pf, &pfd)
	if err != nil {
		return nil, fmt.Errorf("failed to set pixel format: %v", err)
	}
	
	hglrc, err := win32.WglCreateContext(hdc)
	if err != nil {
		return nil, fmt.Errorf("failed to create wgl context: %v", err)
	}
	
	err = win32.WglMakeCurrent(hdc, hglrc)
	if err != nil {
		return nil, fmt.Errorf("failed to make wgl context current: %v", err)
	}

	return &GPUEngine{}, nil
}

func (g *GPUEngine) Setup(hdc uintptr) error {
	if err := gl.Init(); err != nil {
		return err
	}
	fmt.Printf("Successfully initialized: Hardware GPU Engine (%s)\n", gl.GoStr(gl.GetString(gl.RENDERER)))
	return nil
}

func (g *GPUEngine) Paint(hdc uintptr, state *ApplicationState) {
	// 1. Draw frame backgrounds on the graphics card
	gl.ClearColor(0.09, 0.58, 0.95, 1.0) // Corporate Premium Blue
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// 2. [Future GLSL draw calls]

	// 3. Swap buffers
	win32.SwapBuffers(hdc)
}
