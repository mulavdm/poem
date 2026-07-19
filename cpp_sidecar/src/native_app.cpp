#include "native_app.h"

#include <algorithm>
#include <charconv>
#include <filesystem>
#include <limits>
#include <stdexcept>

namespace poem::host {
namespace {

constexpr std::uint32_t kABIVersion = 1;
constexpr std::uint32_t kMaxMessageBytes = 64u * 1024u * 1024u;
constexpr std::int32_t kMaxMetadataBytes = 4096;

template <typename T>
T RequiredExport(HMODULE module, const char* name) {
    auto address = GetProcAddress(module, name);
    if (!address) throw std::runtime_error(std::string("missing Go application export: ") + name);
    return reinterpret_cast<T>(address);
}

std::size_t ValueStart(const std::string& json, const char* key) {
    const std::string needle = std::string("\"") + key + "\"";
    const auto keyPos = json.find(needle);
    if (keyPos == std::string::npos) throw std::runtime_error(std::string("metadata missing ") + key);
    auto pos = json.find(':', keyPos + needle.size());
    if (pos == std::string::npos) throw std::runtime_error("malformed application metadata");
    do { ++pos; } while (pos < json.size() && (json[pos] == ' ' || json[pos] == '\t' || json[pos] == '\r' || json[pos] == '\n'));
    return pos;
}

std::string JsonString(const std::string& json, const char* key) {
    auto pos = ValueStart(json, key);
    if (pos >= json.size() || json[pos++] != '"') throw std::runtime_error("metadata string expected");
    std::string value;
    while (pos < json.size()) {
        const char ch = json[pos++];
        if (ch == '"') return value;
        if (static_cast<unsigned char>(ch) < 0x20) throw std::runtime_error("metadata contains control character");
        if (ch != '\\') {
            value.push_back(ch);
            continue;
        }
        if (pos >= json.size()) throw std::runtime_error("unterminated metadata escape");
        const char escaped = json[pos++];
        switch (escaped) {
        case '"': case '\\': case '/': value.push_back(escaped); break;
        case 'b': value.push_back('\b'); break;
        case 'f': value.push_back('\f'); break;
        case 'n': value.push_back('\n'); break;
        case 'r': value.push_back('\r'); break;
        case 't': value.push_back('\t'); break;
        default: throw std::runtime_error("unsupported metadata escape");
        }
    }
    throw std::runtime_error("unterminated metadata string");
}

std::int32_t JsonInt(const std::string& json, const char* key) {
    auto pos = ValueStart(json, key);
    const char* begin = json.data() + pos;
    const char* end = json.data() + json.size();
    std::int32_t value = 0;
    const auto result = std::from_chars(begin, end, value);
    if (result.ec != std::errc{}) throw std::runtime_error("invalid metadata integer");
    auto cursor = result.ptr;
    while (cursor < end && (*cursor == ' ' || *cursor == '\t' || *cursor == '\r' || *cursor == '\n')) ++cursor;
    if (cursor == end || (*cursor != ',' && *cursor != '}')) throw std::runtime_error("malformed metadata integer");
    return value;
}

std::wstring Utf8ToWide(const std::string& value) {
    if (value.empty()) return {};
    const int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), static_cast<int>(value.size()), nullptr, 0);
    if (count <= 0) throw std::runtime_error("metadata is not valid UTF-8");
    std::wstring wide(static_cast<std::size_t>(count), L'\0');
    if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), static_cast<int>(value.size()), wide.data(), count) != count) {
        throw std::runtime_error("metadata UTF-8 conversion failed");
    }
    return wide;
}

bool ValidIdentity(const std::string& value) {
    if (value.size() < 3 || value.size() > 128) return false;
    for (std::size_t i = 0; i < value.size(); ++i) {
        const unsigned char ch = static_cast<unsigned char>(value[i]);
        if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) continue;
        if (i > 0 && (ch == '.' || ch == '-')) continue;
        return false;
    }
    return true;
}

} // namespace

AppMetadata ParseMetadata(const std::string& json) {
    if (json.empty() || json.size() > static_cast<std::size_t>(kMaxMetadataBytes) || json.front() != '{' || json.back() != '}') {
        throw std::runtime_error("malformed application metadata");
    }
    AppMetadata result;
    result.identity = JsonString(json, "identity");
    result.title = Utf8ToWide(JsonString(json, "title"));
    result.width = JsonInt(json, "width");
    result.height = JsonInt(json, "height");
    if (!ValidIdentity(result.identity)) throw std::runtime_error("invalid application identity");
    if (result.title.empty() || result.title.size() > 256) throw std::runtime_error("invalid application title");
    if (result.width < 160 || result.width > 16384 || result.height < 160 || result.height > 16384) {
        throw std::runtime_error("invalid preferred application dimensions");
    }
    return result;
}

void NativeApp::LoadAdjacent() {
    if (module_) throw std::runtime_error("Go application already loaded");

    std::wstring executable(32768, L'\0');
    const DWORD length = GetModuleFileNameW(nullptr, executable.data(), static_cast<DWORD>(executable.size()));
    if (length == 0 || length >= executable.size()) throw std::runtime_error("cannot resolve host executable path");
    executable.resize(length);
    const auto dllPath = std::filesystem::path(executable).parent_path() / L"poem_app.dll";
    if (!dllPath.is_absolute()) throw std::runtime_error("Go application path is not absolute");

    SetDefaultDllDirectories(LOAD_LIBRARY_SEARCH_SYSTEM32 | LOAD_LIBRARY_SEARCH_USER_DIRS);
    module_ = LoadLibraryExW(dllPath.c_str(), nullptr, LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | LOAD_LIBRARY_SEARCH_SYSTEM32);
    if (!module_) throw std::runtime_error("cannot load adjacent poem_app.dll");

    abiVersion_ = RequiredExport<AbiVersionFn>(module_, "PoemWindowsABIVersion");
    metadataFn_ = RequiredExport<MetadataFn>(module_, "PoemWindowsMetadata");
    start_ = RequiredExport<StartFn>(module_, "PoemWindowsStart");
    read_ = RequiredExport<ReadFn>(module_, "PoemHostRead");
    write_ = RequiredExport<WriteFn>(module_, "PoemHostWrite");
    stop_ = RequiredExport<StopFn>(module_, "PoemWindowsStop");
    if (abiVersion_() != kABIVersion) throw std::runtime_error("incompatible Go application ABI");

    const auto required = metadataFn_(nullptr, 0);
    if (required <= 0 || required > kMaxMetadataBytes) throw std::runtime_error("invalid application metadata size");
    std::string encoded(static_cast<std::size_t>(required), '\0');
    if (metadataFn_(encoded.data(), required) != required) throw std::runtime_error("application metadata changed during load");
    metadata_ = ParseMetadata(encoded);
}

void NativeApp::Start(std::int32_t width, std::int32_t height) {
    if (!module_ || !start_) throw std::runtime_error("Go application is not loaded");
    if (started_) throw std::runtime_error("Go application is already started");
    if (width <= 0 || height <= 0 || start_(width, height) != 0) throw std::runtime_error("Go application failed to start");
    started_ = true;
}

void NativeApp::Stop() noexcept {
    if (started_ && stop_) {
        stop_();
        started_ = false;
    }
}

void NativeApp::ReadExact(void* buffer, std::uint32_t bytes) {
    auto* cursor = static_cast<std::uint8_t*>(buffer);
    while (bytes > 0) {
        const auto request = static_cast<std::int32_t>(std::min<std::uint32_t>(bytes, static_cast<std::uint32_t>(std::numeric_limits<std::int32_t>::max())));
        const auto count = read_(cursor, request);
        if (count <= 0 || count > request) throw std::runtime_error("Go application transport read failed");
        cursor += count;
        bytes -= static_cast<std::uint32_t>(count);
    }
}

void NativeApp::WriteExact(const void* buffer, std::uint32_t bytes) {
    auto* cursor = static_cast<const std::uint8_t*>(buffer);
    while (bytes > 0) {
        const auto request = static_cast<std::int32_t>(std::min<std::uint32_t>(bytes, static_cast<std::uint32_t>(std::numeric_limits<std::int32_t>::max())));
        const auto count = write_(cursor, request);
        if (count <= 0 || count > request) throw std::runtime_error("Go application transport write failed");
        cursor += count;
        bytes -= static_cast<std::uint32_t>(count);
    }
}

std::vector<std::uint8_t> NativeApp::ReadMessage() {
    std::uint32_t size = 0;
    ReadExact(&size, sizeof(size));
    if (size > kMaxMessageBytes) throw std::runtime_error("Go application message exceeds safety limit");
    std::vector<std::uint8_t> payload(size);
    if (size > 0) ReadExact(payload.data(), size);
    return payload;
}

void NativeApp::WriteMessage(const std::vector<std::uint8_t>& payload) {
    if (payload.size() > kMaxMessageBytes) throw std::runtime_error("host message exceeds safety limit");
    const auto size = static_cast<std::uint32_t>(payload.size());
    WriteExact(&size, sizeof(size));
    if (size > 0) WriteExact(payload.data(), size);
}

} // namespace poem::host
