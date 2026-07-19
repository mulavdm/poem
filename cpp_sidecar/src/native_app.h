#pragma once

#include <windows.h>

#include <cstdint>
#include <string>
#include <vector>

namespace poem::host {

struct AppMetadata {
    std::string identity;
    std::wstring title;
    std::int32_t width = 0;
    std::int32_t height = 0;
};

class NativeApp {
public:
    NativeApp() = default;
    NativeApp(const NativeApp&) = delete;
    NativeApp& operator=(const NativeApp&) = delete;

    void LoadAdjacent();
    const AppMetadata& Metadata() const noexcept { return metadata_; }
    void Start(std::int32_t width, std::int32_t height);
    void Stop() noexcept;
    bool Started() const noexcept { return started_; }

    std::vector<std::uint8_t> ReadMessage();
    void WriteMessage(const std::vector<std::uint8_t>& payload);

private:
    using AbiVersionFn = std::uint32_t(__cdecl*)();
    using MetadataFn = std::int32_t(__cdecl*)(void*, std::int32_t);
    using StartFn = std::int32_t(__cdecl*)(std::int32_t, std::int32_t);
    using ReadFn = std::int32_t(__cdecl*)(void*, std::int32_t);
    using WriteFn = std::int32_t(__cdecl*)(const void*, std::int32_t);
    using StopFn = void(__cdecl*)();

    HMODULE module_ = nullptr; // Go runtimes are intentionally never unloaded.
    AbiVersionFn abiVersion_ = nullptr;
    MetadataFn metadataFn_ = nullptr;
    StartFn start_ = nullptr;
    ReadFn read_ = nullptr;
    WriteFn write_ = nullptr;
    StopFn stop_ = nullptr;
    AppMetadata metadata_;
    bool started_ = false;

    void ReadExact(void* buffer, std::uint32_t bytes);
    void WriteExact(const void* buffer, std::uint32_t bytes);
};

AppMetadata ParseMetadata(const std::string& json);

} // namespace poem::host
