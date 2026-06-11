package protocol

const (
	Magic   = "POEM"
	Version = uint16(1)
)

type MessageType uint16

const (
	MessageInitEngine           MessageType = 1
	MessageRenderFrame          MessageType = 2
	MessagePlaySound            MessageType = 3
	MessageEventBatch           MessageType = 101
	MessageNativeDebugRequest   MessageType = 201
	MessageNativeDebugResponse  MessageType = 202
	MessageNativeDialogRequest  MessageType = 203
	MessageNativeDialogResponse MessageType = 204
)

type DrawCommandType byte

const (
	DrawCommandTypeDrawRoundedRect DrawCommandType = iota
	DrawCommandTypeDrawText
	DrawCommandTypeFillRect
	DrawCommandTypeDrawLine
	DrawCommandTypeSetGlow
	DrawCommandTypeSetGlass
	DrawCommandTypeSetShadow
	DrawCommandTypeSetOffset
	DrawCommandTypeSetClip
	DrawCommandTypeDrawImage
)

type SoundType byte

const (
	SoundTypeHover SoundType = iota
	SoundTypeClick
	SoundTypeSuccess
)

type EventType byte

const (
	EventTypeWindowClose EventType = iota
	EventTypeWindowSize
	EventTypeMouseDown
	EventTypeMouseUp
	EventTypeMouseMove
	EventTypeMouseWheel
	EventTypeKeyDown
	EventTypeKeyUp
	EventTypeKeyChar
)

type CharInfo struct {
	R       int32
	U1      float32
	V1      float32
	U2      float32
	V2      float32
	Width   int32
	Height  int32
	Advance int32
}

type DrawCommand struct {
	Type   DrawCommandType
	X1     int32
	Y1     int32
	X2     int32
	Y2     int32
	W      int32
	H      int32
	Radius int32
	R      byte
	G      byte
	B      byte
	A      byte
	Text   string
	Bytes  []byte
	Val1   float32
	Val2   float32
	Val3   float32
	Flag   bool
}

type InitEngine struct {
	Width       int32
	Height      int32
	AtlasWidth  int32
	AtlasHeight int32
	AtlasPixels []byte
	Chars       []CharInfo
}

type RenderFrame struct {
	Width    int32
	Height   int32
	Commands []DrawCommand
	Cursor   byte
}

type PlaySound struct {
	Type SoundType
}

type Event struct {
	Type    EventType
	X       int32
	Y       int32
	Button  int32
	Delta   int32
	Keycode uint32
	Char    uint32
	Width   int32
	Height  int32
}

type EventBatch struct {
	Events []Event
}

type NativeDebugRequest struct {
	CaptureFrame          bool
	CapturePresentedFrame bool
	CaptureDesktopFrame   bool
	RestoreWindow         bool
	ClampToWorkArea       bool
	BringToForeground     bool
	MaximizeWindow        bool
}

type NativeDebugResponse struct {
	Error string

	DPI int32

	WindowVisible    bool
	WindowMinimized  bool
	WindowForeground bool

	WindowLeft   int32
	WindowTop    int32
	WindowRight  int32
	WindowBottom int32

	ClientWidth  int32
	ClientHeight int32

	WorkLeft   int32
	WorkTop    int32
	WorkRight  int32
	WorkBottom int32

	BackbufferWidth  int32
	BackbufferHeight int32

	FrameWidth  int32
	FrameHeight int32
	FrameRGBA   []byte
}

type NativeDialogRequest struct {
	Kind       string
	Title      string
	InitialDir string
}

type NativeDialogResponse struct {
	Error    string
	Canceled bool
	Path     string
}
