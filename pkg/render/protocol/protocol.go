package protocol

const (
	Magic   = "POEM"
	Version = uint16(4)
)

type MessageType uint16

const (
	MessageInitEngine           MessageType = 1
	MessageRenderFrame          MessageType = 2
	MessagePlaySound            MessageType = 3
	MessageSemanticTree         MessageType = 4
	MessageFontAtlas            MessageType = 5
	MessageSetImeVisible        MessageType = 6
	MessageMapSceneDelta        MessageType = 7
	MessageMapCamera            MessageType = 8
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
	DrawCommandTypeDrawMapScene
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
	EventTypeCompositionStart
	EventTypeCompositionUpdate
	EventTypeCompositionEnd
	EventTypeSemanticAction
	EventTypePanGesture
	EventTypePinchGesture
	EventTypeCapabilities
	EventTypeMapCamera
	EventTypeMapFeature
	EventTypeMapFailure
)

type GesturePhase byte

const (
	GesturePhaseBegin GesturePhase = iota
	GesturePhaseUpdate
	GesturePhaseEnd
	GesturePhaseCancel
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

// SetImeVisible asks the presenter to show or hide the platform text-input
// method (the soft keyboard on Android). Presenters without an IME concept
// ignore it; the engine only emits it in hosted mode.
type SetImeVisible struct {
	Visible bool
}

type SemanticNode struct {
	Parent            int32
	ID                string
	Role              string
	Name              string
	Description       string
	AccessKey         string
	Value             string
	X1, Y1            int32
	X2, Y2            int32
	State             uint32
	HasRange          bool
	RangeMin          float64
	RangeMax          float64
	SmallChange       float64
	LargeChange       float64
	HasText           bool
	SelectionStart    int32
	SelectionEnd      int32
	Multiline         bool
	HasCollection     bool
	CanSelectMultiple bool
	SelectionRequired bool
	HasGrid           bool
	GridRows          int32
	GridColumns       int32
	HasGridItem       bool
	GridRow           int32
	GridColumn        int32
	GridRowSpan       int32
	GridColumnSpan    int32
	HasScroll         bool
	HScrollable       bool
	VScrollable       bool
	HScrollPercent    float64
	VScrollPercent    float64
	HViewSize         float64
	VViewSize         float64
	LabeledBy         []string
	DescribedBy       []string
	Controls          []string
	FlowsTo           []string
	Actions           []string
}

type SemanticTree struct {
	Revision uint64
	Nodes    []SemanticNode
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
	Text    string
	Target  string
	Action  string
	Value   string
	DeltaX  int32
	DeltaY  int32
	Scale   float32
	Phase   GesturePhase
}

type EventBatch struct {
	Events []Event
}

type MapResourceType byte

const (
	MapResourceVertexBuffer MapResourceType = iota
	MapResourceIndexBuffer
	MapResourceTextureRGBA
	MapResourceTextureSDF
	MapResourceTextureAlpha
)

type MapResourceOperation byte

const (
	MapResourceUpload MapResourceOperation = iota
	MapResourceRelease
)

type MapPrimitive byte

const (
	MapPrimitiveTriangles MapPrimitive = iota
	MapPrimitiveLines
	MapPrimitivePoints
)

type MapSceneResource struct {
	Operation MapResourceOperation
	Type      MapResourceType
	Hash      [32]byte
	Stride    uint32
	Width     uint32
	Height    uint32
	Bytes     []byte
}

type MapDrawBatch struct {
	VertexHash  [32]byte
	IndexHash   [32]byte
	TextureHash [32]byte
	Primitive   MapPrimitive
	First       uint32
	Count       uint32
	Layer       int32
	Opacity     float32
	DepthTest   bool
}

type MapCamera struct {
	Latitude, Longitude           float64
	Zoom, Bearing, Pitch          float32
	ViewportWidth, ViewportHeight uint32
}

// MapSceneDelta updates retained resources and the ordered draw list for one
// viewport generation. A presenter must discard deltas older than the newest
// accepted Generation for the same ViewportID.
type MapSceneDelta struct {
	ViewportID   string
	Generation   uint64
	Resources    []MapSceneResource
	Draws        []MapDrawBatch
	Camera       MapCamera
	SunAzimuth   float32
	SunElevation float32
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
