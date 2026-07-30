#pragma once
#include <cstddef>
#include <cstdint>

namespace poem::realtime {
inline constexpr std::uint32_t kABIVersion = 4;
inline constexpr std::uint32_t kLegacyABIVersion = 3;
inline constexpr std::uint32_t kMaxSemanticBytes = 256 * 1024;
inline constexpr std::uint32_t kMaxMessageBytes = 1024 * 1024;
enum class Result : std::uint32_t { ok, invalid_argument, unsupported_version, device_error, cancelled };
struct Rect { std::int32_t x, y, width, height; };
struct FrameInput {
    std::uint32_t structSize;
    std::uint64_t frameID;
    float deltaSeconds;
    Rect viewport;
    const void* nativeDevice;
    const void* nativeCommandQueue;
    void* nativeCommandList;
    std::uint32_t renderTargetFormat;
    std::uint32_t flags;
    // DXGI_FORMAT of a depth-stencil buffer already bound by the caller
    // alongside the render target (0 = none). Mirrors
    // HamsterViewportInput::depth_stencil_format byte-for-byte.
    std::uint32_t depthStencilFormat;
    // D3D12_CPU_DESCRIPTOR_HANDLE::ptr of the render target (and, below,
    // depth-stencil view) already bound by the caller (0 = not supplied).
    // Lets a sub-pass that must temporarily rebind OM state (e.g. a shadow
    // map) restore the caller's binding before returning. Mirrors
    // HamsterViewportInput::render_target_view/depth_stencil_view
    // byte-for-byte.
    std::uint64_t renderTargetView;
    std::uint64_t depthStencilView;
};
struct SemanticSnapshot { std::uint32_t structSize; const std::uint8_t* bytes; std::uint32_t byteCount; };
enum class Action : std::uint32_t { moveLeft=1, moveRight=2, confirm=3, cancel=4 };
struct ActionEvent { std::uint32_t structSize; Action action; std::uint32_t down; float value; };
struct Command { std::uint32_t structSize; const std::uint8_t* bytes; std::uint32_t byteCount; };
struct EventBuffer { std::uint32_t structSize; std::uint8_t* bytes; std::uint32_t capacity; std::uint32_t byteCount; };
struct Exports {
    std::uint32_t structSize;
    std::uint32_t abiVersion;
    void* userData;
    Result (*initialize)(void*, const FrameInput*);
    Result (*render)(void*, const FrameInput*);
    void (*resize)(void*, Rect);
    void (*deviceLost)(void*);
    void (*shutdown)(void*);
    Result (*semanticSnapshot)(void*, SemanticSnapshot*);
    Result (*action)(void*, const ActionEvent*);
    Result (*submitCommand)(void*, const Command*);
    Result (*pollEvent)(void*, EventBuffer*);
};
inline Result Validate(const Exports* value) {
    constexpr auto legacySize=offsetof(Exports,submitCommand);
    if (!value || value->structSize < legacySize) return Result::invalid_argument;
    if (value->abiVersion != kABIVersion && value->abiVersion != kLegacyABIVersion) return Result::unsupported_version;
    if (!value->initialize || !value->render || !value->resize || !value->deviceLost ||
        !value->shutdown || !value->semanticSnapshot || !value->action) return Result::invalid_argument;
    if(value->abiVersion==kABIVersion &&
       (value->structSize<sizeof(Exports)||!value->submitCommand||!value->pollEvent))
        return Result::invalid_argument;
    return Result::ok;
}
inline Result Validate(const Command* value) {
    if(!value||value->structSize<sizeof(Command)||value->byteCount>kMaxMessageBytes||
       (value->byteCount&&!value->bytes))return Result::invalid_argument;
    return Result::ok;
}
inline Result Validate(const EventBuffer* value) {
    if(!value||value->structSize<sizeof(EventBuffer)||value->capacity>kMaxMessageBytes||
       (value->capacity&&!value->bytes)||value->byteCount>value->capacity)return Result::invalid_argument;
    return Result::ok;
}
inline Result ValidateSnapshot(const SemanticSnapshot* value) {
    if (!value || value->structSize < sizeof(SemanticSnapshot)) return Result::invalid_argument;
    if (value->byteCount > kMaxSemanticBytes || (value->byteCount && !value->bytes))
        return Result::invalid_argument;
    return Result::ok;
}
} // namespace poem::realtime
