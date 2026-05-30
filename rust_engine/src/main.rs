mod audio;
mod poem_generated;
mod renderer;

use audio::AudioEngine;
use poem_generated::poem;
use renderer::Renderer;

use std::collections::HashMap;
use std::io::{Read, Write};
use std::sync::{Arc, Mutex, mpsc};
use std::thread;
use std::time::Duration;

use winit::{
    event::{ElementState, Event, MouseButton, WindowEvent},
    event_loop::{ControlFlow, EventLoop},
    window::{WindowBuilder, CursorIcon},
};

const INVALID_HANDLE_VALUE: isize = -1;

struct PipeConn {
    handle: windows_sys::Win32::Foundation::HANDLE,
}

impl Read for PipeConn {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        use windows_sys::Win32::Storage::FileSystem::ReadFile;
        let mut read = 0;
        let ok = unsafe {
            ReadFile(
                self.handle,
                buf.as_mut_ptr() as *mut _,
                buf.len() as u32,
                &mut read,
                std::ptr::null_mut(),
            )
        };
        if ok == 0 {
            let err = std::io::Error::last_os_error();
            if err.raw_os_error() == Some(109) { // ERROR_BROKEN_PIPE
                return Ok(0);
            }
            return Err(err);
        }
        Ok(read as usize)
    }
}

impl Write for PipeConn {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        use windows_sys::Win32::Storage::FileSystem::WriteFile;
        let mut written = 0;
        let ok = unsafe {
            WriteFile(
                self.handle,
                buf.as_ptr() as *const _,
                buf.len() as u32,
                &mut written,
                std::ptr::null_mut(),
            )
        };
        if ok == 0 {
            return Err(std::io::Error::last_os_error());
        }
        Ok(written as usize)
    }

    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}

// FlatBuffer Event structure to queue internally before batch transmission
#[derive(Clone, Debug)]
struct PendingEvent {
    type_: poem::EventType,
    x: i32,
    y: i32,
    button: i32,
    delta: i32,
    keycode: u32,
    char: u32,
    width: i32,
    height: i32,
}

enum GoCommand {
    RenderFrame(Vec<u8>),
    PlaySound(poem::SoundType),
}

fn main() {
    env_logger::init();
    println!("🔌 Rust presentation sidecar engine starting up...");

    // 1. Connect to both Windows Named Pipes
    let pipe_name_read: Vec<u16> = "\\\\.\\pipe\\poem_ipc_go_to_rust".encode_utf16().chain(Some(0)).collect();
    let pipe_name_write: Vec<u16> = "\\\\.\\pipe\\poem_ipc_rust_to_go".encode_utf16().chain(Some(0)).collect();

    let mut read_handle = INVALID_HANDLE_VALUE as windows_sys::Win32::Foundation::HANDLE;
    let mut write_handle = INVALID_HANDLE_VALUE as windows_sys::Win32::Foundation::HANDLE;

    // Connect to read pipe
    for _attempt in 1..=20 {
        use windows_sys::Win32::Storage::FileSystem::{
            CreateFileW, FILE_GENERIC_READ, FILE_GENERIC_WRITE, OPEN_EXISTING
        };
        let h = unsafe {
            CreateFileW(
                pipe_name_read.as_ptr(),
                FILE_GENERIC_READ | FILE_GENERIC_WRITE,
                0,
                std::ptr::null(),
                OPEN_EXISTING,
                0,
                0,
            )
        };
        if h != INVALID_HANDLE_VALUE as windows_sys::Win32::Foundation::HANDLE {
            read_handle = h;
            break;
        }
        thread::sleep(Duration::from_millis(100));
    }

    // Connect to write pipe
    for _attempt in 1..=20 {
        use windows_sys::Win32::Storage::FileSystem::{
            CreateFileW, FILE_GENERIC_READ, FILE_GENERIC_WRITE, OPEN_EXISTING
        };
        let h = unsafe {
            CreateFileW(
                pipe_name_write.as_ptr(),
                FILE_GENERIC_READ | FILE_GENERIC_WRITE,
                0,
                std::ptr::null(),
                OPEN_EXISTING,
                0,
                0,
            )
        };
        if h != INVALID_HANDLE_VALUE as windows_sys::Win32::Foundation::HANDLE {
            write_handle = h;
            break;
        }
        thread::sleep(Duration::from_millis(100));
    }

    if read_handle == INVALID_HANDLE_VALUE as windows_sys::Win32::Foundation::HANDLE 
        || write_handle == INVALID_HANDLE_VALUE as windows_sys::Win32::Foundation::HANDLE
    {
        panic!("❌ Failed to connect to Go Named Pipes after multiple attempts.");
    }
    println!("🤝 Bound to Go Named Pipes successfully!");

    let mut pipe_reader = PipeConn { handle: read_handle };
    let pipe_writer = Arc::new(Mutex::new(PipeConn { handle: write_handle }));

    // 2. Read bootstrap InitEngine FlatBuffer package
    let init_payload = match read_message(&mut pipe_reader) {
        Ok(p) => p,
        Err(e) => panic!("❌ Failed to read InitEngine packet from Go orchestrator: {:?}", e),
    };

    let envelope = poem::root_as_go_to_rust_message(&init_payload).unwrap();
    if envelope.message_type() != poem::GoToRustUnion::InitEngine {
        panic!("❌ Violating handshaking logic. Received unexpected envelope: {:?}", envelope.message_type());
    }

    let init = envelope.message_as_init_engine().unwrap();
    let win_w = init.width() as u32;
    let win_h = init.height() as u32;
    let atlas_w = init.atlas_width() as u32;
    let atlas_h = init.atlas_height() as u32;
    let atlas_pixels = init.atlas_pixels().unwrap().bytes();

    // Map atlas CharInfo into a HashMap for fast O(1) text quad queries
    let mut char_map = HashMap::new();
    let chars = init.chars().unwrap();
    for i in 0..chars.len() {
        let char_info = chars.get(i);
        char_map.insert(char_info.r() as u32, char_info);
    }
    println!("🔤 Font Atlas glyph mapping synchronized: {} characters loaded.", char_map.len());

    let args: Vec<String> = std::env::args().collect();
    let window_title = if args.len() > 1 && !args[1].is_empty() {
        args[1].clone()
    } else {
        "P.O.E.M. Operational Engine Matrix (Dual-Process wgpu)".to_string()
    };

    // 3. Spawns winit Event Loop
    let event_loop = EventLoop::new().unwrap();
    let window = WindowBuilder::new()
        .with_title(window_title)
        .with_inner_size(winit::dpi::LogicalSize::new(win_w, win_h))
        .build(&event_loop)
        .unwrap();

    let window = Arc::new(window);

    // 4. Initialize rodio audio engine
    let (audio_engine, _audio_stream) = if let Ok((stream, handle)) = rodio::OutputStream::try_default() {
        println!("🔊 Portable Acoustic Audio Device discovered and initialized.");
        (AudioEngine::new(Some(handle)), Some(stream))
    } else {
        println!("⚠️ No active audio device detected. Sound feedback muted.");
        (AudioEngine::new(None), None)
    };
    let audio_engine = Arc::new(audio_engine);

    // 5. Initialize wgpu Context
    let instance = wgpu::Instance::default();
    let surface = instance.create_surface(window.clone()).unwrap();
    let adapter = pollster::block_on(instance.request_adapter(&wgpu::RequestAdapterOptions {
        power_preference: wgpu::PowerPreference::HighPerformance,
        compatible_surface: Some(&surface),
        force_fallback_adapter: false,
    }))
    .expect("❌ Failed to request high performance GPU adapter.");

    let (device, queue) = pollster::block_on(adapter.request_device(
        &wgpu::DeviceDescriptor {
            label: Some("Sidecar Device"),
            required_features: wgpu::Features::empty(),
            required_limits: wgpu::Limits::default(),
        },
        None,
    ))
    .expect("❌ Failed to bind WebGPU device context.");

    let device = Arc::new(device);
    let queue = Arc::new(queue);

    let caps = surface.get_capabilities(&adapter);
    let format = caps.formats[0];

    let physical_size = window.inner_size();
    let scale_factor = window.scale_factor() as f32;

    let surface_config = wgpu::SurfaceConfiguration {
        usage: wgpu::TextureUsages::RENDER_ATTACHMENT,
        format,
        width: physical_size.width,
        height: physical_size.height,
        present_mode: wgpu::PresentMode::Fifo,
        alpha_mode: caps.alpha_modes[0],
        view_formats: vec![],
        desired_maximum_frame_latency: 2,
    };
    surface.configure(&device, &surface_config);

    // Create GPU Renderer
    let mut renderer = Renderer::new(
        device.clone(),
        queue.clone(),
        surface_config,
        atlas_w,
        atlas_h,
        atlas_pixels,
    );

    // Explicitly configure projection bounds and textures with scale factor on startup
    renderer.resize(physical_size.width, physical_size.height, scale_factor);

    let renderer = Arc::new(Mutex::new(renderer));

    // 6. Spawn Background Named Pipe Reader thread (avoids winit event blocking)
    let (cmd_tx, cmd_rx) = mpsc::channel();
    let mut pipe_reader = pipe_reader;
    thread::spawn(move || {
        loop {
            match read_message(&mut pipe_reader) {
                Ok(payload) => {
                    let env = poem::root_as_go_to_rust_message(&payload).unwrap();
                    match env.message_type() {
                        poem::GoToRustUnion::RenderFrame => {
                            let _ = cmd_tx.send(GoCommand::RenderFrame(payload));
                        }
                        poem::GoToRustUnion::PlaySound => {
                            let sound = env.message_as_play_sound().unwrap();
                            let _ = cmd_tx.send(GoCommand::PlaySound(sound.type_()));
                        }
                        _ => {}
                    }
                }
                Err(e) => {
                    println!("🔌 Go orchestrator pipe severed: {:?}", e);
                    break;
                }
            }
        }
    });

    // 7. winit Cross-platform Event loop execution
    let mut mouse_x = 0;
    let mut mouse_y = 0;
    let mut is_ctrl_pressed = false;
    let mut is_shift_pressed = false;

    let _ = event_loop.run(move |event, window_target| {
        window_target.set_control_flow(ControlFlow::Poll);

        match event {
            Event::AboutToWait => {
                // Poll for incoming paint commands or sound events from Go
                let mut latest_frame_payload: Option<Vec<u8>> = None;

                while let Ok(cmd) = cmd_rx.try_recv() {
                    match cmd {
                        GoCommand::PlaySound(sound_type) => {
                            match sound_type {
                                poem::SoundType::Hover => audio_engine.play_hover(),
                                poem::SoundType::Click => audio_engine.play_click(),
                                poem::SoundType::Success => audio_engine.play_success(),
                                _ => {}
                            }
                        }
                        GoCommand::RenderFrame(payload) => {
                            latest_frame_payload = Some(payload);
                        }
                    }
                }

                if let Some(payload) = latest_frame_payload {
                    let env = poem::root_as_go_to_rust_message(&payload).unwrap();
                    let frame = env.message_as_render_frame().unwrap();
                    let cmd_len = frame.commands().unwrap().len();
                    println!("🎨 [DIAGNOSTIC] Rust received RenderFrame with {} commands!", cmd_len);

                    // Update active cursor icon
                    let cursor_type = frame.cursor();
                    let icon = match cursor_type {
                        1 => CursorIcon::Pointer,
                        2 => CursorIcon::Text,
                        _ => CursorIcon::Default,
                    };
                    window.set_cursor_icon(icon);

                    // Load vertices and flush draw commands into WebGPU pipeline
                    let mut ren = renderer.lock().unwrap();
                    let commands = frame.commands().unwrap();
                    for i in 0..commands.len() {
                        let cmd = commands.get(i);
                        if i < 10 {
                            println!("🦀 [RUST DIAGNOSTIC] Command {}: Type={:?}, Rect=({},{},{},{}), Color=({},{},{},{})", 
                                i, cmd.type_(), cmd.x1(), cmd.y1(), cmd.x2(), cmd.y2(), cmd.r(), cmd.g(), cmd.b(), cmd.a());
                        }
                        let col = [
                            cmd.r() as f32 / 255.0,
                            cmd.g() as f32 / 255.0,
                            cmd.b() as f32 / 255.0,
                            cmd.a() as f32 / 255.0,
                        ];
                        match cmd.type_() {
                            poem::DrawCommandType::DrawRoundedRect => {
                                let r_val = cmd.radius();
                                if r_val == -999 {
                                    ren.draw_raycaster(
                                        cmd.x1() as f32,
                                        cmd.y1() as f32,
                                        cmd.x2() as f32,
                                        cmd.y2() as f32,
                                        cmd.val1(),
                                        cmd.val2(),
                                        cmd.val3(),
                                        col,
                                    );
                                } else if r_val == -998 || r_val == -997 {
                                    ren.draw_billboard(
                                        cmd.x1() as f32,
                                        cmd.y1() as f32,
                                        cmd.x2() as f32,
                                        cmd.y2() as f32,
                                        r_val as f32,
                                        cmd.val1(),
                                        cmd.val2(),
                                        col,
                                    );
                                } else {
                                    ren.draw_rounded_rect(
                                        cmd.x1() as f32,
                                        cmd.y1() as f32,
                                        cmd.x2() as f32,
                                        cmd.y2() as f32,
                                        r_val as f32,
                                        col,
                                    );
                                }
                            }
                            poem::DrawCommandType::FillRect => {
                                ren.fill_rect(
                                    cmd.x1() as f32,
                                    cmd.y1() as f32,
                                    cmd.x2() as f32,
                                    cmd.y2() as f32,
                                    col,
                                );
                            }
                            poem::DrawCommandType::DrawLine => {
                                ren.draw_line(
                                    cmd.x1() as f32,
                                    cmd.y1() as f32,
                                    cmd.x2() as f32,
                                    cmd.y2() as f32,
                                    col,
                                );
                            }
                            poem::DrawCommandType::DrawText => {
                                if let Some(text) = cmd.text() {
                                    let mut x = cmd.x1() as f32;
                                    let y = cmd.y1() as f32;
                                    for c in text.chars() {
                                        if let Some(info) = char_map.get(&(c as u32)) {
                                            let x1 = x;
                                            let y1 = y;
                                            let x2 = x + info.width() as f32;
                                            let y2 = y + info.height() as f32;
                                            let uv = [info.u1(), info.v1(), info.u2(), info.v2()];
                                            ren.draw_text_char(x1, y1, x2, y2, uv, col);
                                            x += info.advance() as f32;
                                        }
                                    }
                                }
                            }
                            poem::DrawCommandType::SetGlow => {
                                ren.set_glow(cmd.val1());
                            }
                            poem::DrawCommandType::SetGlass => {
                                ren.set_glass(cmd.flag());
                            }
                            poem::DrawCommandType::SetShadow => {
                                ren.set_shadow(cmd.val1(), cmd.val2(), cmd.val3());
                            }
                            poem::DrawCommandType::SetOffset => {
                                ren.set_offset(cmd.val1(), cmd.val2());
                            }
                            poem::DrawCommandType::SetClip => {
                                ren.set_clip(
                                    cmd.x1() as f32,
                                    cmd.y1() as f32,
                                    cmd.w() as f32,
                                    cmd.h() as f32,
                                    cmd.flag(),
                                );
                            }
                            _ => {}
                        }
                    }

                    // Trigger GPU paint Pass
                    if let Ok(frame) = surface.get_current_texture() {
                        let view = frame.texture.create_view(&wgpu::TextureViewDescriptor::default());
                        ren.render(&view);
                        frame.present();
                    }
                }
            }

            Event::WindowEvent { event, .. } => match event {
                WindowEvent::CloseRequested => {
                    // Send Close event to Go and exit gracefully
                    let ev = PendingEvent {
                        type_: poem::EventType::WindowClose,
                        x: 0,
                        y: 0,
                        button: 0,
                        delta: 0,
                        keycode: 0,
                        char: 0,
                        width: 0,
                        height: 0,
                    };
                    send_event_batch(&pipe_writer, vec![ev]);
                    window_target.exit();
                }

                WindowEvent::ScaleFactorChanged { scale_factor, .. } => {
                    let physical_size = window.inner_size();
                    let sf = scale_factor as f32;
                    let mut ren = renderer.lock().unwrap();
                    ren.resize(physical_size.width, physical_size.height, sf);
                    surface.configure(&device, &ren.surface_config);

                    let logical_w = (physical_size.width as f64 / scale_factor) as i32;
                    let logical_h = (physical_size.height as f64 / scale_factor) as i32;
                    let ev = PendingEvent {
                        type_: poem::EventType::WindowSize,
                        x: 0,
                        y: 0,
                        button: 0,
                        delta: 0,
                        keycode: 0,
                        char: 0,
                        width: logical_w,
                        height: logical_h,
                    };
                    send_event_batch(&pipe_writer, vec![ev]);
                }

                WindowEvent::Resized(physical_size) => {
                    let scale_factor = window.scale_factor();
                    let mut ren = renderer.lock().unwrap();
                    ren.resize(physical_size.width, physical_size.height, scale_factor as f32);
                    surface.configure(&device, &ren.surface_config);

                    let logical_w = (physical_size.width as f64 / scale_factor) as i32;
                    let logical_h = (physical_size.height as f64 / scale_factor) as i32;
                    let ev = PendingEvent {
                        type_: poem::EventType::WindowSize,
                        x: 0,
                        y: 0,
                        button: 0,
                        delta: 0,
                        keycode: 0,
                        char: 0,
                        width: logical_w,
                        height: logical_h,
                    };
                    send_event_batch(&pipe_writer, vec![ev]);
                }

                WindowEvent::CursorMoved { position, .. } => {
                    let sf = window.scale_factor();
                    mouse_x = (position.x / sf) as i32;
                    mouse_y = (position.y / sf) as i32;

                    let ev = PendingEvent {
                        type_: poem::EventType::MouseMove,
                        x: mouse_x,
                        y: mouse_y,
                        button: 0,
                        delta: 0,
                        keycode: 0,
                        char: 0,
                        width: 0,
                        height: 0,
                    };
                    send_event_batch(&pipe_writer, vec![ev]);
                }

                WindowEvent::MouseInput { state, button, .. } => {
                    let btn_id = match button {
                        MouseButton::Left => 1,
                        MouseButton::Right => 2,
                        MouseButton::Middle => 3,
                        _ => 0,
                    };
                    let type_ = if state == ElementState::Pressed {
                        poem::EventType::MouseDown
                    } else {
                        poem::EventType::MouseUp
                    };

                    let ev = PendingEvent {
                        type_,
                        x: mouse_x,
                        y: mouse_y,
                        button: btn_id,
                        delta: 0,
                        keycode: 0,
                        char: 0,
                        width: 0,
                        height: 0,
                    };
                    send_event_batch(&pipe_writer, vec![ev]);
                }

                WindowEvent::MouseWheel { delta, .. } => {
                    let sf = window.scale_factor();
                    let scroll_delta = match delta {
                        winit::event::MouseScrollDelta::LineDelta(_, y) => (y * 120.0) as i32,
                        winit::event::MouseScrollDelta::PixelDelta(pos) => (pos.y / sf) as i32,
                    };

                    let ev = PendingEvent {
                        type_: poem::EventType::MouseWheel,
                        x: mouse_x,
                        y: mouse_y,
                        button: 0,
                        delta: scroll_delta,
                        keycode: 0,
                        char: 0,
                        width: 0,
                        height: 0,
                    };
                    send_event_batch(&pipe_writer, vec![ev]);
                }

                WindowEvent::KeyboardInput { event: kb_event, .. } => {
                    // Check modifiers
                    if kb_event.physical_key == winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ControlLeft)
                        || kb_event.physical_key == winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ControlRight)
                    {
                        is_ctrl_pressed = kb_event.state == ElementState::Pressed;
                    }
                    if kb_event.physical_key == winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ShiftLeft)
                        || kb_event.physical_key == winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ShiftRight)
                    {
                        is_shift_pressed = kb_event.state == ElementState::Pressed;
                    }

                    // Extract key code (match GDI vkCode mappings)
                    let vk_code = match kb_event.physical_key {
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Escape) => 0x1B,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Tab) => 0x09,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Backspace) => 0x08,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Enter) => 0x0D,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Space) => 0x20,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ArrowLeft) => 0x25,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ArrowUp) => 0x26,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ArrowRight) => 0x27,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::ArrowDown) => 0x28,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyA) => 'A' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyB) => 'B' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyC) => 'C' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyD) => 'D' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyE) => 'E' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyF) => 'F' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyG) => 'G' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyH) => 'H' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyI) => 'I' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyJ) => 'J' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyK) => 'K' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyL) => 'L' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyM) => 'M' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyN) => 'N' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyO) => 'O' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyP) => 'P' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyQ) => 'Q' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyR) => 'R' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyS) => 'S' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyT) => 'T' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyU) => 'U' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyV) => 'V' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyW) => 'W' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyX) => 'X' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyY) => 'Y' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::KeyZ) => 'Z' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit0) => '0' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit1) => '1' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit2) => '2' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit3) => '3' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit4) => '4' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit5) => '5' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit6) => '6' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit7) => '7' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit8) => '8' as u32,
                        winit::keyboard::PhysicalKey::Code(winit::keyboard::KeyCode::Digit9) => '9' as u32,
                        _ => 0,
                    };

                    if kb_event.state == ElementState::Pressed {
                        if vk_code != 0 {
                            let mut modifier_mask = 0;
                            if is_ctrl_pressed { modifier_mask |= 1; }
                            if is_shift_pressed { modifier_mask |= 2; }

                            let ev = PendingEvent {
                                type_: poem::EventType::KeyDown,
                                x: 0,
                                y: 0,
                                button: modifier_mask,
                                delta: 0,
                                keycode: vk_code,
                                char: 0,
                                width: 0,
                                height: 0,
                            };
                            send_event_batch(&pipe_writer, vec![ev]);
                        }

                        if let Some(text) = &kb_event.text {
                            for c in text.chars() {
                                if !c.is_control() {
                                    let ev = PendingEvent {
                                        type_: poem::EventType::KeyChar,
                                        x: 0,
                                        y: 0,
                                        button: 0,
                                        delta: 0,
                                        keycode: 0,
                                        char: c as u32,
                                        width: 0,
                                        height: 0,
                                    };
                                    send_event_batch(&pipe_writer, vec![ev]);
                                }
                            }
                        }
                    } else {
                        // Released
                        if vk_code != 0 {
                            let mut modifier_mask = 0;
                            if is_ctrl_pressed { modifier_mask |= 1; }
                            if is_shift_pressed { modifier_mask |= 2; }

                            let ev = PendingEvent {
                                type_: poem::EventType::KeyUp,
                                x: 0,
                                y: 0,
                                button: modifier_mask,
                                delta: 0,
                                keycode: vk_code,
                                char: 0,
                                width: 0,
                                height: 0,
                            };
                            send_event_batch(&pipe_writer, vec![ev]);
                        }
                    }
                }

                _ => {}
            },

            _ => {}
        }
    });
}



// Named Pipe Message read helper
fn read_message(conn: &mut PipeConn) -> std::io::Result<Vec<u8>> {
    let mut len_buf = [0u8; 4];
    conn.read_exact(&mut len_buf)?;
    let length = u32::from_le_bytes(len_buf) as usize;
    let mut payload = vec![0u8; length];
    conn.read_exact(&mut payload)?;
    Ok(payload)
}

// Transmit flat-serialized Event batches back to Go
fn send_event_batch(pipe_writer: &Arc<Mutex<PipeConn>>, events: Vec<PendingEvent>) {
    let mut builder = flatbuffers::FlatBufferBuilder::new();

    let mut offsets = Vec::new();
    for ev in events {
        let ev_offset = poem::Event::create(
            &mut builder,
            &poem::EventArgs {
                type_: ev.type_,
                x: ev.x,
                y: ev.y,
                button: ev.button,
                delta: ev.delta,
                keycode: ev.keycode,
                char: ev.char,
                width: ev.width,
                height: ev.height,
            },
        );
        offsets.push(ev_offset);
    }

    let vec_offset = builder.create_vector(&offsets);
    let batch_offset = poem::EventBatch::create(
        &mut builder,
        &poem::EventBatchArgs {
            events: Some(vec_offset),
        },
    );

    let msg_offset = poem::RustToGoMessage::create(
        &mut builder,
        &poem::RustToGoMessageArgs {
            message_type: poem::RustToGoUnion::EventBatch,
            message: Some(batch_offset.as_union_value()),
        },
    );

    builder.finish(msg_offset, None);
    let data = builder.finished_data();

    // Size-prefixed stream write
    let mut writer = pipe_writer.lock().unwrap();
    let length = data.len() as u32;
    let _ = writer.write_all(&length.to_le_bytes());
    let _ = writer.write_all(data);
}
