#pragma once

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace poem::protocol {

constexpr char kMagic[4] = {'P', 'O', 'E', 'M'};
constexpr std::uint16_t kVersion = 4;

enum class MessageType : std::uint16_t {
    InitEngine = 1,
    RenderFrame = 2,
    PlaySound = 3,
    SemanticTree = 4,
    FontAtlas = 5,
    SetImeVisible = 6,
    MapSceneDelta = 7,
    MapCamera = 8,
    EventBatch = 101,
    NativeDebugRequest = 201,
    NativeDebugResponse = 202,
    NativeDialogRequest = 203,
    NativeDialogResponse = 204,
};

enum class DrawCommandType : std::uint8_t {
    DrawRoundedRect = 0,
    DrawText = 1,
    FillRect = 2,
    DrawLine = 3,
    SetGlow = 4,
    SetGlass = 5,
    SetShadow = 6,
    SetOffset = 7,
    SetClip = 8,
    DrawImage = 9,
    DrawMapScene = 10,
    DrawRealtimeViewport = 11,
};

enum class SoundType : std::uint8_t {
    Hover = 0,
    Click = 1,
    Success = 2,
};

enum class EventType : std::uint8_t {
    WindowClose = 0,
    WindowSize = 1,
    MouseDown = 2,
    MouseUp = 3,
    MouseMove = 4,
    MouseWheel = 5,
    KeyDown = 6,
    KeyUp = 7,
    KeyChar = 8,
    CompositionStart = 9,
    CompositionUpdate = 10,
    CompositionEnd = 11,
	SemanticAction = 12,
	PanGesture = 13,
	PinchGesture = 14,
	Capabilities = 15,
	MapCamera = 16,
	MapFeature = 17,
	MapFailure = 18,
};

enum class GesturePhase : std::uint8_t {
    Begin = 0,
    Update = 1,
    End = 2,
    Cancel = 3,
};

struct Envelope {
    MessageType type{};
    std::vector<std::uint8_t> body;
};

struct CharInfo {
    std::int32_t r{};
    float u1{};
    float v1{};
    float u2{};
    float v2{};
    std::int32_t width{};
    std::int32_t height{};
    std::int32_t advance{};
};

struct DrawCommand {
    DrawCommandType type{};
    std::int32_t x1{};
    std::int32_t y1{};
    std::int32_t x2{};
    std::int32_t y2{};
    std::int32_t w{};
    std::int32_t h{};
    std::int32_t radius{};
    std::uint8_t r{};
    std::uint8_t g{};
    std::uint8_t b{};
    std::uint8_t a{};
    std::string text;
    std::vector<std::uint8_t> bytes;
    float val1{};
    float val2{};
    float val3{};
    bool flag{};
};

struct InitEngine {
    std::int32_t width{};
    std::int32_t height{};
    std::int32_t atlasWidth{};
    std::int32_t atlasHeight{};
    std::vector<std::uint8_t> atlasPixels;
    std::vector<CharInfo> chars;
};

struct RenderFrame {
    std::int32_t width{};
    std::int32_t height{};
    std::uint8_t cursor{};
    std::vector<DrawCommand> commands;
};

enum class MapResourceType : std::uint8_t { VertexBuffer = 0, IndexBuffer = 1, TextureRGBA = 2, TextureSDF = 3, TextureAlpha = 4 };
enum class MapResourceOperation : std::uint8_t { Upload = 0, Release = 1 };
enum class MapPrimitive : std::uint8_t { Triangles = 0, Lines = 1, Points = 2 };

struct MapSceneResource {
    MapResourceOperation operation{};
    MapResourceType type{};
    std::array<std::uint8_t, 32> hash{};
    std::uint32_t stride{};
    std::uint32_t width{};
    std::uint32_t height{};
    std::vector<std::uint8_t> bytes;
};

struct MapDrawBatch {
    std::array<std::uint8_t, 32> vertexHash{};
    std::array<std::uint8_t, 32> indexHash{};
    std::array<std::uint8_t, 32> textureHash{};
    MapPrimitive primitive{};
    std::uint32_t first{};
    std::uint32_t count{};
    std::int32_t layer{};
    float opacity{};
    bool depthTest{};
};

struct MapCamera {
    double latitude{};
    double longitude{};
    float zoom{};
    float bearing{};
    float pitch{};
    std::uint32_t viewportWidth{};
    std::uint32_t viewportHeight{};
};

struct MapSceneDelta {
    std::string viewportId;
    std::uint64_t generation{};
    std::vector<MapSceneResource> resources;
    std::vector<MapDrawBatch> draws;
    MapCamera camera;
    float sunAzimuth{};
    float sunElevation{};
    // Height fog: density 0 disables it. Colour is the haze distant ground
    // fades into (normally the map.s own land tone).
    float fogDensity{};
    float fogRed{};
    float fogGreen{};
    float fogBlue{};
    // Shadow cascades: 0 off, 1 Balanced, 2 High.
    std::uint8_t shadowCascades{};
};

struct SetImeVisible {
    bool visible{};
};

struct PlaySound {
    SoundType type{};
};

struct SemanticNode {
    std::int32_t parent{-1};
    std::string id;
    std::string role;
    std::string name;
    std::string description;
	std::string accessKey;
    std::string value;
    std::int32_t x1{}, y1{}, x2{}, y2{};
    std::uint32_t state{};
	bool hasRange{};
	double rangeMin{};
	double rangeMax{};
	double smallChange{};
	double largeChange{};
	bool hasText{};
	std::int32_t selectionStart{};
	std::int32_t selectionEnd{};
	bool multiline{};
	bool hasCollection{};
	bool canSelectMultiple{};
	bool selectionRequired{};
	bool hasGrid{};
	std::int32_t gridRows{};
	std::int32_t gridColumns{};
	bool hasGridItem{};
	std::int32_t gridRow{};
	std::int32_t gridColumn{};
	std::int32_t gridRowSpan{};
	std::int32_t gridColumnSpan{};
	bool hasScroll{};
	bool hScrollable{};
	bool vScrollable{};
	double hScrollPercent{};
	double vScrollPercent{};
	double hViewSize{};
	double vViewSize{};
	std::vector<std::string> labeledBy;
	std::vector<std::string> describedBy;
	std::vector<std::string> controls;
	std::vector<std::string> flowsTo;
    std::vector<std::string> actions;
};

struct SemanticTree {
    std::uint64_t revision{};
    std::vector<SemanticNode> nodes;
};

struct Event {
    EventType type{};
    std::int32_t x{};
    std::int32_t y{};
    std::int32_t button{};
    std::int32_t delta{};
    std::uint32_t keycode{};
    std::uint32_t ch{};
    std::int32_t width{};
    std::int32_t height{};
    std::string text;
	std::string target;
	std::string action;
	std::string value;
	std::int32_t deltaX{};
	std::int32_t deltaY{};
	float scale{1.0f};
	GesturePhase phase{};
};

struct EventBatch {
    std::vector<Event> events;
};

struct NativeDebugRequest {
    bool captureFrame{};
    bool capturePresentedFrame{};
    bool captureDesktopFrame{};
    bool restoreWindow{};
    bool clampToWorkArea{};
    bool bringToForeground{};
    bool maximizeWindow{};
    // Clears the native timing rings after this response is built, so a
    // caller can scope a measurement to one driven scene.
    bool resetPerf{};
};

// NativePerfPhase mirrors poem::perf::PhaseStats on the wire. One entry per
// timed native channel (frame presentation, protocol decode, map-scene apply).
struct NativePerfPhase {
    std::string name;
    std::uint32_t count{};
    double meanMS{};
    double p50MS{};
    double p95MS{};
    double p99MS{};
    double maxMS{};
};

struct NativeDebugResponse {
    std::string error;

    std::int32_t dpi{};

    bool windowVisible{};
    bool windowMinimized{};
    bool windowForeground{};

    std::int32_t windowLeft{};
    std::int32_t windowTop{};
    std::int32_t windowRight{};
    std::int32_t windowBottom{};

    std::int32_t clientWidth{};
    std::int32_t clientHeight{};

    std::int32_t workLeft{};
    std::int32_t workTop{};
    std::int32_t workRight{};
    std::int32_t workBottom{};

    std::int32_t backbufferWidth{};
    std::int32_t backbufferHeight{};

    std::int32_t frameWidth{};
    std::int32_t frameHeight{};
    std::vector<std::uint8_t> frameRgba;

    std::vector<NativePerfPhase> perfPhases;
};

struct NativeDialogRequest {
    std::string kind;
    std::string title;
    std::string initialDir;
};

struct NativeDialogResponse {
    std::string error;
    bool canceled{};
    std::string path;
};

Envelope DecodeEnvelope(const std::vector<std::uint8_t>& payload);
InitEngine DecodeInitEngine(const std::vector<std::uint8_t>& body);
RenderFrame DecodeRenderFrame(const std::vector<std::uint8_t>& body);
MapSceneDelta DecodeMapSceneDelta(const std::vector<std::uint8_t>& body);
MapCamera DecodeMapCamera(const std::vector<std::uint8_t>& body);
PlaySound DecodePlaySound(const std::vector<std::uint8_t>& body);
SetImeVisible DecodeSetImeVisible(const std::vector<std::uint8_t>& body);
SemanticTree DecodeSemanticTree(const std::vector<std::uint8_t>& body);
NativeDebugRequest DecodeNativeDebugRequest(const std::vector<std::uint8_t>& body);
NativeDialogRequest DecodeNativeDialogRequest(const std::vector<std::uint8_t>& body);
std::vector<std::uint8_t> EncodeEventBatch(const EventBatch& batch);
std::vector<std::uint8_t> EncodeNativeDebugResponse(const NativeDebugResponse& response);
std::vector<std::uint8_t> EncodeNativeDialogResponse(const NativeDialogResponse& response);

} // namespace poem::protocol
