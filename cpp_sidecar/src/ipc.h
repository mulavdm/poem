#pragma once

#include <windows.h>

#include <cstdint>
#include <string>
#include <vector>

namespace poem::ipc {

struct PipeConnection {
    HANDLE handle = INVALID_HANDLE_VALUE;
};

PipeConnection ConnectToPipe(const wchar_t* pipeName);
void Close(PipeConnection& conn);
std::vector<std::uint8_t> ReadMessage(PipeConnection& conn);
void WriteMessage(PipeConnection& conn, const std::vector<std::uint8_t>& payload);

} // namespace poem::ipc
