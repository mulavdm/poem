//go:build windows

package windows

import (
	"context"
	"syscall"
	"unsafe"

	"go_native_gpu_gui/pkg/render/platform"
	"go_native_gpu_gui/pkg/render/theme"
)

const (
	spiGetHighContrast        = 0x0042
	spiGetClientAreaAnimation = 0x1042
	hcfHighContrastOn         = 0x00000001
	hkeyCurrentUser           = 0x80000001
	rrfRTRegDWORD             = 0x00000010
)

var (
	preferencesUser32         = syscall.NewLazyDLL("user32.dll")
	preferencesAdvapi         = syscall.NewLazyDLL("advapi32.dll")
	procSystemParametersInfoW = preferencesUser32.NewProc("SystemParametersInfoW")
	procRegGetValueW          = preferencesAdvapi.NewProc("RegGetValueW")
)

type highContrast struct {
	Size          uint32
	Flags         uint32
	DefaultScheme *uint16
}

type SystemPreferences struct{}

func NewSystemPreferences() *SystemPreferences { return &SystemPreferences{} }

func (p *SystemPreferences) Current(ctx context.Context) (platform.PreferenceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return platform.PreferenceSnapshot{}, err
	}
	high := highContrast{Size: uint32(unsafe.Sizeof(highContrast{}))}
	highResult, _, _ := procSystemParametersInfoW.Call(spiGetHighContrast, uintptr(high.Size), uintptr(unsafe.Pointer(&high)), 0)
	highContrastEnabled := highResult != 0 && high.Flags&hcfHighContrastOn != 0

	animationsEnabled := int32(1)
	animationResult, _, _ := procSystemParametersInfoW.Call(spiGetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&animationsEnabled)), 0)
	reducedMotion := animationResult != 0 && animationsEnabled == 0

	mode := theme.ModeDark
	if highContrastEnabled {
		mode = theme.ModeHighContrast
	} else if appsUseLightTheme() {
		mode = theme.ModeLight
	}
	if err := ctx.Err(); err != nil {
		return platform.PreferenceSnapshot{}, err
	}
	return platform.PreferenceSnapshot{ThemeMode: mode, ReducedMotion: reducedMotion}, nil
}

func appsUseLightTheme() bool {
	subkey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	valueName, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	var value uint32
	size := uint32(unsafe.Sizeof(value))
	result, _, _ := procRegGetValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(subkey)), uintptr(unsafe.Pointer(valueName)), rrfRTRegDWORD, 0, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
	return result == 0 && value != 0
}
