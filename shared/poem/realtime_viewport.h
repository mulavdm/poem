#pragma once
#include <cstdint>

namespace poem::realtime {
inline constexpr std::uint32_t kABIVersion = 2;
inline constexpr std::uint32_t kMaxSemanticBytes = 256 * 1024;
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
};
struct SemanticSnapshot { std::uint32_t structSize; const std::uint8_t* bytes; std::uint32_t byteCount; };
enum class Action : std::uint32_t { moveLeft=1, moveRight=2, confirm=3, cancel=4 };
struct ActionEvent { std::uint32_t structSize; Action action; std::uint32_t down; float value; };
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
};
inline Result Validate(const Exports* value) {
    if (!value || value->structSize < sizeof(Exports)) return Result::invalid_argument;
    if (value->abiVersion != kABIVersion) return Result::unsupported_version;
    if (!value->initialize || !value->render || !value->resize || !value->deviceLost ||
        !value->shutdown || !value->semanticSnapshot || !value->action) return Result::invalid_argument;
    return Result::ok;
}
inline Result ValidateSnapshot(const SemanticSnapshot* value) {
    if (!value || value->structSize < sizeof(SemanticSnapshot)) return Result::invalid_argument;
    if (value->byteCount > kMaxSemanticBytes || (value->byteCount && !value->bytes))
        return Result::invalid_argument;
    return Result::ok;
}
} // namespace poem::realtime
