#pragma once

#include <cstdint>
#include <string>
#include <vector>

namespace poem::protocol {

constexpr char kMagic[4] = {'P', 'O', 'E', 'M'};
constexpr std::uint16_t kVersion = 1;

enum class MessageType : std::uint16_t {
    InitEngine = 1,
    RenderFrame = 2,
    PlaySound = 3,
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

struct PlaySound {
    SoundType type{};
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
PlaySound DecodePlaySound(const std::vector<std::uint8_t>& body);
NativeDebugRequest DecodeNativeDebugRequest(const std::vector<std::uint8_t>& body);
NativeDialogRequest DecodeNativeDialogRequest(const std::vector<std::uint8_t>& body);
std::vector<std::uint8_t> EncodeEventBatch(const EventBatch& batch);
std::vector<std::uint8_t> EncodeNativeDebugResponse(const NativeDebugResponse& response);
std::vector<std::uint8_t> EncodeNativeDialogResponse(const NativeDialogResponse& response);

} // namespace poem::protocol
