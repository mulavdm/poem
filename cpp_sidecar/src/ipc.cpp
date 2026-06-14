#include "ipc.h"

#include <stdexcept>
#include <thread>

namespace poem::ipc {

PipeConnection ConnectToPipe(const wchar_t* pipeName) {
    for (int attempt = 0; attempt < 40; ++attempt) {
        HANDLE handle = ::CreateFileW(pipeName, GENERIC_READ | GENERIC_WRITE, 0, nullptr, OPEN_EXISTING, 0, nullptr);
        if (handle != INVALID_HANDLE_VALUE) {
            return PipeConnection{handle};
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    throw std::runtime_error("failed to connect to named pipe");
}

void Close(PipeConnection& conn) {
    if (conn.handle != INVALID_HANDLE_VALUE) {
        ::CloseHandle(conn.handle);
        conn.handle = INVALID_HANDLE_VALUE;
    }
}

static void ReadExact(HANDLE handle, void* buffer, std::uint32_t bytes) {
    std::uint8_t* ptr = static_cast<std::uint8_t*>(buffer);
    std::uint32_t remaining = bytes;
    while (remaining > 0) {
        DWORD read = 0;
        if (!::ReadFile(handle, ptr, remaining, &read, nullptr) || read == 0) {
            throw std::runtime_error("pipe read failed");
        }
        ptr += read;
        remaining -= read;
    }
}

static void WriteExact(HANDLE handle, const void* buffer, std::uint32_t bytes) {
    const std::uint8_t* ptr = static_cast<const std::uint8_t*>(buffer);
    std::uint32_t remaining = bytes;
    while (remaining > 0) {
        DWORD written = 0;
        if (!::WriteFile(handle, ptr, remaining, &written, nullptr) || written == 0) {
            throw std::runtime_error("pipe write failed");
        }
        ptr += written;
        remaining -= written;
    }
}

std::vector<std::uint8_t> ReadMessage(PipeConnection& conn) {
    std::uint32_t size = 0;
    ReadExact(conn.handle, &size, sizeof(size));
    std::vector<std::uint8_t> payload(size);
    if (size > 0) {
        ReadExact(conn.handle, payload.data(), size);
    }
    return payload;
}

void WriteMessage(PipeConnection& conn, const std::vector<std::uint8_t>& payload) {
    const auto size = static_cast<std::uint32_t>(payload.size());
    thread_local std::vector<std::uint8_t> buffer;
    buffer.resize(sizeof(size) + size);
    std::memcpy(buffer.data(), &size, sizeof(size));
    if (size > 0) {
        std::memcpy(buffer.data() + sizeof(size), payload.data(), size);
    }
    WriteExact(conn.handle, buffer.data(), static_cast<std::uint32_t>(buffer.size()));
}

} // namespace poem::ipc
