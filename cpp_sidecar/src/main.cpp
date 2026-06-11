#include "audio.h"
#include "ipc.h"
#include "protocol.h"
#include "renderer_d3d11.h"

#include <shellscalingapi.h>
#include <shellapi.h>
#include <shlobj.h>
#include <shobjidl.h>
#include <windows.h>
#include <windowsx.h>

#include <algorithm>
#include <atomic>
#include <cstdio>
#include <memory>
#include <mutex>
#include <string>
#include <thread>

namespace {

constexpr UINT WM_POEM_FRAME = WM_APP + 1;

struct AppState {
    poem::ipc::PipeConnection toRenderer;
    poem::ipc::PipeConnection toGo;
    poem::protocol::InitEngine init;
    poem::RendererD3D11 renderer;
    poem::AudioEngine audio;
    std::mutex frameMutex;
    poem::protocol::RenderFrame latestFrame;
    bool hasFrame = false;
    BYTE cursorType = 0;
    std::atomic<bool> running = true;
    HWND hwnd = nullptr;
};

std::uint32_t ModifierMask() {
    std::uint32_t mask = 0;
    if ((GetKeyState(VK_CONTROL) & 0x8000) != 0) mask |= 1;
    if ((GetKeyState(VK_SHIFT) & 0x8000) != 0) mask |= 2;
    return mask;
}

void SendEvent(AppState* app, const poem::protocol::Event& ev) {
    poem::protocol::EventBatch batch;
    batch.events.push_back(ev);
    auto payload = poem::protocol::EncodeEventBatch(batch);
    poem::ipc::WriteMessage(app->toGo, payload);
}

LRESULT CALLBACK WndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    auto* app = reinterpret_cast<AppState*>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
    switch (msg) {
    case WM_NCCREATE: {
        auto* create = reinterpret_cast<CREATESTRUCTW*>(lParam);
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(create->lpCreateParams));
        return TRUE;
    }
    case WM_SIZE:
        if (app && wParam != SIZE_MINIMIZED) {
            int width = LOWORD(lParam);
            int height = HIWORD(lParam);
            app->renderer.Resize(width, height);
            const auto dpi = GetDpiForWindow(hwnd);
            float scale = static_cast<float>(dpi) / 96.0f;
            SendEvent(app, {poem::protocol::EventType::WindowSize, width, height, 0, 0, 0, 0,
                            static_cast<std::int32_t>(width / scale), static_cast<std::int32_t>(height / scale)});
        }
        return 0;
    case WM_DPICHANGED:
        if (app) {
            auto* suggested = reinterpret_cast<RECT*>(lParam);
            SetWindowPos(hwnd, nullptr, suggested->left, suggested->top, suggested->right - suggested->left,
                         suggested->bottom - suggested->top, SWP_NOZORDER | SWP_NOACTIVATE);
            const auto dpi = HIWORD(wParam);
            RECT rc{};
            GetClientRect(hwnd, &rc);
            float scale = static_cast<float>(dpi) / 96.0f;
            SendEvent(app, {poem::protocol::EventType::WindowSize, rc.right - rc.left, rc.bottom - rc.top, 0, 0, 0, 0,
                            static_cast<std::int32_t>((rc.right - rc.left) / scale),
                            static_cast<std::int32_t>((rc.bottom - rc.top) / scale)});
        }
        return 0;
    case WM_MOUSEMOVE:
        if (app) {
            const auto dpi = GetDpiForWindow(hwnd);
            float scale = static_cast<float>(dpi) / 96.0f;
            SendEvent(app, {poem::protocol::EventType::MouseMove,
                            static_cast<std::int32_t>(GET_X_LPARAM(lParam) / scale),
                            static_cast<std::int32_t>(GET_Y_LPARAM(lParam) / scale), 0, 0, 0, 0, 0, 0});
        }
        return 0;
    case WM_LBUTTONDOWN:
    case WM_RBUTTONDOWN:
    case WM_MBUTTONDOWN:
    case WM_LBUTTONUP:
    case WM_RBUTTONUP:
    case WM_MBUTTONUP:
        if (app) {
            const auto dpi = GetDpiForWindow(hwnd);
            float scale = static_cast<float>(dpi) / 96.0f;
            int button = msg == WM_RBUTTONDOWN || msg == WM_RBUTTONUP ? 2 : (msg == WM_MBUTTONDOWN || msg == WM_MBUTTONUP ? 3 : 1);
            bool down = (msg == WM_LBUTTONDOWN || msg == WM_RBUTTONDOWN || msg == WM_MBUTTONDOWN);
            SendEvent(app, {down ? poem::protocol::EventType::MouseDown : poem::protocol::EventType::MouseUp,
                            static_cast<std::int32_t>(GET_X_LPARAM(lParam) / scale),
                            static_cast<std::int32_t>(GET_Y_LPARAM(lParam) / scale),
                            button, 0, 0, 0, 0, 0});
        }
        return 0;
    case WM_MOUSEWHEEL:
        if (app) {
            const auto dpi = GetDpiForWindow(hwnd);
            float scale = static_cast<float>(dpi) / 96.0f;
            POINT pt{GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            ScreenToClient(hwnd, &pt);
            SendEvent(app, {poem::protocol::EventType::MouseWheel,
                            static_cast<std::int32_t>(pt.x / scale),
                            static_cast<std::int32_t>(pt.y / scale),
                            0, GET_WHEEL_DELTA_WPARAM(wParam), 0, 0, 0, 0});
        }
        return 0;
    case WM_KEYDOWN:
    case WM_SYSKEYDOWN:
        if (app) {
            SendEvent(app, {poem::protocol::EventType::KeyDown, 0, 0, static_cast<std::int32_t>(ModifierMask()), 0,
                            static_cast<std::uint32_t>(wParam), 0, 0, 0});
        }
        return 0;
    case WM_KEYUP:
    case WM_SYSKEYUP:
        if (app) {
            SendEvent(app, {poem::protocol::EventType::KeyUp, 0, 0, static_cast<std::int32_t>(ModifierMask()), 0,
                            static_cast<std::uint32_t>(wParam), 0, 0, 0});
        }
        return 0;
    case WM_CHAR:
        if (app) {
            SendEvent(app, {poem::protocol::EventType::KeyChar, 0, 0, 0, 0, 0, static_cast<std::uint32_t>(wParam), 0, 0});
        }
        return 0;
    case WM_SETCURSOR:
        if (app) {
            LPCWSTR cursor = IDC_ARROW;
            if (app->cursorType == 1) cursor = IDC_HAND;
            else if (app->cursorType == 2) cursor = IDC_IBEAM;
            SetCursor(LoadCursorW(nullptr, cursor));
            return TRUE;
        }
        break;
    case WM_PAINT:
    case WM_POEM_FRAME:
        if (app) {
            std::lock_guard<std::mutex> lock(app->frameMutex);
            if (app->hasFrame) {
                app->renderer.Render(app->latestFrame);
            }
            ValidateRect(hwnd, nullptr);
            return 0;
        }
        break;
    case WM_CLOSE:
        if (app) {
            SendEvent(app, {poem::protocol::EventType::WindowClose, 0, 0, 0, 0, 0, 0, 0, 0});
            app->running = false;
        }
        DestroyWindow(hwnd);
        return 0;
    case WM_DESTROY:
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

std::wstring Utf8ToWide(const std::string& input) {
    if (input.empty()) return L"POEM";
    int count = MultiByteToWideChar(CP_UTF8, 0, input.data(), static_cast<int>(input.size()), nullptr, 0);
    std::wstring out(count, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, input.data(), static_cast<int>(input.size()), out.data(), count);
    return out;
}

std::string WideToUtf8(const std::wstring& input) {
    if (input.empty()) return "";
    int count = WideCharToMultiByte(CP_UTF8, 0, input.data(), static_cast<int>(input.size()), nullptr, 0, nullptr, nullptr);
    std::string out(count, '\0');
    WideCharToMultiByte(CP_UTF8, 0, input.data(), static_cast<int>(input.size()), out.data(), count, nullptr, nullptr);
    return out;
}

poem::protocol::NativeDialogResponse OpenNativeDialog(HWND hwnd, const poem::protocol::NativeDialogRequest& request) {
    poem::protocol::NativeDialogResponse response;
    if (request.kind != "directory") {
        response.error = "unsupported native dialog kind";
        return response;
    }

    HRESULT coHr = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
    bool uninitialize = SUCCEEDED(coHr);
    if (FAILED(coHr) && coHr != RPC_E_CHANGED_MODE) {
        response.error = "failed to initialize COM for native dialog";
        return response;
    }

    IFileOpenDialog* dialog = nullptr;
    HRESULT hr = CoCreateInstance(CLSID_FileOpenDialog, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&dialog));
    if (FAILED(hr) || dialog == nullptr) {
        if (uninitialize) CoUninitialize();
        response.error = "failed to create directory picker";
        return response;
    }

    DWORD options = 0;
    if (SUCCEEDED(dialog->GetOptions(&options))) {
        dialog->SetOptions(options | FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM | FOS_PATHMUSTEXIST);
    }
    if (!request.title.empty()) {
        std::wstring title = Utf8ToWide(request.title);
        dialog->SetTitle(title.c_str());
    }
    if (!request.initialDir.empty()) {
        std::wstring initialDir = Utf8ToWide(request.initialDir);
        IShellItem* folder = nullptr;
        if (SUCCEEDED(SHCreateItemFromParsingName(initialDir.c_str(), nullptr, IID_PPV_ARGS(&folder))) && folder != nullptr) {
            dialog->SetFolder(folder);
            folder->Release();
        }
    }

    hr = dialog->Show(hwnd);
    if (hr == HRESULT_FROM_WIN32(ERROR_CANCELLED)) {
        response.canceled = true;
        dialog->Release();
        if (uninitialize) CoUninitialize();
        return response;
    }
    if (FAILED(hr)) {
        response.error = "directory picker failed";
        dialog->Release();
        if (uninitialize) CoUninitialize();
        return response;
    }

    IShellItem* item = nullptr;
    hr = dialog->GetResult(&item);
    if (FAILED(hr) || item == nullptr) {
        response.error = "directory picker did not return a folder";
        dialog->Release();
        if (uninitialize) CoUninitialize();
        return response;
    }

    PWSTR path = nullptr;
    hr = item->GetDisplayName(SIGDN_FILESYSPATH, &path);
    if (FAILED(hr) || path == nullptr) {
        response.error = "failed to resolve selected folder path";
    } else {
        response.path = WideToUtf8(path);
        CoTaskMemFree(path);
    }
    item->Release();
    dialog->Release();
    if (uninitialize) CoUninitialize();
    return response;
}

struct StartupMonitorInfo {
    HMONITOR monitor = nullptr;
    MONITORINFO info{};
    UINT dpiX = 96;
    UINT dpiY = 96;
};

StartupMonitorInfo ResolveStartupMonitorInfo() {
    POINT anchor{};
    if (!GetCursorPos(&anchor)) {
        anchor = POINT{0, 0};
    }

    StartupMonitorInfo out;
    out.monitor = MonitorFromPoint(anchor, MONITOR_DEFAULTTONEAREST);
    out.info.cbSize = sizeof(out.info);
    if (!GetMonitorInfoW(out.monitor, &out.info)) {
        out.info = MONITORINFO{};
        out.info.cbSize = sizeof(out.info);
        return out;
    }

    UINT dpiX = 96;
    UINT dpiY = 96;
    if (GetDpiForMonitor(out.monitor, MDT_EFFECTIVE_DPI, &dpiX, &dpiY) == S_OK) {
        out.dpiX = dpiX;
        out.dpiY = dpiY;
    }
    return out;
}

RECT ResolveStartupWindowRect(const RECT& desiredWindowRect, const RECT& workArea) {
    constexpr double kStartupWorkAreaUsage = 0.92;

    if (workArea.right <= workArea.left || workArea.bottom <= workArea.top) {
        return desiredWindowRect;
    }

    const int desiredWidth = desiredWindowRect.right - desiredWindowRect.left;
    const int desiredHeight = desiredWindowRect.bottom - desiredWindowRect.top;
    const int workWidth = workArea.right - workArea.left;
    const int workHeight = workArea.bottom - workArea.top;

    const int maxWidth = std::max(640, static_cast<int>(workWidth * kStartupWorkAreaUsage));
    const int maxHeight = std::max(480, static_cast<int>(workHeight * kStartupWorkAreaUsage));

    double scale = 1.0;
    if (desiredWidth > maxWidth || desiredHeight > maxHeight) {
        const double scaleX = desiredWidth > 0 ? static_cast<double>(maxWidth) / static_cast<double>(desiredWidth) : 1.0;
        const double scaleY = desiredHeight > 0 ? static_cast<double>(maxHeight) / static_cast<double>(desiredHeight) : 1.0;
        scale = std::min(scaleX, scaleY);
    }

    const int clampedWidth = std::max(640, std::min(workWidth, static_cast<int>(desiredWidth * scale)));
    const int clampedHeight = std::max(480, std::min(workHeight, static_cast<int>(desiredHeight * scale)));
    const int left = workArea.left + std::max(0, (workWidth - clampedWidth) / 2);
    const int top = workArea.top + std::max(0, (workHeight - clampedHeight) / 2);

    return RECT{left, top, left + clampedWidth, top + clampedHeight};
}

void ClampWindowToCurrentMonitorWorkArea(HWND hwnd);

bool CapturePresentedWindowRGBA(HWND hwnd, std::vector<std::uint8_t>& rgba, int& width, int& height) {
    width = 0;
    height = 0;
    rgba.clear();
    if (!hwnd) {
        return false;
    }

    RECT clientRect{};
    if (!GetClientRect(hwnd, &clientRect)) {
        return false;
    }
    width = clientRect.right - clientRect.left;
    height = clientRect.bottom - clientRect.top;
    if (width <= 0 || height <= 0) {
        return false;
    }

    HDC windowDC = GetDC(hwnd);
    if (!windowDC) {
        return false;
    }

    HDC memoryDC = CreateCompatibleDC(windowDC);
    if (!memoryDC) {
        ReleaseDC(hwnd, windowDC);
        return false;
    }

    BITMAPINFO bmi{};
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = width;
    bmi.bmiHeader.biHeight = -height;
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;

    void* dibBits = nullptr;
    HBITMAP dib = CreateDIBSection(windowDC, &bmi, DIB_RGB_COLORS, &dibBits, nullptr, 0);
    if (!dib || !dibBits) {
        DeleteDC(memoryDC);
        ReleaseDC(hwnd, windowDC);
        return false;
    }

    HGDIOBJ oldBitmap = SelectObject(memoryDC, dib);
    bool captured = BitBlt(memoryDC, 0, 0, width, height, windowDC, 0, 0, SRCCOPY | CAPTUREBLT) == TRUE;

    if (captured) {
        const auto* bgra = static_cast<const std::uint8_t*>(dibBits);
        rgba.resize(static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4);
        for (std::size_t src = 0, dst = 0; dst < rgba.size(); src += 4, dst += 4) {
            rgba[dst + 0] = bgra[src + 2];
            rgba[dst + 1] = bgra[src + 1];
            rgba[dst + 2] = bgra[src + 0];
            rgba[dst + 3] = bgra[src + 3];
        }
    }

    SelectObject(memoryDC, oldBitmap);
    DeleteObject(dib);
    DeleteDC(memoryDC);
    ReleaseDC(hwnd, windowDC);
    return captured;
}

bool CaptureDesktopRegionRGBA(HWND hwnd, std::vector<std::uint8_t>& rgba, int& width, int& height) {
    width = 0;
    height = 0;
    rgba.clear();
    if (!hwnd) {
        return false;
    }

    RECT clientRect{};
    if (!GetClientRect(hwnd, &clientRect)) {
        return false;
    }
    width = clientRect.right - clientRect.left;
    height = clientRect.bottom - clientRect.top;
    if (width <= 0 || height <= 0) {
        return false;
    }

    POINT origin{0, 0};
    if (!ClientToScreen(hwnd, &origin)) {
        return false;
    }

    HDC screenDC = GetDC(nullptr);
    if (!screenDC) {
        return false;
    }

    HDC memoryDC = CreateCompatibleDC(screenDC);
    if (!memoryDC) {
        ReleaseDC(nullptr, screenDC);
        return false;
    }

    BITMAPINFO bmi{};
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = width;
    bmi.bmiHeader.biHeight = -height;
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;

    void* dibBits = nullptr;
    HBITMAP dib = CreateDIBSection(screenDC, &bmi, DIB_RGB_COLORS, &dibBits, nullptr, 0);
    if (!dib || !dibBits) {
        DeleteDC(memoryDC);
        ReleaseDC(nullptr, screenDC);
        return false;
    }

    HGDIOBJ oldBitmap = SelectObject(memoryDC, dib);
    bool captured = BitBlt(memoryDC, 0, 0, width, height, screenDC, origin.x, origin.y, SRCCOPY | CAPTUREBLT) == TRUE;

    if (captured) {
        const auto* bgra = static_cast<const std::uint8_t*>(dibBits);
        rgba.resize(static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4);
        for (std::size_t src = 0, dst = 0; dst < rgba.size(); src += 4, dst += 4) {
            rgba[dst + 0] = bgra[src + 2];
            rgba[dst + 1] = bgra[src + 1];
            rgba[dst + 2] = bgra[src + 0];
            rgba[dst + 3] = bgra[src + 3];
        }
    }

    SelectObject(memoryDC, oldBitmap);
    DeleteObject(dib);
    DeleteDC(memoryDC);
    ReleaseDC(nullptr, screenDC);
    return captured;
}

void RequestForegroundActivation(HWND hwnd) {
    if (!hwnd) {
        return;
    }

    ShowWindow(hwnd, SW_SHOW);
    BringWindowToTop(hwnd);

    const HWND currentForeground = GetForegroundWindow();
    const DWORD currentThread = GetCurrentThreadId();
    DWORD foregroundThread = 0;
    if (currentForeground) {
        foregroundThread = GetWindowThreadProcessId(currentForeground, nullptr);
    }

    bool attached = false;
    if (foregroundThread != 0 && foregroundThread != currentThread) {
        attached = AttachThreadInput(currentThread, foregroundThread, TRUE) == TRUE;
    }

    SetForegroundWindow(hwnd);
    SetActiveWindow(hwnd);
    SetFocus(hwnd);

    SetWindowPos(hwnd, HWND_TOPMOST, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_SHOWWINDOW);
    SetWindowPos(hwnd, HWND_NOTOPMOST, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_SHOWWINDOW);

    BringWindowToTop(hwnd);
    SetForegroundWindow(hwnd);

    if (attached) {
        AttachThreadInput(currentThread, foregroundThread, FALSE);
    }
}

void ApplyNativeWindowControl(HWND hwnd, const poem::protocol::NativeDebugRequest& request) {
    if (!hwnd) {
        return;
    }

    if (request.restoreWindow) {
        if (IsIconic(hwnd)) {
            ShowWindow(hwnd, SW_RESTORE);
        } else {
            ShowWindow(hwnd, SW_SHOW);
        }
    }
    if (request.clampToWorkArea) {
        ClampWindowToCurrentMonitorWorkArea(hwnd);
    }
    if (request.bringToForeground) {
        RequestForegroundActivation(hwnd);
    }
    if (request.maximizeWindow) {
        ShowWindow(hwnd, SW_MAXIMIZE);
        if (request.bringToForeground) {
            RequestForegroundActivation(hwnd);
        }
    }
}

void ClampWindowToCurrentMonitorWorkArea(HWND hwnd) {
    if (!hwnd) {
        return;
    }

    HMONITOR monitor = MonitorFromWindow(hwnd, MONITOR_DEFAULTTONEAREST);
    MONITORINFO monitorInfo{};
    monitorInfo.cbSize = sizeof(monitorInfo);
    if (!GetMonitorInfoW(monitor, &monitorInfo)) {
        return;
    }

    RECT workArea = monitorInfo.rcWork;
    RECT windowRect{};
    if (!GetWindowRect(hwnd, &windowRect)) {
        return;
    }

    const int windowWidth = windowRect.right - windowRect.left;
    const int windowHeight = windowRect.bottom - windowRect.top;
    const int maxWidth = workArea.right - workArea.left;
    const int maxHeight = workArea.bottom - workArea.top;

    const int clampedWidth = std::min(windowWidth, maxWidth);
    const int clampedHeight = std::min(windowHeight, maxHeight);

    int left = windowRect.left;
    int top = windowRect.top;

    if (left < workArea.left) {
        left = workArea.left;
    }
    if (top < workArea.top) {
        top = workArea.top;
    }
    if (left + clampedWidth > workArea.right) {
        left = workArea.right - clampedWidth;
    }
    if (top + clampedHeight > workArea.bottom) {
        top = workArea.bottom - clampedHeight;
    }

    SetWindowPos(hwnd, nullptr, left, top, clampedWidth, clampedHeight, SWP_NOZORDER | SWP_NOACTIVATE);
}

poem::protocol::NativeDebugResponse BuildNativeDebugResponse(AppState& app, const poem::protocol::NativeDebugRequest& request) {
    poem::protocol::NativeDebugResponse response;
    if (!app.hwnd) {
        response.error = "window handle not initialized";
        return response;
    }

    ApplyNativeWindowControl(app.hwnd, request);

    RECT windowRect{};
    RECT clientRect{};
    if (!GetWindowRect(app.hwnd, &windowRect) || !GetClientRect(app.hwnd, &clientRect)) {
        response.error = "failed to query window rectangles";
        return response;
    }

    HMONITOR monitor = MonitorFromWindow(app.hwnd, MONITOR_DEFAULTTONEAREST);
    MONITORINFO monitorInfo{};
    monitorInfo.cbSize = sizeof(monitorInfo);
    if (!GetMonitorInfoW(monitor, &monitorInfo)) {
        response.error = "failed to query monitor info";
        return response;
    }

    response.dpi = static_cast<std::int32_t>(GetDpiForWindow(app.hwnd));
    response.windowVisible = IsWindowVisible(app.hwnd) == TRUE;
    response.windowMinimized = IsIconic(app.hwnd) == TRUE;
    response.windowForeground = GetForegroundWindow() == app.hwnd;
    response.windowLeft = windowRect.left;
    response.windowTop = windowRect.top;
    response.windowRight = windowRect.right;
    response.windowBottom = windowRect.bottom;
    response.clientWidth = clientRect.right - clientRect.left;
    response.clientHeight = clientRect.bottom - clientRect.top;
    response.workLeft = monitorInfo.rcWork.left;
    response.workTop = monitorInfo.rcWork.top;
    response.workRight = monitorInfo.rcWork.right;
    response.workBottom = monitorInfo.rcWork.bottom;
    response.backbufferWidth = app.renderer.BackbufferWidth();
    response.backbufferHeight = app.renderer.BackbufferHeight();

    if (request.captureFrame || request.capturePresentedFrame || request.captureDesktopFrame) {
        int frameWidth = 0;
        int frameHeight = 0;
        bool ok = false;
        if (request.captureDesktopFrame) {
            ok = CaptureDesktopRegionRGBA(app.hwnd, response.frameRgba, frameWidth, frameHeight);
            if (!ok) {
                response.error = "failed to capture desktop region";
            }
        } else if (request.capturePresentedFrame) {
            ok = CapturePresentedWindowRGBA(app.hwnd, response.frameRgba, frameWidth, frameHeight);
            if (!ok) {
                response.error = "failed to capture presented window";
            }
        } else {
            std::lock_guard<std::mutex> lock(app.frameMutex);
            ok = app.renderer.CaptureBackbufferRGBA(response.frameRgba, frameWidth, frameHeight);
            if (!ok) {
                response.error = "failed to capture backbuffer";
            }
        }
        if (!ok) {
            response.frameRgba.clear();
            return response;
        }
        response.frameWidth = frameWidth;
        response.frameHeight = frameHeight;
    }

    return response;
}

} // namespace

int WINAPI wWinMain(HINSTANCE hInst, HINSTANCE, PWSTR, int) {
    SetProcessDpiAwareness(PROCESS_PER_MONITOR_DPI_AWARE);

    std::printf("POEM C++ sidecar starting up...\n");
    AppState app;
    try {
        app.toRenderer = poem::ipc::ConnectToPipe(L"\\\\.\\pipe\\poem_ipc_go_to_sidecar");
        app.toGo = poem::ipc::ConnectToPipe(L"\\\\.\\pipe\\poem_ipc_sidecar_to_go");
        auto initPayload = poem::ipc::ReadMessage(app.toRenderer);
        auto initEnvelope = poem::protocol::DecodeEnvelope(initPayload);
        if (initEnvelope.type != poem::protocol::MessageType::InitEngine) {
            throw std::runtime_error("expected InitEngine");
        }
        app.init = poem::protocol::DecodeInitEngine(initEnvelope.body);
    } catch (const std::exception& ex) {
        std::fprintf(stderr, "Startup failure: %s\n", ex.what());
        return 1;
    }

    std::wstring title = L"POEM C++ Sidecar";
    int argc = 0;
    LPWSTR* argv = CommandLineToArgvW(GetCommandLineW(), &argc);
    if (argc > 1) title = argv[1];
    LocalFree(argv);

    WNDCLASSW wc{};
    wc.lpfnWndProc = WndProc;
    wc.hInstance = hInst;
    wc.lpszClassName = L"POEMCppSidecarWindow";
    wc.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    RegisterClassW(&wc);

    const StartupMonitorInfo startupMonitor = ResolveStartupMonitorInfo();
    const UINT initialDpi = startupMonitor.dpiX;
    int desiredClientWidth = MulDiv(app.init.width, static_cast<int>(initialDpi), 96);
    int desiredClientHeight = MulDiv(app.init.height, static_cast<int>(initialDpi), 96);
    RECT desiredWindowRect{0, 0, desiredClientWidth, desiredClientHeight};
    AdjustWindowRectExForDpi(&desiredWindowRect, WS_OVERLAPPEDWINDOW, FALSE, 0, initialDpi);
    RECT startupWindowRect = ResolveStartupWindowRect(desiredWindowRect, startupMonitor.info.rcWork);

    HWND hwnd = CreateWindowExW(0, wc.lpszClassName, title.c_str(), WS_OVERLAPPEDWINDOW,
                                startupWindowRect.left, startupWindowRect.top,
                                startupWindowRect.right - startupWindowRect.left,
                                startupWindowRect.bottom - startupWindowRect.top,
                                nullptr, nullptr, hInst, &app);
    if (!hwnd) {
        return 2;
    }
    app.hwnd = hwnd;
    ClampWindowToCurrentMonitorWorkArea(hwnd);
    ShowWindow(hwnd, SW_SHOW);
    UpdateWindow(hwnd);

    RECT rc{};
    GetClientRect(hwnd, &rc);
    if (!app.renderer.Initialize(hwnd, rc.right - rc.left, rc.bottom - rc.top, app.init)) {
        return 3;
    }

    std::thread reader([&app]() {
        while (app.running) {
            try {
                auto payload = poem::ipc::ReadMessage(app.toRenderer);
                auto env = poem::protocol::DecodeEnvelope(payload);
                if (env.type == poem::protocol::MessageType::RenderFrame) {
                    auto frame = poem::protocol::DecodeRenderFrame(env.body);
                    {
                        std::lock_guard<std::mutex> lock(app.frameMutex);
                        app.cursorType = frame.cursor;
                        app.latestFrame = std::move(frame);
                        app.hasFrame = true;
                    }
                    if (app.hwnd) PostMessageW(app.hwnd, WM_POEM_FRAME, 0, 0);
                } else if (env.type == poem::protocol::MessageType::PlaySound) {
                    auto sound = poem::protocol::DecodePlaySound(env.body);
                    app.audio.Play(sound.type);
                } else if (env.type == poem::protocol::MessageType::NativeDebugRequest) {
                    auto request = poem::protocol::DecodeNativeDebugRequest(env.body);
                    auto response = BuildNativeDebugResponse(app, request);
                    auto payload = poem::protocol::EncodeNativeDebugResponse(response);
                    poem::ipc::WriteMessage(app.toGo, payload);
                } else if (env.type == poem::protocol::MessageType::NativeDialogRequest) {
                    auto request = poem::protocol::DecodeNativeDialogRequest(env.body);
                    auto response = OpenNativeDialog(app.hwnd, request);
                    auto payload = poem::protocol::EncodeNativeDialogResponse(response);
                    poem::ipc::WriteMessage(app.toGo, payload);
                }
            } catch (...) {
                app.running = false;
                PostMessageW(app.hwnd, WM_CLOSE, 0, 0);
                break;
            }
        }
    });

    MSG msg{};
    while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }

    app.running = false;
    if (reader.joinable()) reader.join();
    poem::ipc::Close(app.toRenderer);
    poem::ipc::Close(app.toGo);
    return 0;
}
