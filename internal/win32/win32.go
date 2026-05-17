package win32

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	opengl32 = syscall.NewLazyDLL("opengl32.dll")

	procRegisterClass  = user32.NewProc("RegisterClassW")
	procCreateWindow   = user32.NewProc("CreateWindowExW")
	procGetDC          = user32.NewProc("GetDC")
	procDefWindowProc  = user32.NewProc("DefWindowProcW")
	procGetMessage     = user32.NewProc("GetMessageW")
	procTranslateMsg   = user32.NewProc("TranslateMessage")
	procDispatchMsg    = user32.NewProc("DispatchMessageW")
	procPostQuitMsg    = user32.NewProc("PostQuitMessage")
	procPostMessage    = user32.NewProc("PostMessageW")
	procShowWindow     = user32.NewProc("ShowWindow")
	procInvalidateRect = user32.NewProc("InvalidateRect")
	procValidateRect   = user32.NewProc("ValidateRect")
	procGetKeyState    = user32.NewProc("GetKeyState")
	procSetCapture     = user32.NewProc("SetCapture")
	procReleaseCapture = user32.NewProc("ReleaseCapture")
	procSetCursor      = user32.NewProc("SetCursor")
	procLoadCursor     = user32.NewProc("LoadCursorW")
	procUpdateWindow   = user32.NewProc("UpdateWindow")

	procStretchDIBits = gdi32.NewProc("StretchDIBits")
	procChoosePF      = gdi32.NewProc("ChoosePixelFormat")
	procSetPF         = gdi32.NewProc("SetPixelFormat")
	procSwapBuffers   = gdi32.NewProc("SwapBuffers")

	procWglCreateCtx = opengl32.NewProc("wglCreateContext")
	procWglMakeCur   = opengl32.NewProc("wglMakeCurrent")
)

func RegisterClass(wc *WNDCLASS) (uintptr, error) {
	ret, _, err := procRegisterClass.Call(uintptr(unsafe.Pointer(wc)))
	if ret == 0 {
		return 0, err
	}
	return ret, nil
}

func CreateWindow(className, windowName *uint16, style uint32, x, y, w, h int32) (uintptr, error) {
	ret, _, err := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, 0, 0)
	if ret == 0 {
		return 0, err
	}
	return ret, nil
}

func GetDC(hwnd uintptr) (uintptr, error) {
	ret, _, err := procGetDC.Call(hwnd)
	if ret == 0 {
		return 0, err
	}
	return ret, nil
}

func DefWindowProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret
}

func GetMessage(msg *uintptr) (int32, error) {
	ret, _, err := procGetMessage.Call(uintptr(unsafe.Pointer(msg)), 0, 0, 0)
	if int32(ret) == -1 {
		return -1, err
	}
	return int32(ret), nil
}

func TranslateMessage(msg *uintptr) {
	procTranslateMsg.Call(uintptr(unsafe.Pointer(msg)))
}

func DispatchMessage(msg *uintptr) {
	procDispatchMsg.Call(uintptr(unsafe.Pointer(msg)))
}

func PostQuitMessage(exitCode int32) {
	procPostQuitMsg.Call(uintptr(exitCode))
}

func PostMessage(hwnd uintptr, msg uint32, wparam, lparam uintptr) bool {
	ret, _, _ := procPostMessage.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret != 0
}

func ShowWindow(hwnd uintptr, cmdShow int32) {
	procShowWindow.Call(hwnd, uintptr(cmdShow))
}

func InvalidateRect(hwnd uintptr, rect *RECT, erase bool) {
	eraseVal := 0
	if erase {
		eraseVal = 1
	}
	procInvalidateRect.Call(hwnd, uintptr(unsafe.Pointer(rect)), uintptr(eraseVal))
}

func UpdateWindow(hwnd uintptr) {
	procUpdateWindow.Call(hwnd)
}

func ValidateRect(hwnd uintptr, rect *RECT) {
	procValidateRect.Call(hwnd, uintptr(unsafe.Pointer(rect)))
}

func GetKeyState(nVirtKey int32) int16 {
	ret, _, _ := procGetKeyState.Call(uintptr(nVirtKey))
	return int16(ret)
}

func SetCapture(hwnd uintptr) uintptr {
	ret, _, _ := procSetCapture.Call(hwnd)
	return ret
}

func ReleaseCapture() bool {
	ret, _, _ := procReleaseCapture.Call()
	return ret != 0
}

func SetCursor(hcursor uintptr) uintptr {
	ret, _, _ := procSetCursor.Call(hcursor)
	return ret
}

func LoadCursor(hInstance uintptr, cursorName uintptr) uintptr {
	ret, _, _ := procLoadCursor.Call(hInstance, cursorName)
	return ret
}

const (
	IDC_ARROW       = 32512
	IDC_IBEAM       = 32513
	IDC_WAIT        = 32514
	IDC_CROSS       = 32515
	IDC_UPARROW     = 32516
	IDC_SIZE        = 32640
	IDC_ICON        = 32641
	IDC_SIZENWSE    = 32642
	IDC_SIZENESW    = 32643
	IDC_SIZEWE      = 32644
	IDC_SIZENS      = 32645
	IDC_SIZEALL     = 32646
	IDC_NO          = 32648
	IDC_HAND        = 32649
	IDC_APPSTARTING = 32650
	IDC_HELP        = 32651
)

func StretchDIBits(hdc uintptr, xDest, yDest, wDest, hDest, xSrc, ySrc, wSrc, hSrc int32, bits uintptr, bmi *BITMAPINFO) {
	procStretchDIBits.Call(
		hdc, uintptr(xDest), uintptr(yDest), uintptr(wDest), uintptr(hDest),
		uintptr(xSrc), uintptr(ySrc), uintptr(wSrc), uintptr(hSrc),
		bits, uintptr(unsafe.Pointer(bmi)), 0, 0x00CC0020,
	)
}

func ChoosePixelFormat(hdc uintptr, pfd *PIXELFORMATDESCRIPTOR) (int32, error) {
	ret, _, err := procChoosePF.Call(hdc, uintptr(unsafe.Pointer(pfd)))
	if ret == 0 {
		return 0, err
	}
	return int32(ret), nil
}

func SetPixelFormat(hdc uintptr, format int32, pfd *PIXELFORMATDESCRIPTOR) error {
	ret, _, err := procSetPF.Call(hdc, uintptr(format), uintptr(unsafe.Pointer(pfd)))
	if ret == 0 {
		return err
	}
	return nil
}

func SwapBuffers(hdc uintptr) {
	procSwapBuffers.Call(hdc)
}

func WglCreateContext(hdc uintptr) (uintptr, error) {
	ret, _, err := procWglCreateCtx.Call(hdc)
	if ret == 0 {
		return 0, err
	}
	return ret, nil
}

func WglMakeCurrent(hdc, hglrc uintptr) error {
	ret, _, err := procWglMakeCur.Call(hdc, hglrc)
	if ret == 0 {
		return err
	}
	return nil
}

func GET_X_LPARAM(lp uintptr) int16 {
	return int16(lp & 0xFFFF)
}

func GET_Y_LPARAM(lp uintptr) int16 {
	return int16(lp >> 16)
}
