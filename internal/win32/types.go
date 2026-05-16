package win32

type WNDCLASS struct {
	Style, PfnWndProc                uintptr
	ClsExtra, WndExtra               int32
	HInstance, HIcon, HCursor, HbrBg uintptr
	MenuName, ClassName              *uint16
}

type BITMAPINFOHEADER struct {
	Size, Width, Height                         int32
	Planes, BitCount                            uint16
	Compression, SizeImage                      uint32
	XPelsPerMeter, YPelsPerMeter, ClrUsed, ClrI int32
}

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]uint32
}

type PIXELFORMATDESCRIPTOR struct {
	Size, Version                                                                                                                                                                                                                               uint16
	Flags                                                                                                                                                                                                                                       uint32
	PixelType, ColorBits, RedBits, RedShift, GreenBits, GreenShift, BlueBits, BlueShift, AlphaBits, AlphaShift, AccumBits, AccumRedBits, AccumGreenBits, AccumBlueBits, AccumAlphaBits, DepthBits, StencilBits, AuxBuffers, LayerType, Reserved uint8
	LayerMask, VisibleMask, DamageMask                                                                                                                                                                                                          uint32
}

type RECT struct {
	Left, Top, Right, Bottom int32
}
