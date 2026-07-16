// POEM Android presenter host: a NativeActivity that embeds the Go engine
// (built as a c-shared library) and presents its frames via GLES2.
//
// Process shape: android_main owns the EGL surface and render loop; a
// dedicated transport thread drains the engine→presenter byte stream through
// the Go export PoemHostRead (framed exactly like the Windows named pipes:
// 4-byte little-endian length + POEM v2 envelope); input events accumulate
// and flush to the engine as one framed EventBatch per frame through
// PoemHostWrite — one batched cgo crossing each way per frame.
#include <android/log.h>
#include <android_native_app_glue.h>
#include <EGL/egl.h>
#include <GLES2/gl2.h>

#include <atomic>
#include <cstring>
#include <mutex>
#include <optional>
#include <thread>
#include <vector>

#include "ime_jni.h"
#include "protocol.h"
#include "renderer_gles.h"

#define HLOGI(...) __android_log_print(ANDROID_LOG_INFO, "poem-host", __VA_ARGS__)
#define HLOGE(...) __android_log_print(ANDROID_LOG_ERROR, "poem-host", __VA_ARGS__)

extern "C" {
// Exported by the Go engine library (pkg/mobile).
int PoemHostRead(void* buf, int capacity);
int PoemHostWrite(void* buf, int length);
// Exported by the application's Go main package: registers the app and calls
// mobile.Start with the initial surface size in physical pixels.
void PoemAndroidStart(int width, int height);
}

namespace {

struct Host {
    android_app* app = nullptr;
    EGLDisplay display = EGL_NO_DISPLAY;
    EGLSurface surface = EGL_NO_SURFACE;
    EGLContext context = EGL_NO_CONTEXT;
    int width = 0, height = 0;   // physical surface pixels
    float scale = 1.0f;          // display density (physical px per logical px)
    int logicalW = 0, logicalH = 0;
    bool animating = false;
    bool engineStarted = false;

    poem::RendererGLES renderer;

    std::mutex frameMutex;
    std::optional<poem::protocol::RenderFrame> latestFrame;
    std::optional<poem::protocol::InitEngine> pendingAtlas;
    std::optional<poem::protocol::InitEngine> lastAtlas; // re-upload after surface recreate

    std::mutex eventMutex;
    std::vector<poem::protocol::Event> pendingEvents;

    std::mutex writeMutex;
    std::thread transportThread;
    std::atomic<bool> transportRunning{false};
};

Host g_host;

void QueueEvent(const poem::protocol::Event& event) {
    std::lock_guard<std::mutex> lock(g_host.eventMutex);
    g_host.pendingEvents.push_back(event);
}

// WriteFramed sends one length-framed message to the engine.
bool WriteFramed(const std::vector<std::uint8_t>& payload) {
    std::lock_guard<std::mutex> lock(g_host.writeMutex);
    std::uint8_t header[4];
    const auto length = static_cast<std::uint32_t>(payload.size());
    std::memcpy(header, &length, 4); // little-endian on every Android ABI
    if (PoemHostWrite(header, 4) != 4) return false;
    std::size_t sent = 0;
    while (sent < payload.size()) {
        const int n = PoemHostWrite(const_cast<std::uint8_t*>(payload.data()) + sent,
                                    static_cast<int>(payload.size() - sent));
        if (n <= 0) return false;
        sent += static_cast<std::size_t>(n);
    }
    return true;
}

void FlushEvents() {
    poem::protocol::EventBatch batch;
    {
        std::lock_guard<std::mutex> lock(g_host.eventMutex);
        if (g_host.pendingEvents.empty()) return;
        batch.events.swap(g_host.pendingEvents);
    }
    if (!WriteFramed(poem::protocol::EncodeEventBatch(batch))) {
        HLOGE("event batch write failed (engine gone?)");
    }
}

// ReadExact fills buf from the engine stream, blocking; false on EOF.
bool ReadExact(std::uint8_t* buf, std::size_t need) {
    std::size_t got = 0;
    while (got < need) {
        const int n = PoemHostRead(buf + got, static_cast<int>(need - got));
        if (n < 0) return false;
        got += static_cast<std::size_t>(n);
    }
    return true;
}

void TransportLoop() {
    HLOGI("transport thread up");
    while (g_host.transportRunning.load()) {
        std::uint8_t header[4];
        if (!ReadExact(header, 4)) break;
        std::uint32_t length = 0;
        std::memcpy(&length, header, 4);
        if (length > (256u << 20)) {
            HLOGE("frame length %u exceeds limit", length);
            break;
        }
        std::vector<std::uint8_t> payload(length);
        if (!ReadExact(payload.data(), length)) break;

        try {
            const auto envelope = poem::protocol::DecodeEnvelope(payload);
            switch (envelope.type) {
            case poem::protocol::MessageType::InitEngine:
            case poem::protocol::MessageType::FontAtlas: {
                auto init = poem::protocol::DecodeInitEngine(envelope.body);
                std::lock_guard<std::mutex> lock(g_host.frameMutex);
                g_host.pendingAtlas = init;
                g_host.lastAtlas = std::move(init);
                break;
            }
            case poem::protocol::MessageType::RenderFrame: {
                auto frame = poem::protocol::DecodeRenderFrame(envelope.body);
                static int frames = 0;
                if (frames++ % 300 == 0) {
                    HLOGI("frame #%d: %zu commands %dx%d", frames, frame.commands.size(), frame.width, frame.height);
                }
                std::lock_guard<std::mutex> lock(g_host.frameMutex);
                g_host.latestFrame = std::move(frame);
                break;
            }
            case poem::protocol::MessageType::SetImeVisible: {
                const auto ime = poem::protocol::DecodeSetImeVisible(envelope.body);
                HLOGI("ime visible -> %d", ime.visible ? 1 : 0);
                poem::SetSoftKeyboardVisible(g_host.app->activity, ime.visible);
                break;
            }
            case poem::protocol::MessageType::PlaySound:
            case poem::protocol::MessageType::SemanticTree:
                break; // no audio / accessibility bridge on this presenter yet
            default:
                break;
            }
        } catch (const std::exception& error) {
            HLOGE("protocol decode failed: %s", error.what());
        }
    }
    HLOGI("transport thread down");
}

void InitDisplay(Host* host) {
    host->display = eglGetDisplay(EGL_DEFAULT_DISPLAY);
    eglInitialize(host->display, nullptr, nullptr);
    const EGLint attribs[] = {EGL_RENDERABLE_TYPE, EGL_OPENGL_ES2_BIT,
                              EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
                              EGL_RED_SIZE, 8, EGL_GREEN_SIZE, 8, EGL_BLUE_SIZE, 8,
                              EGL_ALPHA_SIZE, 8, EGL_NONE};
    EGLConfig config;
    EGLint numConfigs = 0;
    eglChooseConfig(host->display, attribs, &config, 1, &numConfigs);
    host->surface = eglCreateWindowSurface(host->display, config, host->app->window, nullptr);
    const EGLint ctxAttribs[] = {EGL_CONTEXT_CLIENT_VERSION, 2, EGL_NONE};
    host->context = eglCreateContext(host->display, config, EGL_NO_CONTEXT, ctxAttribs);
    eglMakeCurrent(host->display, host->surface, host->surface, host->context);
    eglQuerySurface(host->display, host->surface, EGL_WIDTH, &host->width);
    eglQuerySurface(host->display, host->surface, EGL_HEIGHT, &host->height);

    // The engine lays out in logical (density-independent) pixels; the
    // renderer scales geometry up to physical pixels, the same division of
    // labor as the Windows presenters' DPI scaleX/scaleY.
    const int densityDpi = AConfiguration_getDensity(host->app->config);
    host->scale = densityDpi > 0 ? static_cast<float>(densityDpi) / 160.0f : 1.0f;
    host->logicalW = static_cast<int>(host->width / host->scale);
    host->logicalH = static_cast<int>(host->height / host->scale);

    if (!host->renderer.Init(host->width, host->height, host->scale)) {
        HLOGE("renderer init failed");
        return;
    }
    HLOGI("EGL surface %dx%d, density %d => logical %dx%d", host->width, host->height,
          densityDpi, host->logicalW, host->logicalH);

    if (!host->engineStarted) {
        host->engineStarted = true;
        PoemAndroidStart(host->logicalW, host->logicalH);
        host->transportRunning.store(true);
        host->transportThread = std::thread(TransportLoop);
        HLOGI("Go engine started");
    } else {
        // Surface was recreated while the engine kept running: restore the atlas.
        std::lock_guard<std::mutex> lock(g_host.frameMutex);
        if (g_host.lastAtlas) g_host.pendingAtlas = g_host.lastAtlas;
    }
    // The engine paints on demand and arms itself from WindowSize events —
    // the Windows sidecar sends one right after startup, so this host does
    // too. Without it NeedsRepaint stays false and no frame is ever produced.
    poem::protocol::Event resize;
    resize.type = poem::protocol::EventType::WindowSize;
    resize.width = host->logicalW;
    resize.height = host->logicalH;
    QueueEvent(resize);
    host->animating = true;
}

void TermDisplay(Host* host) {
    host->animating = false;
    if (host->display != EGL_NO_DISPLAY) {
        eglMakeCurrent(host->display, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
        if (host->context != EGL_NO_CONTEXT) eglDestroyContext(host->display, host->context);
        if (host->surface != EGL_NO_SURFACE) eglDestroySurface(host->display, host->surface);
        eglTerminate(host->display);
    }
    host->display = EGL_NO_DISPLAY;
    host->surface = EGL_NO_SURFACE;
    host->context = EGL_NO_CONTEXT;
}

void DrawFrame(Host* host) {
    if (host->display == EGL_NO_DISPLAY) return;
    std::optional<poem::protocol::InitEngine> atlas;
    std::optional<poem::protocol::RenderFrame> frame;
    {
        std::lock_guard<std::mutex> lock(host->frameMutex);
        atlas.swap(host->pendingAtlas);
        frame = host->latestFrame;
    }
    if (atlas) host->renderer.UploadAtlas(*atlas);
    if (frame) host->renderer.Render(*frame);
    eglSwapBuffers(host->display, host->surface);
}

void HandleCmd(android_app* app, int32_t cmd) {
    Host* host = static_cast<Host*>(app->userData);
    switch (cmd) {
    case APP_CMD_INIT_WINDOW:
        if (app->window) InitDisplay(host);
        break;
    case APP_CMD_TERM_WINDOW:
        TermDisplay(host);
        break;
    case APP_CMD_GAINED_FOCUS:
        host->animating = host->display != EGL_NO_DISPLAY;
        break;
    case APP_CMD_LOST_FOCUS:
        host->animating = false;
        break;
    }
}

// VkForAndroidKey maps the editing/navigation keys the engine's text
// components act on (Win32 virtual-key vocabulary, matching what the Windows
// sidecar delivers) from Android keycodes. Returns 0 for keys with no mapping.
std::uint32_t VkForAndroidKey(std::int32_t keyCode) {
    switch (keyCode) {
    case AKEYCODE_DEL: return 0x08;          // backspace -> VK_BACK
    case AKEYCODE_TAB: return 0x09;          // VK_TAB (focus cycling)
    case AKEYCODE_ENTER:
    case AKEYCODE_NUMPAD_ENTER: return 0x0D; // VK_RETURN
    // The system back gesture maps to Escape: POEM dismisses overlays
    // (modals, popups) on Escape, which is what back means inside an app
    // whose whole surface is one activity. Exiting is home/recents.
    case AKEYCODE_BACK: return 0x1B;         // back -> VK_ESCAPE
    case AKEYCODE_ESCAPE: return 0x1B;       // VK_ESCAPE
    case AKEYCODE_SPACE: return 0x20;        // VK_SPACE (button activation)
    case AKEYCODE_MOVE_END: return 0x23;     // VK_END
    case AKEYCODE_MOVE_HOME: return 0x24;    // VK_HOME
    case AKEYCODE_DPAD_LEFT: return 0x25;    // VK_LEFT
    case AKEYCODE_DPAD_UP: return 0x26;      // VK_UP
    case AKEYCODE_DPAD_RIGHT: return 0x27;   // VK_RIGHT
    case AKEYCODE_DPAD_DOWN: return 0x28;    // VK_DOWN
    case AKEYCODE_FORWARD_DEL: return 0x2E;  // VK_DELETE
    default: return 0;
    }
}

int32_t HandleKey(android_app* app, AInputEvent* event) {
    const auto action = AKeyEvent_getAction(event);
    const auto keyCode = AKeyEvent_getKeyCode(event);
    const auto vk = VkForAndroidKey(keyCode);
    if (action == AKEY_EVENT_ACTION_DOWN) {
        if (vk != 0) {
            poem::protocol::Event down;
            down.type = poem::protocol::EventType::KeyDown;
            down.keycode = vk;
            QueueEvent(down);
        }
        // Printable characters travel as KeyChar, mirroring WM_CHAR. The
        // engine inserts text from KeyChar and edits from virtual keys, and
        // it already dedupes Enter arriving as both.
        const int codepoint =
            poem::UnicodeCharForKey(app->activity, keyCode, AKeyEvent_getMetaState(event));
        if (codepoint >= 32) {
            poem::protocol::Event ch;
            ch.type = poem::protocol::EventType::KeyChar;
            ch.ch = static_cast<std::uint32_t>(codepoint);
            QueueEvent(ch);
        }
        return 1;
    }
    if (action == AKEY_EVENT_ACTION_UP && vk != 0) {
        poem::protocol::Event up;
        up.type = poem::protocol::EventType::KeyUp;
        up.keycode = vk;
        QueueEvent(up);
        return 1;
    }
    return 0;
}

int32_t HandleInput(android_app* app, AInputEvent* event) {
    if (AInputEvent_getType(event) == AINPUT_EVENT_TYPE_KEY) return HandleKey(app, event);
    if (AInputEvent_getType(event) != AINPUT_EVENT_TYPE_MOTION) return 0;
    const auto action = AMotionEvent_getAction(event) & AMOTION_EVENT_ACTION_MASK;
    Host* host = static_cast<Host*>(app->userData);
    // Touch arrives in physical pixels; the engine hit-tests in logical.
    const auto x = static_cast<std::int32_t>(AMotionEvent_getX(event, 0) / host->scale);
    const auto y = static_cast<std::int32_t>(AMotionEvent_getY(event, 0) / host->scale);

    poem::protocol::Event out;
    out.x = x;
    out.y = y;
    switch (action) {
    case AMOTION_EVENT_ACTION_DOWN: {
        // A touch has no hover phase; synthesize the move so hit-testing sees
        // the pointer where the press lands, exactly as a mouse would.
        poem::protocol::Event move;
        move.type = poem::protocol::EventType::MouseMove;
        move.x = x;
        move.y = y;
        QueueEvent(move);
        out.type = poem::protocol::EventType::MouseDown;
        out.button = 1;
        break;
    }
    case AMOTION_EVENT_ACTION_MOVE:
        out.type = poem::protocol::EventType::MouseMove;
        break;
    case AMOTION_EVENT_ACTION_UP:
        out.type = poem::protocol::EventType::MouseUp;
        out.button = 1;
        break;
    default:
        return 0;
    }
    QueueEvent(out);
    return 1;
}

} // namespace

void android_main(android_app* app) {
    g_host.app = app;
    app->userData = &g_host;
    app->onAppCmd = HandleCmd;
    app->onInputEvent = HandleInput;
    HLOGI("poem android host entered");

    while (true) {
        int events;
        android_poll_source* source;
        while (ALooper_pollOnce(g_host.animating ? 0 : -1, nullptr, &events, (void**)&source) >= 0) {
            if (source) source->process(app, source);
            if (app->destroyRequested) {
                poem::protocol::Event close;
                close.type = poem::protocol::EventType::WindowClose;
                QueueEvent(close);
                FlushEvents();
                g_host.transportRunning.store(false);
                TermDisplay(&g_host);
                if (g_host.transportThread.joinable()) g_host.transportThread.detach();
                return;
            }
        }
        if (g_host.animating) {
            FlushEvents();
            DrawFrame(&g_host);
        }
    }
}
