// POEM Android presenter host: a NativeActivity that embeds the Go engine
// (built as a c-shared library) and presents its frames via GLES2.
//
// Process shape: android_main owns the EGL surface and render loop; a
// dedicated transport thread drains the engine→presenter byte stream through
// the Go export PoemHostRead (framed exactly like the Windows named pipes:
// 4-byte little-endian length + POEM v3 envelope); input events accumulate
// and flush to the engine as one framed EventBatch per frame through
// PoemHostWrite — one batched cgo crossing each way per frame.
#include <android/log.h>
#include <android_native_app_glue.h>
#include <EGL/egl.h>
#include <GLES2/gl2.h>

#include <algorithm>
#include <array>
#include <atomic>
#include <chrono>
#include <cmath>
#include <cstdlib>
#include <cstring>
#include <mutex>
#include <optional>
#include <string>
#include <thread>
#include <vector>

#include "audio_aaudio.h"
#include "ime_jni.h"
#include "poem/protocol.h"
#include "renderer_gles.h"

#define HLOGI(...) __android_log_print(ANDROID_LOG_INFO, "poem-host", __VA_ARGS__)
#define HLOGE(...) __android_log_print(ANDROID_LOG_ERROR, "poem-host", __VA_ARGS__)

extern "C" {
// Exported by the Go application library. dataDir is the app's private writable
// directory (see PoemAndroidStart call site).
int PoemHostRead(void* buf, int capacity);
int PoemHostWrite(void* buf, int length);
// Exported by the application's Go main package: registers the app and calls
// mobile.Start with the initial surface size in physical pixels.
void PoemAndroidStart(int width, int height, const char* dataDir);
}

namespace {

struct Host {
    android_app* app = nullptr;
    std::string dataDir;         // app private writable dir, passed to the Go app
    EGLDisplay display = EGL_NO_DISPLAY;
    EGLSurface surface = EGL_NO_SURFACE;
    EGLContext context = EGL_NO_CONTEXT;
    int width = 0, height = 0;   // physical surface pixels
    float scale = 1.0f;          // display density (physical px per logical px)
    int logicalW = 0, logicalH = 0;
    int insetX = 0, insetY = 0;   // applied content origin (system bars)
    std::atomic<bool> imeVisible{false}; // set by SetImeVisible on the transport thread
    int imeBottom = 0;            // current IME inset (physical px), render-loop owned
    // contentBottom is the window's visible content bottom (physical px) as
    // reported by onContentRectChanged — the framework's own UI-thread signal
    // for adjustResize/IME geometry. 0 = never reported.
    std::atomic<int> contentBottom{0};
    int pendingInsets[4] = {0, 0, 0, 0}; // l,t,r,b from getRootWindowInsets

    // Touch gesture classification (logical px): a press is Undecided until
    // it moves past the slop; mostly-vertical movement becomes a Scroll
    // (drag translates to wheel events), other movement becomes a Drag
    // (deferred MouseDown at the start point, then moves — sliders), and a
    // release while still Undecided is a Tap (move+down+up).
    enum class Touch { None, Undecided, Scroll, Drag };
    Touch touch = Touch::None;
    int touchStartX = 0, touchStartY = 0;
    int lastTouchX = 0, lastTouchY = 0;
    std::chrono::steady_clock::time_point touchDownAt{};
    bool pinching = false;
    float pinchDistance = 0.0f;
    int pinchX = 0, pinchY = 0;

    // Velocity samples for fling: recent (time, y) pairs from the live drag.
    static constexpr int kVelocitySamples = 4;
    struct VelocitySample {
        std::chrono::steady_clock::time_point at;
        int y;
    };
    VelocitySample velocity[kVelocitySamples]{};
    int velocityHead = 0;

    // Fling state: after a fast release the page keeps scrolling with
    // exponentially decaying velocity (px/s, positive = scroll down),
    // stepped every frame by StepFling. A touch during a fling stops it and
    // is consumed (standard Android: the tap that catches a moving list
    // does not click anything).
    bool flinging = false;
    bool flingGesture = false;
    float flingVelocity = 0.0f;
    float flingRemainder = 0.0f;
    bool eatNextTap = false;
    std::chrono::steady_clock::time_point flingLastStep{};
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
    std::atomic<bool> transportRunning{false};
};

Host g_host;

void QueueEvent(const poem::protocol::Event& event) {
    std::lock_guard<std::mutex> lock(g_host.eventMutex);
    g_host.pendingEvents.push_back(event);
}

void QueueGesture(poem::protocol::EventType type, poem::protocol::GesturePhase phase,
                  int x, int y, int dx, int dy, float scale) {
    poem::protocol::Event gesture;
    gesture.type = type;
    gesture.x = x;
    gesture.y = y;
    gesture.deltaX = dx;
    gesture.deltaY = dy;
    gesture.scale = scale;
    gesture.phase = phase;
    QueueEvent(gesture);
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
            case poem::protocol::MessageType::MapSceneDelta: {
                auto scene = poem::protocol::DecodeMapSceneDelta(envelope.body);
                HLOGI("map scene: viewport=%s gen=%llu resources=%zu draws=%zu sun=%.1f/%.1f",
                      scene.viewportId.c_str(), static_cast<unsigned long long>(scene.generation),
                      scene.resources.size(), scene.draws.size(), scene.sunAzimuth, scene.sunElevation);
                for (std::size_t i = 0; i < scene.draws.size(); ++i) {
                    const auto& d = scene.draws[i];
                    HLOGI("  draw[%zu] prim=%d count=%u layer=%d opacity=%.2f depthTest=%d",
                          i, static_cast<int>(d.primitive), d.count, d.layer, d.opacity, d.depthTest ? 1 : 0);
                }
                std::lock_guard<std::mutex> lock(g_host.frameMutex);
                g_host.renderer.ApplyMapScene(scene);
                break;
            }
            case poem::protocol::MessageType::SetImeVisible: {
                const auto ime = poem::protocol::DecodeSetImeVisible(envelope.body);
                HLOGI("ime visible -> %d", ime.visible ? 1 : 0);
                g_host.imeVisible.store(ime.visible);
                poem::SetSoftKeyboardVisible(g_host.app->activity, ime.visible);
                break;
            }
            case poem::protocol::MessageType::PlaySound: {
                static poem::AudioEngineAAudio audio;
                audio.Play(poem::protocol::DecodePlaySound(envelope.body).type);
                break;
            }
            case poem::protocol::MessageType::SemanticTree:
                break; // no accessibility bridge on this presenter yet
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
                              EGL_ALPHA_SIZE, 8,
                              // Depth for the GPU map path (extrusion occlusion);
                              // drivers grant at least this many bits.
                              EGL_DEPTH_SIZE, 16, EGL_NONE};
    EGLConfig config;
    EGLint numConfigs = 0;
    eglChooseConfig(host->display, attribs, &config, 1, &numConfigs);
    host->surface = eglCreateWindowSurface(host->display, config, host->app->window, nullptr);
    // Negotiate GLES3 (derivatives + 32-bit indices are core, needed by the
    // GPU map path) and fall back to GLES2, where the presenter checks the
    // equivalent extensions and otherwise keeps the CPU painter path.
    const EGLint ctx3[] = {EGL_CONTEXT_CLIENT_VERSION, 3, EGL_NONE};
    host->context = eglCreateContext(host->display, config, EGL_NO_CONTEXT, ctx3);
    if (host->context == EGL_NO_CONTEXT) {
        const EGLint ctx2[] = {EGL_CONTEXT_CLIENT_VERSION, 2, EGL_NONE};
        host->context = eglCreateContext(host->display, config, EGL_NO_CONTEXT, ctx2);
        HLOGI("EGL context: GLES2 fallback");
    } else {
        HLOGI("EGL context: GLES3");
    }
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
        PoemAndroidStart(host->logicalW, host->logicalH, g_host.dataDir.c_str());
        host->transportRunning.store(true);
        std::thread(TransportLoop).detach(); // process-lifetime thread
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
    poem::protocol::Event capabilities;
    capabilities.type = poem::protocol::EventType::Capabilities;
    capabilities.value = R"({"pointer":1,"hover":false,"keyboard":true,"touch":true,"trackpad":false,"stylus":false,"density":0,"textScale":1,"reducedMotion":false,"highContrast":false})";
    QueueEvent(capabilities);
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

// CheckSurfaceSize reconciles the layout area with reality: the EGL surface
// can change under us (rotation with configChanges declared; which app-cmd
// announces it varies by version) and system bars overlap its edges (the
// content rect from the glue). The engine lays out in the logical CONTENT
// size; the renderer shifts drawing by the content origin; touch translates
// back. Compared every frame — the checks are just two eglQuerySurface calls
// and integer compares.
void CheckSurfaceSize(Host* host) {
    if (host->display == EGL_NO_DISPLAY) return;
    EGLint w = 0, h = 0;
    eglQuerySurface(host->display, host->surface, EGL_WIDTH, &w);
    eglQuerySurface(host->display, host->surface, EGL_HEIGHT, &h);
    if (w <= 0 || h <= 0) return;

    // System-bar insets come from getRootWindowInsets over JNI — on modern
    // edge-to-edge Android the glue's contentRect stays empty, verified on
    // both the API 36 emulator and a physical device. The JNI hop costs an
    // attach, so it runs when the surface changed or on a slow heartbeat
    // (bars can change without a resize, e.g. nav-mode switches).
    static int insetTick = 0;
    const bool sizeChanged = (w != host->width || h != host->height);
    if (sizeChanged || (insetTick++ % 120) == 0) {
        int raw[4] = {0, 0, 0, 0};
        if (poem::GetSystemInsets(host->app->activity, raw)) {
            host->pendingInsets[0] = raw[0];
            host->pendingInsets[1] = raw[1];
            host->pendingInsets[2] = raw[2];
            host->pendingInsets[3] = raw[3];
        }
    }

    // The soft keyboard is one more bottom inset: on edge-to-edge Android
    // (API 35+) adjustResize no longer shrinks the surface, so the viewport
    // must shrink by the IME inset instead; it replaces the nav-bar inset
    // while larger (the keyboard covers the nav bar). The engine's
    // SetImeVisible is the authoritative "keyboard up" signal — the polled
    // inset flakes to 0 transiently on some OEMs (seen on HyperOS, where it
    // briefly reported 0 mid-session and the viewport snapped back under the
    // keyboard) — so while visible we keep the last non-zero reading and
    // only accept 0 once it is sustained (~0.5s: the user dismissed the IME
    // with the system chevron, which bypasses our focus tracking).
    // IME inset strategy, learned the hard way on HyperOS: View-inset JNI
    // reads from the render thread are only trustworthy immediately after
    // the keyboard opens — seconds later they flake to zero/stale, and the
    // framework's onContentRectChanged never fires for the IME on a
    // fullscreen NativeActivity. So the inset is captured ONCE per IME
    // session (first frames after our own SetImeVisible, the one moment the
    // reading is fresh) and held until the engine says the keyboard is gone;
    // if even the first reading fails, a 42%-of-surface estimate is close
    // enough for every mainstream keyboard.
    if (host->imeVisible.load()) {
        if (host->imeBottom == 0) {
            const int polled = poem::GetImeInset(host->app->activity);
            host->imeBottom = polled > 0 ? polled : static_cast<int>(h * 0.42f);
        }
    } else {
        host->imeBottom = 0;
    }
    const int insetX = host->pendingInsets[0];
    const int insetY = host->pendingInsets[1];
    const int bottomInset = std::max(host->pendingInsets[3], host->imeBottom);
    const int contentW = std::max(1, w - insetX - host->pendingInsets[2]);
    const int contentH = std::max(1, h - insetY - bottomInset);

    const int logicalW = static_cast<int>(contentW / host->scale);
    const int logicalH = static_cast<int>(contentH / host->scale);
    if (w == host->width && h == host->height &&
        insetX == host->insetX && insetY == host->insetY &&
        logicalW == host->logicalW && logicalH == host->logicalH) {
        return;
    }
    host->width = w;
    host->height = h;
    host->insetX = insetX;
    host->insetY = insetY;
    host->logicalW = logicalW;
    host->logicalH = logicalH;
    host->renderer.Resize(w, h);
    host->renderer.SetInset(insetX, insetY);
    HLOGI("geometry: surface %dx%d, content inset %d,%d => logical %dx%d",
          w, h, insetX, insetY, logicalW, logicalH);
    poem::protocol::Event resize;
    resize.type = poem::protocol::EventType::WindowSize;
    resize.width = logicalW;
    resize.height = logicalH;
    QueueEvent(resize);
}

void DrawFrame(Host* host) {
    if (host->display == EGL_NO_DISPLAY) return;
    CheckSurfaceSize(host);
    std::optional<poem::protocol::InitEngine> atlas;
    std::optional<poem::protocol::RenderFrame> frame;
    {
        std::lock_guard<std::mutex> lock(host->frameMutex);
        atlas.swap(host->pendingAtlas);
        frame = host->latestFrame;
    }
    if (atlas) host->renderer.UploadAtlas(*atlas);
    if (frame) host->renderer.Render(*frame);
    if (!eglSwapBuffers(host->display, host->surface)) {
        // EGL_BAD_SURFACE etc. — the surface died under us (seen during
        // aggressive lifecycle churn); tear down and wait for INIT_WINDOW.
        HLOGE("eglSwapBuffers failed (0x%x), dropping surface", eglGetError());
        TermDisplay(host);
    }
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
    case APP_CMD_CONTENT_RECT_CHANGED:
        // The framework's UI-thread signal for adjustResize geometry (the
        // IME opening/closing, system-bar changes). Polling View insets from
        // the render thread proved unreliable on HyperOS; this is
        // authoritative when present.
        HLOGI("content rect: %d,%d-%d,%d", app->contentRect.left, app->contentRect.top,
              app->contentRect.right, app->contentRect.bottom);
        host->contentBottom.store(app->contentRect.bottom);
        CheckSurfaceSize(host);
        break;
    case APP_CMD_CONFIG_CHANGED:
        // Rotation: same activity, new geometry.
        CheckSurfaceSize(host);
        break;
    case APP_CMD_GAINED_FOCUS:
    case APP_CMD_RESUME:
        host->animating = host->display != EGL_NO_DISPLAY;
        break;
    case APP_CMD_LOST_FOCUS:
    case APP_CMD_PAUSE:
    case APP_CMD_STOP:
        // Background: stop presenting. The engine keeps running (its state is
        // the app's state); frames simply stop being drawn until resume.
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
    const auto x = static_cast<std::int32_t>((AMotionEvent_getX(event, 0) - host->insetX) / host->scale);
    const auto y = static_cast<std::int32_t>((AMotionEvent_getY(event, 0) - host->insetY) / host->scale);

    auto queueMove = [](int mx, int my) {
        poem::protocol::Event move;
        move.type = poem::protocol::EventType::MouseMove;
        move.x = mx;
        move.y = my;
        QueueEvent(move);
    };
    auto queueButton = [](poem::protocol::EventType type, int bx, int by) {
        poem::protocol::Event button;
        button.type = type;
        button.x = bx;
        button.y = by;
        button.button = 1;
        QueueEvent(button);
    };
    auto queueGesture = [](poem::protocol::EventType type, poem::protocol::GesturePhase phase,
                           int gx, int gy, int dx, int dy, float scale) {
        poem::protocol::Event gesture;
        gesture.type = type;
        gesture.x = gx;
        gesture.y = gy;
        gesture.deltaX = dx;
        gesture.deltaY = dy;
        gesture.scale = scale;
        gesture.phase = phase;
        QueueEvent(gesture);
    };

    const auto pointerCount = AMotionEvent_getPointerCount(event);
    auto pinchMetrics = [event, host]() {
        const float x0 = (AMotionEvent_getX(event, 0) - host->insetX) / host->scale;
        const float y0 = (AMotionEvent_getY(event, 0) - host->insetY) / host->scale;
        const float x1 = (AMotionEvent_getX(event, 1) - host->insetX) / host->scale;
        const float y1 = (AMotionEvent_getY(event, 1) - host->insetY) / host->scale;
        const float dx = x1 - x0;
        const float dy = y1 - y0;
        return std::array<float, 3>{(x0 + x1) * 0.5f, (y0 + y1) * 0.5f,
                                    std::max(1.0f, std::sqrt(dx * dx + dy * dy))};
    };

    if (action == AMOTION_EVENT_ACTION_POINTER_DOWN && pointerCount >= 2) {
        const auto metrics = pinchMetrics();
        if (host->flingGesture) {
            QueueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::End,
                         host->lastTouchX, host->lastTouchY, 0, 0, 1.0f);
            host->flingGesture = false;
        }
        host->pinching = true;
        host->flinging = false;
        host->touch = Host::Touch::None;
        host->pinchX = static_cast<int>(metrics[0]);
        host->pinchY = static_cast<int>(metrics[1]);
        host->pinchDistance = metrics[2];
        queueGesture(poem::protocol::EventType::PinchGesture, poem::protocol::GesturePhase::Begin,
                     host->pinchX, host->pinchY, 0, 0, 1.0f);
        return 1;
    }
    if (host->pinching && action == AMOTION_EVENT_ACTION_MOVE && pointerCount >= 2) {
        const auto metrics = pinchMetrics();
        const int cx = static_cast<int>(metrics[0]);
        const int cy = static_cast<int>(metrics[1]);
        const float scaleDelta = metrics[2] / std::max(1.0f, host->pinchDistance);
        queueGesture(poem::protocol::EventType::PinchGesture, poem::protocol::GesturePhase::Update,
                     cx, cy, cx - host->pinchX, cy - host->pinchY, scaleDelta);
        host->pinchX = cx;
        host->pinchY = cy;
        host->pinchDistance = metrics[2];
        return 1;
    }
    if (host->pinching && (action == AMOTION_EVENT_ACTION_POINTER_UP || action == AMOTION_EVENT_ACTION_UP ||
                           action == AMOTION_EVENT_ACTION_CANCEL)) {
        queueGesture(poem::protocol::EventType::PinchGesture,
                     action == AMOTION_EVENT_ACTION_CANCEL ? poem::protocol::GesturePhase::Cancel
                                                           : poem::protocol::GesturePhase::End,
                     host->pinchX, host->pinchY, 0, 0, 1.0f);
        host->pinching = false;
        host->touch = Host::Touch::None;
        return 1;
    }

    // Slop is generous (≈14 logical px): at high densities a firm tap wobbles
    // more physical pixels than a mouse ever would, and a wobble that
    // classifies as Scroll shifts the page out from under the tap.
    constexpr int kSlop = 14;

    auto recordVelocitySample = [host](int sy) {
        host->velocity[host->velocityHead % Host::kVelocitySamples] =
            Host::VelocitySample{std::chrono::steady_clock::now(), sy};
        host->velocityHead++;
    };

    switch (action) {
    case AMOTION_EVENT_ACTION_DOWN:
        // Nothing is forwarded yet: a press only becomes a tap, drag, or
        // scroll once we see how it moves — or holds (hold-to-grab below).
        // Catching a fling stops it and consumes the tap.
        host->eatNextTap = host->flinging;
        if (host->flingGesture) {
            QueueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::End,
                         host->lastTouchX, host->lastTouchY, 0, 0, 1.0f);
            host->flingGesture = false;
        }
        host->flinging = false;
        host->touch = Host::Touch::Undecided;
        host->touchStartX = x;
        host->touchStartY = y;
        host->lastTouchX = x;
        host->lastTouchY = y;
        host->touchDownAt = std::chrono::steady_clock::now();
        host->velocityHead = 0;
        recordVelocitySample(y);
        return 1;

    case AMOTION_EVENT_ACTION_MOVE: {
        if (host->touch == Host::Touch::Undecided) {
            const int dx = x - host->touchStartX;
            const int dy = y - host->touchStartY;
            // Scroll needs a clearly vertical intent; anything decisively
            // horizontal is a drag (sliders). Held presses become drags via
            // PromoteHeldTouch before ever moving.
            if (std::abs(dy) > kSlop && std::abs(dy) > std::abs(dx) * 3 / 2) {
                host->touch = Host::Touch::Scroll;
                host->lastTouchX = x;
                host->lastTouchY = y;
                queueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::Begin,
                             x, y, 0, 0, 1.0f);
            } else if (std::abs(dx) > kSlop) {
                host->touch = Host::Touch::Drag;
                queueMove(host->touchStartX, host->touchStartY);
                queueButton(poem::protocol::EventType::MouseDown, host->touchStartX, host->touchStartY);
            } else {
                return 1;
            }
        }
        if (host->touch == Host::Touch::Scroll) {
            // Pixel-accurate: every move streams its own wheel event whose
            // delta is the finger movement ×1.2 (the engine scales ×100/120
            // back to pixels), so the content tracks the finger 1:1.
            const int dx = x - host->lastTouchX;
            const int dy = y - host->lastTouchY;
            host->lastTouchX = x;
            host->lastTouchY = y;
            recordVelocitySample(y);
            if (dx != 0 || dy != 0) {
                queueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::Update,
                             x, y, dx, dy, 1.0f);
            }
        } else if (host->touch == Host::Touch::Drag) {
            queueMove(x, y);
        }
        return 1;
    }

    case AMOTION_EVENT_ACTION_UP:
    case AMOTION_EVENT_ACTION_CANCEL:
        if (host->touch == Host::Touch::Undecided && action == AMOTION_EVENT_ACTION_UP) {
            if (!host->eatNextTap) {
                // A clean tap: the full click sequence at the press point.
                queueMove(host->touchStartX, host->touchStartY);
                queueButton(poem::protocol::EventType::MouseDown, host->touchStartX, host->touchStartY);
                queueButton(poem::protocol::EventType::MouseUp, host->touchStartX, host->touchStartY);
            }
        } else if (host->touch == Host::Touch::Drag) {
            queueButton(poem::protocol::EventType::MouseUp, x, y);
        } else if (host->touch == Host::Touch::Scroll && action == AMOTION_EVENT_ACTION_CANCEL) {
            queueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::Cancel,
                         x, y, 0, 0, 1.0f);
        } else if (host->touch == Host::Touch::Scroll && action == AMOTION_EVENT_ACTION_UP) {
            // Release velocity (px/s over the recent sample window) becomes a
            // fling: the page keeps moving and decays frame by frame.
            const auto now = std::chrono::steady_clock::now();
            const int samples = std::min(host->velocityHead, Host::kVelocitySamples);
            for (int i = samples; i > 0; --i) {
                const auto& oldest = host->velocity[(host->velocityHead - i) % Host::kVelocitySamples];
                const float dt = std::chrono::duration<float>(now - oldest.at).count();
                if (dt > 0.02f && dt < 0.25f) {
                    const float v = static_cast<float>(oldest.y - y) / dt; // finger up = scroll down
                    if (std::abs(v) > 180.0f) {
                        host->flinging = true;
                        host->flingGesture = true;
                        host->flingVelocity = std::min(std::max(v * 1.4f, -10000.0f), 10000.0f);
                        host->flingRemainder = 0.0f;
                        host->flingLastStep = now;
                    }
                    break;
                }
            }
            if (!host->flinging) {
                queueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::End,
                             x, y, 0, 0, 1.0f);
            }
        }
        host->eatNextTap = false;
        host->touch = Host::Touch::None;
        return 1;

    default:
        return 0;
    }
}

} // namespace

void android_main(android_app* app) {
    g_host.app = app;
    app->userData = &g_host;
    app->onAppCmd = HandleCmd;
    app->onInputEvent = HandleInput;
    // Resolve the app's private, writable data directory and hand it to the Go
    // app explicitly (see PoemAndroidStart). internalDataPath is null on some
    // devices/emulators, so fall back to Context.getFilesDir() over JNI. HOME is
    // also set as a convenience for os.UserConfigDir-based lookups.
    if (app->activity) {
        if (app->activity->internalDataPath) {
            g_host.dataDir = app->activity->internalDataPath;
        }
        if (g_host.dataDir.empty()) {
            g_host.dataDir = poem::AppFilesDir(app->activity);
        }
        if (!g_host.dataDir.empty()) {
            setenv("HOME", g_host.dataDir.c_str(), 1);
        }
        HLOGI("app data dir: %s", g_host.dataDir.empty() ? "(unresolved)" : g_host.dataDir.c_str());
    }
    HLOGI("poem android host entered");

    while (true) {
        int events;
        android_poll_source* source;
        while (ALooper_pollOnce(g_host.animating ? 0 : -1, nullptr, &events, (void**)&source) >= 0) {
            if (source) source->process(app, source);
            if (app->destroyRequested) {
                // The activity is going away but the PROCESS may be reused:
                // Android can relaunch the activity into this process, which
                // re-enters android_main. The Go engine and its transport
                // thread deliberately stay alive (globals survive), so the
                // relaunch path is just INIT_WINDOW → restore atlas → resize
                // event. Sending WindowClose here would kill the engine and
                // leave any reused process permanently black.
                TermDisplay(&g_host);
                return;
            }
        }
        // Fling: after a fast scroll release the page keeps moving, slowing
        // exponentially (τ≈0.3s; release velocity ×1.4 so flicks launch fast and settle fast) — the standard Android inertia feel. Each
        // frame converts the elapsed motion into one pan update; fractional
        // pixels carry in flingRemainder so slow tails still move.
        if (g_host.flinging) {
            const auto now = std::chrono::steady_clock::now();
            float dt = std::chrono::duration<float>(now - g_host.flingLastStep).count();
            if (dt > 0.05f) dt = 0.05f;
            g_host.flingLastStep = now;
            const float travel = g_host.flingVelocity * dt + g_host.flingRemainder;
            const int whole = static_cast<int>(travel);
            g_host.flingRemainder = travel - static_cast<float>(whole);
            if (whole != 0) {
                QueueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::Update,
                             g_host.lastTouchX, g_host.lastTouchY, 0, -whole, 1.0f);
            }
            g_host.flingVelocity *= std::exp(-dt / 0.3f);
            if (std::abs(g_host.flingVelocity) < 80.0f) {
                g_host.flinging = false;
                if (g_host.flingGesture) {
                    QueueGesture(poem::protocol::EventType::PanGesture, poem::protocol::GesturePhase::End,
                                 g_host.lastTouchX, g_host.lastTouchY, 0, 0, 1.0f);
                    g_host.flingGesture = false;
                }
            }
        }

        // Hold-to-grab: a press held past ~220ms without crossing the slop
        // becomes a Drag — the deferred MouseDown lands, the control (e.g. a
        // slider) grabs, and later movement drags it instead of scrolling.
        if (g_host.touch == Host::Touch::Undecided &&
            std::chrono::steady_clock::now() - g_host.touchDownAt > std::chrono::milliseconds(220)) {
            g_host.touch = Host::Touch::Drag;
            poem::protocol::Event move;
            move.type = poem::protocol::EventType::MouseMove;
            move.x = g_host.touchStartX;
            move.y = g_host.touchStartY;
            QueueEvent(move);
            poem::protocol::Event down;
            down.type = poem::protocol::EventType::MouseDown;
            down.x = g_host.touchStartX;
            down.y = g_host.touchStartY;
            down.button = 1;
            QueueEvent(down);
        }
        if (g_host.animating) {
            FlushEvents();
            DrawFrame(&g_host);
        }
    }
}
