# Technical Design Document
## Cross-Platform C++ Application Framework

**Document status:** Draft 1.0  
**Primary implementation language:** C++20 or later  
**First-class target platforms:** Android, iOS, Windows, macOS, Linux, and Web  
**Build systems:** CMake, Gradle, Xcode, platform packaging tools  
**Audience:** Framework engineers, platform engineers, application developers, build engineers, and maintainers

---

# 1. Executive Summary

This document defines a cross-platform application framework whose shared runtime, application logic, UI model, rendering abstractions, networking, storage, media, and service layer are implemented primarily in modern C++.

The framework targets all of the following as first-class platforms:

- Android
- iOS
- Windows
- macOS
- Linux
- Web through WebAssembly

The design does not require applications to be rewritten in Swift for iOS, Kotlin for Android, C# for Windows, or JavaScript for the web. Instead, each operating system provides a thin host responsible for process startup, native lifecycle events, system APIs, packaging, code signing, permissions, and presentation surfaces.

The architectural rule is:

> Portable application behavior belongs in shared C++; platform-specific behavior belongs behind narrow, explicit platform interfaces.

The framework supports two UI modes:

1. **Framework-rendered UI**, where layout, styling, widgets, input, and rendering are shared across every target.
2. **Native-shell UI**, where SwiftUI, UIKit, Android Views, Jetpack Compose, WinUI, AppKit, GTK, or browser DOM components call into the shared C++ runtime.

Both modes may coexist in the same application.

---

# 2. Goals

## 2.1 Primary Goals

The framework shall:

- compile one C++ application codebase for Android, iOS, Windows, macOS, Linux, and Web;
- share the majority of application logic and framework behavior;
- isolate operating-system dependencies behind narrow interfaces;
- support native performance and deterministic resource ownership;
- support Vulkan, Metal, Direct3D, OpenGL, and WebGPU rendering backends;
- provide a portable application lifecycle;
- support touch, mouse, keyboard, pen, controller, and accessibility input;
- provide portable networking, storage, audio, task scheduling, and resource management;
- support both custom-rendered UI and native platform UI;
- support platform-native packaging and distribution;
- support automated testing on desktop and mobile targets;
- support headless execution for tests, servers, and command-line tools;
- provide stable interfaces suitable for long-term framework evolution.

## 2.2 Secondary Goals

The framework should:

- minimize language-boundary calls;
- support plugins and optional modules;
- support hot asset reload during development;
- expose platform capabilities at runtime;
- allow applications to opt into native services incrementally;
- support deterministic builds where practical;
- remain usable without a mandatory scripting language;
- permit future Rust, C, or other-language bindings without redesigning the core.

---

# 3. Non-Goals

The framework will not:

- pretend that all platforms expose identical behavior;
- emulate every native API on every other platform;
- bypass Apple, Google, Microsoft, Linux distribution, or browser security rules;
- eliminate the need for Xcode when signing and packaging iOS or macOS applications;
- eliminate the Android SDK, NDK, or Gradle for Android builds;
- guarantee pixel-identical output across different GPUs and text-rendering stacks;
- permit unrestricted runtime code downloading on platforms that prohibit it;
- expose raw native objects throughout the portable application layer;
- depend on exceptions crossing ABI or language boundaries.

---

# 4. Design Principles

## 4.1 Shared Core, Thin Platform Hosts

The framework core owns:

- application state;
- navigation state;
- domain logic;
- UI state and layout;
- rendering commands;
- asset loading;
- networking policy;
- serialization;
- task scheduling;
- logging;
- telemetry;
- portable storage abstractions.

Platform hosts own:

- process startup;
- native event loops;
- windows, views, and surfaces;
- operating-system lifecycle callbacks;
- permissions;
- file and directory pickers;
- camera, microphone, sensors, and location;
- notifications;
- secure key storage;
- app-store and payment integration;
- deep links;
- native accessibility bridges;
- packaging, signing, and deployment.

## 4.2 No Native Types in Shared Public Headers

Shared public headers must not expose platform types such as:

- `JNIEnv*`, `jobject`, or Java classes;
- Swift or Objective-C object pointers;
- `UIView`, `CAMetalLayer`, or `NSWindow`;
- Win32 `HWND` or COM interfaces;
- X11, Wayland, GTK, or Qt types;
- JavaScript handles or browser DOM types.

Platform resources must be represented using framework-defined interfaces or opaque handles.

## 4.3 Coarse-Grained Boundaries

Language and ABI boundaries should use operations such as:

```text
OpenFilePicker(options, completion)
RequestPermission(permission, completion)
ShowShareSheet(payload, completion)
RegisterPushNotifications(completion)
```

The framework should avoid large volumes of tiny cross-boundary calls per frame.

## 4.4 Capability-Based Behavior

Applications must query platform capabilities rather than assume their presence.

```cpp
if (platform.capabilities().cameraCapture) {
    cameraButton.setEnabled(true);
}
```

## 4.5 Explicit Ownership

Every boundary object must specify:

- creator;
- owner;
- lifetime;
- thread affinity;
- destruction mechanism;
- callback cancellation behavior.

## 4.6 Asynchronous Native Services

Services involving user interaction or operating-system scheduling must be asynchronous, including:

- permissions;
- file pickers;
- camera capture;
- biometric authentication;
- purchases;
- notification registration;
- browser dialogs;
- location updates;
- background execution requests.

---

# 5. High-Level Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                       Application Code                       │
│ Screens, workflows, domain logic, assets, configuration      │
└──────────────────────────────┬───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                    Shared C++ Framework                      │
│ Runtime | UI | Rendering | Input | Audio | Network | Storage │
│ Tasks | Resources | Navigation | Logging | Serialization     │
└──────────────────────────────┬───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                  Platform Abstraction Layer                  │
│ Lifecycle | Window | Files | Clipboard | Haptics | Camera    │
│ Notifications | Location | Security | Accessibility | Store │
└──────┬─────────┬─────────┬─────────┬─────────┬───────────────┘
       │         │         │         │         │
 Android       iOS      Windows    macOS      Linux          Web
 Kotlin/      Swift/     Win32/     AppKit/    Wayland/       JS glue/
 Java+JNI     ObjC++     WinUI      ObjC++     X11            Web APIs
```

---

# 6. Repository Structure

```text
framework/
├── CMakeLists.txt
├── CMakePresets.json
├── cmake/
│   ├── CompilerOptions.cmake
│   ├── Dependencies.cmake
│   ├── PlatformDetection.cmake
│   ├── Sanitizers.cmake
│   └── toolchains/
│       ├── android.cmake
│       ├── ios.cmake
│       ├── macos.cmake
│       ├── windows.cmake
│       ├── linux.cmake
│       └── emscripten.cmake
├── include/framework/
│   ├── app/
│   ├── async/
│   ├── audio/
│   ├── core/
│   ├── graphics/
│   ├── input/
│   ├── io/
│   ├── logging/
│   ├── math/
│   ├── network/
│   ├── platform/
│   ├── storage/
│   ├── ui/
│   └── version.hpp
├── src/
│   ├── app/
│   ├── async/
│   ├── audio/
│   ├── core/
│   ├── graphics/
│   ├── input/
│   ├── io/
│   ├── logging/
│   ├── network/
│   ├── storage/
│   └── ui/
├── platform/
│   ├── android/
│   ├── ios/
│   ├── windows/
│   ├── macos/
│   ├── linux/
│   └── web/
├── render/
│   ├── common/
│   ├── vulkan/
│   ├── metal/
│   ├── d3d12/
│   ├── opengl/
│   └── webgpu/
├── examples/
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── conformance/
│   └── platform/
├── tools/
│   ├── asset_pipeline/
│   ├── codegen/
│   ├── packaging/
│   └── diagnostics/
├── docs/
│   ├── architecture/
│   ├── api/
│   ├── platform/
│   └── adr/
└── third_party/
```

---

# 7. Core Runtime

## 7.1 Application Interface

```cpp
namespace fw {

class Application {
public:
    virtual ~Application() = default;

    virtual void onCreate() = 0;
    virtual void onStart() = 0;
    virtual void onResume() = 0;
    virtual void onPause() = 0;
    virtual void onStop() = 0;
    virtual void onDestroy() = 0;

    virtual void onFrame(double deltaSeconds) = 0;
    virtual void onEvent(const Event& event) = 0;
};

std::unique_ptr<Application> createApplication();

}
```

## 7.2 Runtime Responsibilities

The runtime coordinates:

- application creation and destruction;
- lifecycle transitions;
- frame scheduling;
- event dispatch;
- rendering;
- worker tasks;
- resource loading;
- platform callbacks;
- logging and crash context;
- orderly shutdown.

```cpp
class Runtime {
public:
    Runtime(PlatformServices& platform, RuntimeConfig config);

    void initialize();
    void start();
    void resume();
    void suspend();
    void stop();
    void tick(double deltaSeconds);
    void submitEvent(Event event);
    void shutdown();
};
```

## 7.3 Lifecycle State Machine

```text
Uninitialized
     │
     ▼
Initializing
     │
     ▼
Created
     │
     ▼
Started
     │
     ▼
Resumed ◄──────────┐
     │              │
     ▼              │
Paused ─────────────┘
     │
     ▼
Stopped
     │
     ▼
Destroyed
```

The runtime must tolerate repeated pause, suspend, resume, surface-loss, and window-recreation events.

---

# 8. Platform Abstraction Layer

## 8.1 PlatformServices

```cpp
class PlatformServices {
public:
    virtual ~PlatformServices() = default;

    virtual FileSystem& fileSystem() = 0;
    virtual Clipboard& clipboard() = 0;
    virtual Permissions& permissions() = 0;
    virtual Haptics& haptics() = 0;
    virtual Notifications& notifications() = 0;
    virtual LocationService& location() = 0;
    virtual SecureStorage& secureStorage() = 0;
    virtual ShareService& sharing() = 0;
    virtual DeviceInfo& deviceInfo() = 0;
    virtual Accessibility& accessibility() = 0;
    virtual PlatformCapabilities capabilities() const = 0;
};
```

## 8.2 Opaque Handles

```cpp
using NativeWindowHandle = std::uintptr_t;
using NativeViewHandle = std::uintptr_t;
using NativeSurfaceHandle = std::uintptr_t;
```

Only the relevant backend may interpret these values.

## 8.3 Platform Capability Model

```cpp
struct PlatformCapabilities {
    bool cameraCapture = false;
    bool location = false;
    bool haptics = false;
    bool biometrics = false;
    bool pushNotifications = false;
    bool backgroundTasks = false;
    bool nativeFilePicker = false;
    bool secureStorage = false;
    bool metal = false;
    bool vulkan = false;
    bool d3d12 = false;
    bool webgpu = false;
};
```

---

# 9. Android Platform Design

## 9.1 Toolchain

- Android Studio
- Android SDK
- Android NDK
- Gradle
- CMake
- Ninja
- NDK Clang
- Kotlin or Java host layer

## 9.2 Host Responsibilities

The Android host shall:

- create and own the Activity;
- load the native shared library;
- forward lifecycle events;
- create and destroy native rendering surfaces;
- translate touch, keyboard, controller, and sensor input;
- request permissions;
- invoke Android services;
- process intents and deep links;
- expose app directories and storage access;
- route notification events;
- preserve runtime state across Activity recreation where possible.

## 9.3 Native Library

The shared application is compiled into one or more `.so` files, for example:

```text
libframework_core.so
libapplication.so
```

## 9.4 JNI Boundary

JNI entry points should be small and stable.

```cpp
extern "C" JNIEXPORT jlong JNICALL
Java_com_example_NativeBridge_nativeCreate(JNIEnv*, jobject);

extern "C" JNIEXPORT void JNICALL
Java_com_example_NativeBridge_nativeResume(JNIEnv*, jobject, jlong);

extern "C" JNIEXPORT void JNICALL
Java_com_example_NativeBridge_nativePause(JNIEnv*, jobject, jlong);

extern "C" JNIEXPORT void JNICALL
Java_com_example_NativeBridge_nativeDestroy(JNIEnv*, jobject, jlong);
```

## 9.5 JNI Safety Rules

- Never retain local references beyond the JNI call.
- Use global references only where long-lived access is required.
- Release all global references during teardown.
- Attach native-created threads before making JNI calls.
- Do not allow C++ exceptions to cross JNI.
- Convert strings explicitly.
- Check pending Java exceptions.
- Cache method identifiers where appropriate.
- Document UI-thread requirements.

## 9.6 Android Rendering

Preferred backend order:

1. Vulkan
2. OpenGL ES fallback where required

The host supplies an `ANativeWindow` to the graphics backend. Surface recreation must not destroy the entire application runtime.

---

# 10. iOS Platform Design

## 10.1 Toolchain

- Xcode
- Apple Clang
- CMake or Xcode project generation
- Swift, Objective-C, or Objective-C++ host layer
- iOS SDK
- Metal framework
- code-signing certificates and provisioning profiles

## 10.2 Language Strategy

The C++ core remains unchanged. The iOS host may use:

- Swift for application and UI code;
- Objective-C++ for robust C++ bridging;
- direct Swift-to-C++ interoperability for compatible APIs;
- Objective-C-compatible C wrappers for especially stable ABI surfaces.

Recommended production boundary:

```text
Swift or UIKit
      │
Objective-C++ bridge
      │
Shared C++ runtime
```

## 10.3 Host Responsibilities

The iOS host shall:

- initialize the application and scene lifecycle;
- own `UIViewController` and native views;
- create and maintain the `CAMetalLayer` or `MTKView`;
- route touch, keyboard, pointer, controller, and motion events;
- request permissions;
- manage background and foreground transitions;
- invoke camera, photo library, location, notifications, and sharing APIs;
- expose secure storage through Keychain;
- handle universal links and custom URL schemes;
- bridge accessibility metadata.

## 10.4 Objective-C++ Wrapper Example

```objective-c++
@interface FWRuntimeBridge : NSObject
- (instancetype)initWithView:(UIView*)view;
- (void)start;
- (void)pause;
- (void)resume;
- (void)shutdown;
@end
```

The `.mm` implementation owns or references the C++ runtime.

## 10.5 iOS Restrictions

The design must account for:

- mandatory Apple signing and packaging;
- App Store policy restrictions;
- limited background execution;
- memory pressure termination;
- no unrestricted runtime native-code downloads;
- strict privacy declarations;
- scene and view recreation.

## 10.6 iOS Rendering

Metal is the primary renderer. A `CAMetalLayer` or `MTKView` is owned by the native host and passed to the Metal backend through an opaque platform handle.

---

# 11. Windows Platform Design

## 11.1 Toolchain

- Visual Studio or clang-cl
- MSVC toolchain
- CMake and Ninja or Visual Studio generators
- Windows SDK
- Win32, WinUI, or SDL-based host as selected

## 11.2 Host Responsibilities

The Windows host shall:

- create the process entry point;
- own the window and message loop;
- translate Win32 input and lifecycle events;
- manage DPI scaling and display changes;
- expose file dialogs, clipboard, notifications, and system services;
- implement secure storage using Windows facilities;
- handle URI activation and file associations;
- create a Direct3D or Vulkan rendering surface.

## 11.3 Rendering

Preferred backend order:

1. Direct3D 12
2. Vulkan
3. OpenGL fallback only where justified

## 11.4 Packaging

Supported deployment models may include:

- unpackaged Win32 application;
- MSIX package;
- Microsoft Store distribution;
- portable zip distribution.

---

# 12. macOS Platform Design

## 12.1 Toolchain

- Xcode
- Apple Clang
- macOS SDK
- Swift, Objective-C++, or AppKit host
- CMake or Xcode project generation

## 12.2 Host Responsibilities

The macOS host shall:

- own `NSApplication` and the main run loop;
- create and manage `NSWindow` and `NSView`;
- route keyboard, mouse, trackpad, controller, and window events;
- support menus, clipboard, file dialogs, drag and drop, and notifications;
- expose Keychain and sandbox-aware filesystem paths;
- handle app activation, reopening, and document events;
- create a Metal rendering layer.

## 12.3 Rendering

Metal is the primary backend. Vulkan may be supported through a portability layer only when it is operationally justified.

## 12.4 Packaging

- `.app` bundle
- code signing
- notarization
- optional Mac App Store packaging
- sandbox and entitlement configuration

---

# 13. Linux Platform Design

## 13.1 Toolchain

- GCC or Clang
- CMake
- Ninja or Make
- pkg-config
- platform libraries for Wayland, X11, audio, and input

## 13.2 Window-System Strategy

The Linux backend should support:

1. Wayland as the preferred modern path;
2. X11 as a compatibility path;
3. an optional abstraction library where it reduces maintenance burden.

## 13.3 Host Responsibilities

The Linux host shall:

- own the process entry point and event loop;
- create Wayland or X11 windows;
- translate keyboard, mouse, touch, and controller input;
- expose clipboard and drag-and-drop services;
- integrate file dialogs through portals where available;
- support desktop notifications;
- provide secure-storage integration where available;
- expose audio through the selected backend;
- create Vulkan or OpenGL rendering surfaces.

## 13.4 Rendering

Preferred backend order:

1. Vulkan
2. OpenGL fallback

## 13.5 Packaging

Potential formats:

- AppImage
- Flatpak
- Snap
- distribution packages
- tarball

The core framework must not depend on one packaging format.

---

# 14. Web Platform Design

## 14.1 Toolchain

- Emscripten
- Clang
- WebAssembly
- JavaScript or TypeScript host glue
- browser APIs
- WebGPU or WebGL

## 14.2 Host Responsibilities

The web host shall:

- load and instantiate the WebAssembly module;
- own the browser event-loop integration;
- route pointer, touch, keyboard, wheel, and gamepad input;
- manage canvas creation and resize behavior;
- bridge browser storage, clipboard, file selection, notifications, and sharing;
- handle page visibility changes;
- integrate asynchronous browser APIs;
- surface browser security and permission restrictions.

## 14.3 Execution Model

The framework must not block the browser main thread indefinitely. The runtime shall integrate with `requestAnimationFrame` or equivalent scheduling.

## 14.4 Rendering

Preferred backend order:

1. WebGPU
2. WebGL 2 fallback

## 14.5 Browser Constraints

The design must account for:

- sandboxed filesystem access;
- user-gesture requirements;
- asynchronous API restrictions;
- limited threading unless cross-origin isolation is configured;
- memory-growth limits;
- browser-specific codec availability;
- no direct operating-system API access.

---

# 15. Rendering Architecture

## 15.1 Renderer Interface

```cpp
class RendererBackend {
public:
    virtual ~RendererBackend() = default;

    virtual void initialize(const SurfaceDescriptor&) = 0;
    virtual void resize(uint32_t width, uint32_t height, float scale) = 0;
    virtual void beginFrame() = 0;
    virtual void submit(const RenderCommandList&) = 0;
    virtual void endFrame() = 0;
    virtual void suspendSurface() = 0;
    virtual void restoreSurface(const SurfaceDescriptor&) = 0;
    virtual void shutdown() = 0;
};
```

## 15.2 Backend Mapping

| Platform | Primary backend | Secondary backend |
|---|---|---|
| Android | Vulkan | OpenGL ES |
| iOS | Metal | None required initially |
| Windows | Direct3D 12 | Vulkan |
| macOS | Metal | Optional portability backend |
| Linux | Vulkan | OpenGL |
| Web | WebGPU | WebGL 2 |

## 15.3 Shader Strategy

The framework should define one canonical shader source or intermediate representation and generate backend-specific variants during the asset build.

The asset pipeline should produce:

- SPIR-V for Vulkan;
- Metal shader libraries or Metal Shading Language output;
- DXIL for Direct3D 12;
- WGSL for WebGPU;
- GLSL variants for OpenGL and WebGL.

Runtime shader compilation should be optional and disabled in production where platform policy or startup performance makes it undesirable.

## 15.4 Device Loss and Surface Loss

The renderer must distinguish:

- surface loss;
- swapchain recreation;
- window resize;
- background suspension;
- full GPU device loss.

Application state must survive surface recreation.

---

# 16. UI Architecture

## 16.1 Framework-Rendered UI

The shared UI system provides:

- retained widget tree;
- declarative layout;
- style resolution;
- text shaping;
- focus navigation;
- pointer and touch handling;
- accessibility semantics;
- animation;
- high-DPI scaling;
- localization and bidirectional text.

## 16.2 Native-Shell UI

Native host UI may render menus, platform-standard dialogs, navigation shells, or accessibility-heavy views while calling shared C++ services.

## 16.3 Hybrid Composition

A hybrid application may use:

- native navigation and system controls;
- a framework-rendered canvas for complex content;
- shared C++ state and domain logic;
- native text entry where appropriate.

## 16.4 UI State Ownership

The shared model owns durable application state. Native views should be projections of that state rather than independent sources of truth.

---

# 17. Input System

## 17.1 Portable Event Types

```cpp
struct PointerEvent;
struct TouchEvent;
struct KeyEvent;
struct TextInputEvent;
struct GamepadEvent;
struct WindowEvent;
struct SensorEvent;
```

## 17.2 Input Normalization

The platform layer translates native input into framework events while preserving:

- device identifier;
- timestamp;
- position;
- pressure;
- tilt;
- modifier state;
- repeat state;
- text-composition state;
- coordinate space.

## 17.3 Text Input

Text entry must be treated separately from key events. The framework must support:

- input method editors;
- composition ranges;
- candidate selection;
- mobile software keyboards;
- dead keys;
- Unicode text;
- accessibility-driven input.

---

# 18. Threading and Task Scheduling

## 18.1 Thread Roles

Potential thread roles include:

- platform main thread;
- render thread;
- worker pool;
- audio callback thread;
- networking threads owned by libraries;
- background I/O thread.

## 18.2 Main-Thread Rule

Native UI operations must execute on the platform UI thread.

The platform abstraction shall provide:

```cpp
void dispatchToMainThread(Task task);
```

## 18.3 Worker Scheduler

The shared scheduler should support:

- CPU task queues;
- priorities;
- cancellation;
- dependencies;
- continuations;
- platform suspension;
- graceful shutdown.

## 18.4 Web Threading

The web target must support a single-threaded mode. Multithreading may be enabled only where browser deployment headers and shared-memory requirements are satisfied.

---

# 19. Memory and ABI Rules

- Use RAII for native resources.
- Do not pass C++ standard-library containers across unstable binary boundaries.
- Do not allow exceptions to cross JNI, Objective-C, C, JavaScript, or plugin ABI boundaries.
- Prefer `std::span`, string views, value types, and explicit buffers inside one toolchain boundary.
- Use opaque C handles for long-lived cross-language objects.
- Version public plugin interfaces.
- Define allocator ownership for every returned buffer.
- Cancel callbacks before destroying their target object.
- Use weak references or generation counters for asynchronous callbacks.

---

# 20. Filesystem and Storage

## 20.1 Portable Paths

The framework exposes logical directories:

- application resources;
- writable application data;
- cache;
- temporary files;
- user-selected files;
- logs;
- secure storage.

## 20.2 Platform Behavior

The implementation must respect:

- Android scoped storage;
- iOS and macOS sandbox paths;
- Windows known folders and package rules;
- Linux XDG conventions;
- browser virtual storage and file-picker restrictions.

## 20.3 Database Layer

SQLite may be used as the default embedded database. The wrapper must handle:

- migrations;
- transactions;
- thread confinement or serialized access;
- backup;
- corruption reporting;
- encryption as a separate optional concern.

---

# 21. Networking

The shared networking layer should support:

- HTTP and HTTPS;
- WebSocket;
- cancellation;
- timeouts;
- TLS validation;
- proxy behavior where supported;
- background transfer adapters where available;
- offline detection;
- retry policy;
- certificate pinning as an optional feature.

Platform-specific network restrictions and lifecycle suspension must be surfaced explicitly.

---

# 22. Audio and Media

The framework should define portable interfaces for:

- audio playback;
- audio capture;
- stream mixing;
- device interruption;
- sample-rate changes;
- media decoding;
- camera preview and capture.

Potential platform implementations include:

- Android audio APIs;
- AVAudioEngine or lower-level Apple audio APIs;
- WASAPI on Windows;
- Core Audio on macOS;
- PipeWire or ALSA on Linux;
- Web Audio on the browser.

Media codecs must be capability-driven because codec availability differs by platform.

---

# 23. Security

## 23.1 Secure Storage

Platform implementations should use native secure-storage facilities, including:

- Android Keystore;
- Apple Keychain;
- Windows credential or data-protection facilities;
- Linux secret-service integration where available;
- browser credential or encrypted application storage strategies where appropriate.

## 23.2 Sensitive Data Rules

- Never log secrets or tokens.
- Zero sensitive buffers when practical.
- Keep authentication tokens out of ordinary preferences storage.
- Validate all external input.
- Treat deep links and file imports as untrusted.
- Enforce TLS certificate validation.
- Use platform code-signing and integrity features.

## 23.3 Supply Chain

Every dependency must be recorded with:

- source;
- version;
- license;
- checksum or commit;
- update policy;
- security-review owner.

---

# 24. Accessibility

The framework-rendered UI must expose a semantic tree containing:

- role;
- label;
- value;
- state;
- bounds;
- actions;
- focus order;
- live-region behavior.

Each platform adapter maps this tree to the native accessibility system.

Accessibility is not optional for framework widgets. Custom-drawn controls without semantics are considered incomplete.

---

# 25. Localization

The framework shall support:

- UTF-8 internal strings;
- locale-aware formatting;
- pluralization;
- right-to-left layout;
- localized resource bundles;
- fallback languages;
- platform locale detection;
- localized app metadata handled by packaging layers.

---

# 26. Build System

## 26.1 CMake as Shared Build Description

CMake defines:

- portable libraries;
- renderer modules;
- platform adapters;
- tests;
- compiler features;
- generated configuration;
- dependency wiring.

## 26.2 Native Packaging Layers

CMake does not replace platform packaging:

- Android uses Gradle plus CMake/NDK integration.
- iOS uses Xcode projects or generated Xcode targets.
- Windows uses Visual Studio, Ninja, MSIX, or installer tooling.
- macOS uses Xcode, bundles, signing, and notarization.
- Linux uses distribution-specific packaging workflows.
- Web uses Emscripten output plus web asset packaging.

## 26.3 Presets

Recommended presets:

```text
android-arm64-debug
android-arm64-release
ios-device-release
ios-simulator-debug
windows-x64-debug
windows-x64-release
macos-arm64-debug
macos-universal-release
linux-x64-debug
linux-x64-release
web-debug
web-release
```

## 26.4 Compiler Policy

The codebase should compile with warnings treated as errors in CI, subject to reviewed platform exceptions.

Supported compilers:

- Clang;
- Apple Clang;
- MSVC;
- GCC;
- Emscripten Clang.

---

# 27. Dependency Policy

Dependencies must:

- support all required target platforms or be isolated behind a replaceable adapter;
- use compatible licenses;
- avoid hidden runtime downloads;
- be pinned to exact versions or commits;
- be buildable in CI;
- have documented security and update procedures.

Platform-only dependencies must not leak into shared public APIs.

---

# 28. Error Handling

## 28.1 Shared Core

The core may use:

- result types;
- error codes;
- exceptions internally where policy permits;
- structured diagnostic objects.

## 28.2 Boundaries

No exception may cross:

- JNI;
- Objective-C or Swift bridge;
- Win32 callbacks;
- C plugin ABI;
- JavaScript or WebAssembly boundary.

Boundary functions must translate failures into explicit result values.

## 28.3 Crash Reporting

Crash reporting should capture:

- application version;
- framework version;
- platform and device;
- renderer backend;
- lifecycle state;
- recent structured logs;
- stack trace where permitted;
- active feature flags.

---

# 29. Logging and Diagnostics

The logging system shall support:

- severity levels;
- categories;
- structured fields;
- platform log sinks;
- file rotation where supported;
- privacy filtering;
- development console output;
- crash-ring buffer.

Platform sinks include Android logcat, Apple unified logging, Windows debugging/event sinks, Linux journaling or stderr, and browser console output.

---

# 30. Testing Strategy

## 30.1 Unit Tests

Shared code should run on desktop CI without mobile emulators whenever possible.

Test targets include:

- layout;
- serialization;
- navigation;
- domain logic;
- resource resolution;
- task scheduling;
- error handling;
- platform-independent storage logic.

## 30.2 Platform Contract Tests

Every platform adapter must pass the same conformance suite for:

- lifecycle ordering;
- file access semantics;
- clipboard behavior;
- task dispatch;
- capability reporting;
- surface recreation;
- input translation;
- secure-storage contract.

## 30.3 Integration Tests

Integration tests should cover:

- startup and shutdown;
- background and foreground transitions;
- window resizing;
- rotation;
- GPU surface loss;
- network interruption;
- permission denial;
- file selection;
- deep links;
- notification routing.

## 30.4 Visual Tests

Framework-rendered UI should support golden-image tests with tolerances for backend differences.

## 30.5 Device Matrix

CI and release testing should include representative:

- Android API levels and GPU vendors;
- current and previous iOS versions;
- Windows versions and GPU vendors;
- Intel and Apple Silicon macOS devices;
- major Linux desktop environments;
- Chromium, Firefox, and Safari-class browsers where supported.

---

# 31. Continuous Integration

CI stages:

1. formatting and static analysis;
2. dependency validation;
3. shared unit tests;
4. desktop builds;
5. Android builds and emulator tests;
6. iOS simulator builds and tests;
7. macOS tests;
8. Linux tests;
9. Windows tests;
10. WebAssembly build and browser tests;
11. package validation;
12. artifact publication.

Recommended analysis tools include compiler warnings, sanitizers, static analyzers, and dependency scanners.

---

# 32. Packaging and Distribution

## 32.1 Android

- Android App Bundle
- APK for local testing
- manifest and permission generation
- signing configuration
- ABI splits where appropriate

## 32.2 iOS

- `.app` and archive generation
- signing and provisioning
- entitlements
- privacy declarations
- App Store submission package

## 32.3 Windows

- executable and dependent libraries
- MSIX or installer
- code signing
- optional Microsoft Store package

## 32.4 macOS

- `.app` bundle
- signing
- notarization
- entitlements
- optional Mac App Store package

## 32.5 Linux

- AppImage, Flatpak, Snap, native packages, or tarball
- desktop entry
- icons
- MIME associations

## 32.6 Web

- `.wasm` module
- JavaScript loader
- HTML shell
- assets
- service-worker support where selected
- deployment headers for threaded builds

---

# 33. API Versioning

The framework shall use semantic versioning for public releases.

Public interfaces must distinguish:

- source compatibility;
- binary compatibility;
- asset-format compatibility;
- serialized-data compatibility;
- plugin ABI compatibility.

Breaking changes require migration notes.

---

# 34. Plugin Architecture

Plugins may be:

- statically linked;
- dynamically linked on platforms that permit it;
- compiled into the WebAssembly bundle;
- platform-specific service adapters.

A stable C ABI is recommended for long-lived binary plugins.

Plugin descriptors should include:

- name;
- version;
- required framework version;
- capabilities;
- dependencies;
- initialization and shutdown hooks.

---

# 35. Asset Pipeline

The asset pipeline should:

- compile shaders;
- process textures;
- generate mipmaps;
- compress audio;
- package fonts;
- validate localization;
- fingerprint files;
- produce per-platform bundles;
- generate asset manifests;
- support incremental rebuilds.

Runtime assets should be referenced using logical identifiers rather than raw operating-system paths.

---

# 36. Performance Requirements

The framework should target:

- no per-frame JNI or Swift bridge chatter for ordinary rendering;
- bounded allocations in frame-critical paths;
- configurable frame pacing;
- lazy asset loading;
- asynchronous I/O;
- background decompression;
- zero-copy transfer where practical;
- renderer resource pooling;
- startup telemetry;
- memory-pressure handling.

Performance budgets should be defined per application profile rather than globally fixed.

---

# 37. Observability

The runtime should expose metrics for:

- startup duration;
- frame time;
- dropped frames;
- CPU usage;
- GPU timing where available;
- memory usage;
- asset load time;
- network latency;
- task queue depth;
- cache hit rate;
- crash-free sessions.

Telemetry must be optional, privacy-aware, and configurable.

---

# 38. Development Workflow

A typical workflow is:

1. develop and unit-test shared C++ on desktop;
2. run the shared application on Windows, macOS, or Linux;
3. validate Android and iOS platform integrations;
4. validate browser constraints on WebAssembly;
5. run platform conformance tests;
6. build signed release packages;
7. execute device-matrix acceptance tests.

The framework should provide sample applications demonstrating:

- custom-rendered UI;
- native-shell UI;
- hybrid UI;
- file picking;
- notifications;
- camera access;
- secure storage;
- Vulkan, Metal, Direct3D, and WebGPU rendering.

---

# 39. Implementation Phases

## Phase 1: Portable Core

- runtime;
- lifecycle;
- event queue;
- logging;
- filesystem abstraction;
- basic task scheduler;
- CMake structure;
- desktop test harness.

## Phase 2: Desktop Foundations

- Windows host;
- macOS host;
- Linux host;
- native windows;
- input translation;
- initial graphics backends.

## Phase 3: Mobile Hosts

- Android host and JNI bridge;
- iOS host and Objective-C++ bridge;
- touch input;
- mobile lifecycle;
- permissions;
- app packaging.

## Phase 4: Web Target

- Emscripten build;
- browser event loop;
- WebGPU or WebGL renderer;
- browser storage and input;
- deployment templates.

## Phase 5: Shared UI

- widget tree;
- layout;
- text;
- styling;
- accessibility semantics;
- animation;
- localization.

## Phase 6: Native Services

- notifications;
- camera;
- location;
- sharing;
- secure storage;
- deep links;
- biometrics;
- store services.

## Phase 7: Production Hardening

- conformance suite;
- crash reporting;
- performance telemetry;
- packaging automation;
- security review;
- release process;
- documentation stabilization.

---

# 40. Major Risks and Mitigations

## 40.1 Platform Abstraction Becomes Too Broad

**Risk:** The abstraction attempts to hide every native difference.  
**Mitigation:** Model common capabilities and permit explicit platform extensions.

## 40.2 Language-Boundary Complexity

**Risk:** JNI, Swift, Objective-C++, and JavaScript bridges become fragile.  
**Mitigation:** Keep boundaries coarse-grained, versioned, and heavily tested.

## 40.3 Rendering Backend Divergence

**Risk:** Backend behavior drifts.  
**Mitigation:** Use one renderer contract, shared validation, shader tooling, and conformance tests.

## 40.4 Lifecycle Bugs

**Risk:** Mobile suspension and surface recreation corrupt state.  
**Mitigation:** Separate durable application state from window and GPU surface state.

## 40.5 Accessibility Gaps

**Risk:** Custom UI lacks native semantics.  
**Mitigation:** Make semantic metadata mandatory in widget APIs and test it per platform.

## 40.6 Dependency Portability

**Risk:** A dependency blocks one target.  
**Mitigation:** Require platform support or isolate it behind a replaceable adapter.

## 40.7 Web Constraints

**Risk:** Native assumptions fail in browsers.  
**Mitigation:** Maintain a first-class asynchronous, sandbox-aware web platform contract.

---

# 41. Acceptance Criteria

The initial framework release is accepted when:

- one nontrivial sample application compiles and runs on all six target platforms;
- shared application logic is unchanged across targets;
- each platform passes lifecycle conformance tests;
- each target can render through its designated backend;
- input works for the platform's principal device types;
- file storage, networking, logging, and task scheduling pass conformance tests;
- signed or distributable packages can be produced for every platform;
- the framework supports both custom-rendered and native-shell examples;
- no shared public header exposes native platform SDK types;
- automated builds exist for every target.

---

# 42. Architectural Decision Summary

| Area | Decision |
|---|---|
| Shared language | Modern C++ |
| Android bridge | Kotlin or Java plus JNI |
| iOS bridge | Swift or UIKit plus Objective-C++ |
| Windows host | Win32 or WinUI adapter |
| macOS host | AppKit or Swift host plus Objective-C++ |
| Linux host | Wayland first, X11 compatibility |
| Web host | Emscripten plus JavaScript browser glue |
| Build orchestration | CMake plus native packaging systems |
| Android graphics | Vulkan, OpenGL ES fallback |
| iOS graphics | Metal |
| Windows graphics | Direct3D 12, Vulkan optional |
| macOS graphics | Metal |
| Linux graphics | Vulkan, OpenGL fallback |
| Web graphics | WebGPU, WebGL 2 fallback |
| Cross-boundary errors | Explicit result values |
| Cross-boundary ownership | Opaque handles and documented lifetime |
| UI | Shared, native, or hybrid |
| Testing | Shared unit tests plus platform conformance suites |

---

# 43. Conclusion

A C++ application does not need to be rewritten in Swift to run on iOS, nor rewritten in the native language of every other target platform. The practical requirement is a platform host that integrates the shared C++ runtime with each operating system's lifecycle, SDKs, rendering surfaces, security model, and packaging system.

By treating Android, iOS, Windows, macOS, Linux, and Web as first-class targets from the beginning, the framework can avoid desktop-first or mobile-first assumptions becoming permanent architectural limitations. The portable core remains the source of application behavior, while each platform adapter remains narrow, testable, and replaceable.
