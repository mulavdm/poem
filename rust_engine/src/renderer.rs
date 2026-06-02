use std::sync::Arc;
use wgpu::util::DeviceExt;

#[repr(C)]
#[derive(Copy, Clone, Debug, bytemuck::Pod, bytemuck::Zeroable)]
pub struct Vertex {
    pub pos: [f32; 2],
    pub uv: [f32; 2],
    pub color: [f32; 4],
    pub params: [f32; 4],
    pub radius: f32,
    pub draw_type: f32,
    pub glow: f32,
    pub is_glass: f32,
    pub shadow_offset: [f32; 2],
    pub shadow_softness: f32,
    pub padding: f32,
}

#[repr(C)]
#[derive(Copy, Clone, Debug, bytemuck::Pod, bytemuck::Zeroable)]
struct Globals {
    projection: [f32; 16],
    screen_size: [f32; 2],
    padding: [f32; 2],
}

#[derive(Clone, Copy, Debug)]
struct DrawBatch {
    start_vertex: u32,
    vertex_count: u32,
    clip_rect: Option<[u32; 4]>,
}

pub struct Renderer {
    device: Arc<wgpu::Device>,
    queue: Arc<wgpu::Queue>,
    pub surface_config: wgpu::SurfaceConfiguration,

    // Pipelines
    render_pipeline: wgpu::RenderPipeline,
    blur_horizontal_pipeline: wgpu::RenderPipeline,
    blur_vertical_pipeline: wgpu::RenderPipeline,

    // Textures & Samplers
    #[allow(dead_code)]
    text_atlas_texture: wgpu::Texture,
    text_atlas_view: wgpu::TextureView,
    sampler: wgpu::Sampler,

    // Glassmorphism Buffers
    fbo_texture: wgpu::Texture,
    fbo_view: wgpu::TextureView,
    pingpong_textures: [wgpu::Texture; 2],
    pingpong_views: [wgpu::TextureView; 2],

    // Bind Groups
    globals_buffer: wgpu::Buffer,
    main_bind_group: wgpu::BindGroup,
    blur_horizontal_bind_group: wgpu::BindGroup,
    blur_horizontal_pingpong_bind_group: wgpu::BindGroup,
    blur_vertical_bind_group: wgpu::BindGroup,

    // Rendering State
    batch_vertices: Vec<Vertex>,
    batches: Vec<DrawBatch>,
    offset_x: f32,
    offset_y: f32,
    glow: f32,
    is_glass: f32,
    shadow_offset: [f32; 2],
    shadow_blur: f32,
    clip_rect: Option<[u32; 4]>, // [x, y, w, h]
    scale_factor: f32,
    pub player_x: f32,
    pub player_y: f32,
    pub player_angle: f32,
}

impl Renderer {
    pub fn new(
        device: Arc<wgpu::Device>,
        queue: Arc<wgpu::Queue>,
        surface_config: wgpu::SurfaceConfiguration,
        atlas_w: u32,
        atlas_h: u32,
        atlas_pixels: &[u8],
    ) -> Self {
        // Shaders
        let shader = device.create_shader_module(wgpu::ShaderModuleDescriptor {
            label: Some("Main Shader"),
            source: wgpu::ShaderSource::Wgsl(include_str!("shaders/main.wgsl").into()),
        });

        let blur_shader = device.create_shader_module(wgpu::ShaderModuleDescriptor {
            label: Some("Blur Shader"),
            source: wgpu::ShaderSource::Wgsl(include_str!("shaders/blur.wgsl").into()),
        });

        // Buffers
        let globals_buffer = device.create_buffer(&wgpu::BufferDescriptor {
            label: Some("Globals Uniform Buffer"),
            size: std::mem::size_of::<Globals>() as u64,
            usage: wgpu::BufferUsages::UNIFORM | wgpu::BufferUsages::COPY_DST,
            mapped_at_creation: false,
        });

        // Font Atlas Texture (LayerMajor is the correct wgpu 0.19 variant)
        let text_atlas_texture = device.create_texture_with_data(
            &queue,
            &wgpu::TextureDescriptor {
                label: Some("Font Atlas"),
                size: wgpu::Extent3d {
                    width: atlas_w,
                    height: atlas_h,
                    depth_or_array_layers: 1,
                },
                mip_level_count: 1,
                sample_count: 1,
                dimension: wgpu::TextureDimension::D2,
                format: wgpu::TextureFormat::Rgba8Unorm,
                usage: wgpu::TextureUsages::TEXTURE_BINDING | wgpu::TextureUsages::COPY_DST,
                view_formats: &[],
            },
            wgpu::util::TextureDataOrder::LayerMajor,
            atlas_pixels,
        );
        let text_atlas_view = text_atlas_texture.create_view(&wgpu::TextureViewDescriptor::default());

        let sampler = device.create_sampler(&wgpu::SamplerDescriptor {
            label: Some("Sampler"),
            address_mode_u: wgpu::AddressMode::ClampToEdge,
            address_mode_v: wgpu::AddressMode::ClampToEdge,
            mag_filter: wgpu::FilterMode::Linear,
            min_filter: wgpu::FilterMode::Linear,
            mipmap_filter: wgpu::FilterMode::Linear,
            ..Default::default()
        });

        // Create Glassmorphic Blur Buffers
        let fbo_texture = device.create_texture(&wgpu::TextureDescriptor {
            label: Some("Background Texture"),
            size: wgpu::Extent3d {
                width: surface_config.width,
                height: surface_config.height,
                depth_or_array_layers: 1,
            },
            mip_level_count: 1,
            sample_count: 1,
            dimension: wgpu::TextureDimension::D2,
            format: surface_config.format,
            usage: wgpu::TextureUsages::RENDER_ATTACHMENT | wgpu::TextureUsages::TEXTURE_BINDING,
            view_formats: &[],
        });
        let fbo_view = fbo_texture.create_view(&wgpu::TextureViewDescriptor::default());

        let mut pingpong_textures = Vec::new();
        let mut pingpong_views = Vec::new();
        for i in 0..2 {
            let tex = device.create_texture(&wgpu::TextureDescriptor {
                label: Some(&format!("Ping-Pong Texture {}", i)),
                size: wgpu::Extent3d {
                    width: surface_config.width,
                    height: surface_config.height,
                    depth_or_array_layers: 1,
                },
                mip_level_count: 1,
                sample_count: 1,
                dimension: wgpu::TextureDimension::D2,
                format: surface_config.format,
                usage: wgpu::TextureUsages::RENDER_ATTACHMENT | wgpu::TextureUsages::TEXTURE_BINDING,
                view_formats: &[],
            });
            let view = tex.create_view(&wgpu::TextureViewDescriptor::default());
            pingpong_textures.push(tex);
            pingpong_views.push(view);
        }

        let pingpong_textures = [pingpong_textures.remove(0), pingpong_textures.remove(0)];
        let pingpong_views = [pingpong_views.remove(0), pingpong_views.remove(0)];

        // Bind Group Layouts
        let main_bind_group_layout = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
            label: Some("Main Bind Group Layout"),
            entries: &[
                wgpu::BindGroupLayoutEntry {
                    binding: 0,
                    visibility: wgpu::ShaderStages::VERTEX_FRAGMENT,
                    ty: wgpu::BindingType::Buffer {
                        ty: wgpu::BufferBindingType::Uniform,
                        has_dynamic_offset: false,
                        min_binding_size: None,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 1,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Texture {
                        sample_type: wgpu::TextureSampleType::Float { filterable: true },
                        view_dimension: wgpu::TextureViewDimension::D2,
                        multisampled: false,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 2,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Sampler(wgpu::SamplerBindingType::Filtering),
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 3,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Texture {
                        sample_type: wgpu::TextureSampleType::Float { filterable: true },
                        view_dimension: wgpu::TextureViewDimension::D2,
                        multisampled: false,
                    },
                    count: None,
                },
            ],
        });

        let main_bind_group = device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Main Bind Group"),
            layout: &main_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: globals_buffer.as_entire_binding(),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::TextureView(&text_atlas_view),
                },
                wgpu::BindGroupEntry {
                    binding: 2,
                    resource: wgpu::BindingResource::Sampler(&sampler),
                },
                wgpu::BindGroupEntry {
                    binding: 3,
                    resource: wgpu::BindingResource::TextureView(&pingpong_views[1]), // blurred output
                },
            ],
        });

        // Blur Bind Groups
        let blur_bind_group_layout = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
            label: Some("Blur Bind Group Layout"),
            entries: &[
                wgpu::BindGroupLayoutEntry {
                    binding: 0,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Texture {
                        sample_type: wgpu::TextureSampleType::Float { filterable: true },
                        view_dimension: wgpu::TextureViewDimension::D2,
                        multisampled: false,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 1,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Sampler(wgpu::SamplerBindingType::Filtering),
                    count: None,
                },
            ],
        });

        let blur_horizontal_bind_group = device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Blur Horizontal Bind Group"),
            layout: &blur_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: wgpu::BindingResource::TextureView(&fbo_view),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::Sampler(&sampler),
                },
            ],
        });

        let blur_horizontal_pingpong_bind_group = device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Blur Horizontal Ping-Pong Bind Group"),
            layout: &blur_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: wgpu::BindingResource::TextureView(&pingpong_views[1]),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::Sampler(&sampler),
                },
            ],
        });

        let blur_vertical_bind_group = device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Blur Vertical Bind Group"),
            layout: &blur_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: wgpu::BindingResource::TextureView(&pingpong_views[0]),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::Sampler(&sampler),
                },
            ],
        });

        // Pipelines Layout
        let render_pipeline_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("Render Pipeline Layout"),
            bind_group_layouts: &[&main_bind_group_layout],
            push_constant_ranges: &[],
        });

        let render_pipeline = device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
            label: Some("Main Render Pipeline"),
            layout: Some(&render_pipeline_layout),
            vertex: wgpu::VertexState {
                module: &shader,
                entry_point: "vs_main",
                buffers: &[wgpu::VertexBufferLayout {
                    array_stride: std::mem::size_of::<Vertex>() as wgpu::BufferAddress,
                    step_mode: wgpu::VertexStepMode::Vertex,
                    attributes: &wgpu::vertex_attr_array![
                        0 => Float32x2, // pos
                        1 => Float32x2, // uv
                        2 => Float32x4, // color
                        3 => Float32x4, // rect_params
                        4 => Float32,   // radius
                        5 => Float32,   // draw_type
                        6 => Float32,   // glow
                        7 => Float32,   // is_glass
                        8 => Float32x2, // shadow_offset
                        9 => Float32,   // shadow_softness
                        10 => Float32,  // padding
                    ],
                }],
            },
            fragment: Some(wgpu::FragmentState {
                module: &shader,
                entry_point: "fs_main",
                targets: &[Some(wgpu::ColorTargetState {
                    format: surface_config.format,
                    blend: Some(wgpu::BlendState::ALPHA_BLENDING),
                    write_mask: wgpu::ColorWrites::ALL,
                })],
            }),
            primitive: wgpu::PrimitiveState {
                topology: wgpu::PrimitiveTopology::TriangleList,
                ..Default::default()
            },
            depth_stencil: None,
            multisample: wgpu::MultisampleState::default(),
            multiview: None,
        });

        let blur_pipeline_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("Blur Pipeline Layout"),
            bind_group_layouts: &[&blur_bind_group_layout],
            push_constant_ranges: &[],
        });

        let blur_horizontal_pipeline = device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
            label: Some("Blur Horizontal Pipeline"),
            layout: Some(&blur_pipeline_layout),
            vertex: wgpu::VertexState {
                module: &blur_shader,
                entry_point: "vs_main",
                buffers: &[],
            },
            fragment: Some(wgpu::FragmentState {
                module: &blur_shader,
                entry_point: "fs_horizontal",
                targets: &[Some(wgpu::ColorTargetState {
                    format: surface_config.format,
                    blend: None,
                    write_mask: wgpu::ColorWrites::ALL,
                })],
            }),
            primitive: wgpu::PrimitiveState {
                topology: wgpu::PrimitiveTopology::TriangleStrip,
                ..Default::default()
            },
            depth_stencil: None,
            multisample: wgpu::MultisampleState::default(),
            multiview: None,
        });

        let blur_vertical_pipeline = device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
            label: Some("Blur Vertical Pipeline"),
            layout: Some(&blur_pipeline_layout),
            vertex: wgpu::VertexState {
                module: &blur_shader,
                entry_point: "vs_main",
                buffers: &[],
            },
            fragment: Some(wgpu::FragmentState {
                module: &blur_shader,
                entry_point: "fs_vertical",
                targets: &[Some(wgpu::ColorTargetState {
                    format: surface_config.format,
                    blend: None,
                    write_mask: wgpu::ColorWrites::ALL,
                })],
            }),
            primitive: wgpu::PrimitiveState {
                topology: wgpu::PrimitiveTopology::TriangleStrip,
                ..Default::default()
            },
            depth_stencil: None,
            multisample: wgpu::MultisampleState::default(),
            multiview: None,
        });

        Self {
            device,
            queue,
            surface_config,
            render_pipeline,
            blur_horizontal_pipeline,
            blur_vertical_pipeline,
            text_atlas_texture,
            text_atlas_view,
            sampler,
            fbo_texture,
            fbo_view,
            pingpong_textures,
            pingpong_views,
            globals_buffer,
            main_bind_group,
            blur_horizontal_bind_group,
            blur_horizontal_pingpong_bind_group,
            blur_vertical_bind_group,
            batch_vertices: Vec::with_capacity(512),
            batches: Vec::new(),
            offset_x: 0.0,
            offset_y: 0.0,
            glow: 0.0,
            is_glass: 0.0,
            shadow_offset: [0.0, 0.0],
            shadow_blur: 0.0,
            clip_rect: None,
            scale_factor: 1.0,
            player_x: 0.0,
            player_y: 0.0,
            player_angle: 0.0,
        }
    }

    pub fn resize(&mut self, width: u32, height: u32, scale_factor: f32) {
        if width == 0 || height == 0 {
            return;
        }
        self.scale_factor = scale_factor;
        self.surface_config.width = width;
        self.surface_config.height = height;

        // Recreate blur framebuffers
        self.fbo_texture = self.device.create_texture(&wgpu::TextureDescriptor {
            label: Some("Background Texture"),
            size: wgpu::Extent3d {
                width,
                height,
                depth_or_array_layers: 1,
            },
            mip_level_count: 1,
            sample_count: 1,
            dimension: wgpu::TextureDimension::D2,
            format: self.surface_config.format,
            usage: wgpu::TextureUsages::RENDER_ATTACHMENT | wgpu::TextureUsages::TEXTURE_BINDING,
            view_formats: &[],
        });
        self.fbo_view = self.fbo_texture.create_view(&wgpu::TextureViewDescriptor::default());

        for i in 0..2 {
            self.pingpong_textures[i] = self.device.create_texture(&wgpu::TextureDescriptor {
                label: Some(&format!("Ping-Pong Texture {}", i)),
                size: wgpu::Extent3d {
                    width,
                    height,
                    depth_or_array_layers: 1,
                },
                mip_level_count: 1,
                sample_count: 1,
                dimension: wgpu::TextureDimension::D2,
                format: self.surface_config.format,
                usage: wgpu::TextureUsages::RENDER_ATTACHMENT | wgpu::TextureUsages::TEXTURE_BINDING,
                view_formats: &[],
            });
            self.pingpong_views[i] = self.pingpong_textures[i].create_view(&wgpu::TextureViewDescriptor::default());
        }

        // Re-compile orthographic projection matrix using logical bounds to support high-DPI scaling automatically
        let left = 0.0;
        let right = width as f32 / scale_factor;
        let bottom = height as f32 / scale_factor; // Top-down
        let top = 0.0;
        let near = -1.0;
        let far = 1.0;

        let projection = [
            2.0 / (right - left), 0.0, 0.0, 0.0,
            0.0, 2.0 / (top - bottom), 0.0, 0.0,
            0.0, 0.0, -2.0 / (far - near), 0.0,
            -(right + left) / (right - left), -(top + bottom) / (top - bottom), -(far + near) / (far - near), 1.0,
        ];

        let globals = Globals {
            projection,
            screen_size: [width as f32, height as f32],
            padding: [0.0, 0.0],
        };

        self.queue.write_buffer(
            &self.globals_buffer,
            0,
            bytemuck::cast_slice(&[globals]),
        );

        // Rebuild bind groups to bind new views
        let main_bind_group_layout = self.render_pipeline.get_bind_group_layout(0);
        self.main_bind_group = self.device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Main Bind Group"),
            layout: &main_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: self.globals_buffer.as_entire_binding(),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::TextureView(&self.text_atlas_view),
                },
                wgpu::BindGroupEntry {
                    binding: 2,
                    resource: wgpu::BindingResource::Sampler(&self.sampler),
                },
                wgpu::BindGroupEntry {
                    binding: 3,
                    resource: wgpu::BindingResource::TextureView(&self.pingpong_views[1]), // blurred output
                },
            ],
        });

        let blur_bind_group_layout = self.blur_horizontal_pipeline.get_bind_group_layout(0);
        self.blur_horizontal_bind_group = self.device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Blur Horizontal Bind Group"),
            layout: &blur_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: wgpu::BindingResource::TextureView(&self.fbo_view),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::Sampler(&self.sampler),
                },
            ],
        });

        self.blur_horizontal_pingpong_bind_group = self.device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Blur Horizontal Ping-Pong Bind Group"),
            layout: &blur_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: wgpu::BindingResource::TextureView(&self.pingpong_views[1]),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::Sampler(&self.sampler),
                },
            ],
        });

        self.blur_vertical_bind_group = self.device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("Blur Vertical Bind Group"),
            layout: &blur_bind_group_layout,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: wgpu::BindingResource::TextureView(&self.pingpong_views[0]),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::Sampler(&self.sampler),
                },
            ],
        });
    }

    pub fn draw_rounded_rect(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, r: f32, col: [f32; 4]) {
        self.push_quad(x1, y1, x2, y2, r, col, 0.0, [0.0, 0.0, 0.0, 0.0]);
    }

    pub fn draw_raycaster(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, px: f32, py: f32, angle: f32, col: [f32; 4]) {
        self.player_x = px;
        self.player_y = py;
        self.player_angle = angle;
        self.push_quad_special(x1, y1, x2, y2, -999.0, col, 0.0, [0.0, 0.0, 0.0, 0.0], [px, py], angle, 0.0);
    }

    pub fn draw_billboard(&mut self, viewport_x1: f32, viewport_y1: f32, viewport_x2: f32, viewport_y2: f32, radius: f32, sx: f32, sy: f32, col: [f32; 4]) {
        let viewport_w = viewport_x2 - viewport_x1;
        let viewport_h = viewport_y2 - viewport_y1;
        let y_center = viewport_y1 + viewport_h / 2.0 + 30.0;

        let angle = self.player_angle;
        let dir_x = angle.cos();
        let dir_y = angle.sin();
        let plane_x = -dir_y * 0.66;
        let plane_y = dir_x * 0.66;

        let sprite_x = sx - self.player_x;
        let sprite_y = sy - self.player_y;

        let inv_det = 1.0 / (plane_x * dir_y - dir_x * plane_y);
        let transform_x = inv_det * (dir_y * sprite_x - dir_x * sprite_y);
        let transform_y = inv_det * (-plane_y * sprite_x + plane_x * sprite_y); // depth

        if transform_y > 0.1 {
            let sprite_screen_x = ((viewport_w / 2.0) * (1.0 + transform_x / transform_y)) + viewport_x1;
            
            let factor: f32 = if radius == -998.0 {
                0.38
            } else if radius == -997.0 {
                0.72
            } else if radius == -996.0 {
                0.92
            } else {
                0.32
            };
            let sprite_h = (viewport_h * factor / transform_y).abs();
            let sprite_w = sprite_h;

            let x1 = sprite_screen_x - sprite_w / 2.0;
            let y1 = y_center - sprite_h / 2.0;
            let x2 = sprite_screen_x + sprite_w / 2.0;
            let y2 = y_center + sprite_h / 2.0;

            self.push_quad_special(
                x1, y1, x2, y2,
                radius, col, 0.0, [0.0, 0.0, 0.0, 0.0],
                [sx, sy], self.player_angle, transform_y
            );
        }
    }

    fn push_quad_special(
        &mut self,
        x1: f32,
        y1: f32,
        x2: f32,
        y2: f32,
        radius: f32,
        col: [f32; 4],
        draw_type: f32,
        _uv: [f32; 4],
        shadow_offset: [f32; 2],
        shadow_softness: f32,
        padding: f32,
    ) {
        if self.batches.is_empty() {
            self.batches.push(DrawBatch {
                start_vertex: 0,
                vertex_count: 0,
                clip_rect: self.clip_rect,
            });
        }
        let ox = self.offset_x;
        let oy = self.offset_y;
        let p_x1 = x1 + ox;
        let p_y1 = y1 + oy;
        let p_x2 = x2 + ox;
        let p_y2 = y2 + oy;

        let rect_params = [p_x1, p_y1, x2 - x1, y2 - y1];

        let v1 = Vertex {
            pos: [p_x1, p_y1],
            uv: [self.player_x, self.player_y],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: 0.0,
            is_glass: 0.0,
            shadow_offset,
            shadow_softness,
            padding,
        };
        let v2 = Vertex {
            pos: [p_x2, p_y1],
            uv: [self.player_x, self.player_y],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: 0.0,
            is_glass: 0.0,
            shadow_offset,
            shadow_softness,
            padding,
        };
        let v3 = Vertex {
            pos: [p_x2, p_y2],
            uv: [self.player_x, self.player_y],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: 0.0,
            is_glass: 0.0,
            shadow_offset,
            shadow_softness,
            padding,
        };
        let v4 = Vertex {
            pos: [p_x1, p_y2],
            uv: [self.player_x, self.player_y],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: 0.0,
            is_glass: 0.0,
            shadow_offset,
            shadow_softness,
            padding,
        };

        self.batch_vertices.push(v1);
        self.batch_vertices.push(v2);
        self.batch_vertices.push(v3);

        self.batch_vertices.push(v1);
        self.batch_vertices.push(v3);
        self.batch_vertices.push(v4);

        if let Some(last) = self.batches.last_mut() {
            last.vertex_count += 6;
        }
    }

    pub fn fill_rect(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, col: [f32; 4]) {
        self.push_quad(x1, y1, x2, y2, 0.0, col, 0.0, [0.0, 0.0, 0.0, 0.0]);
    }

    pub fn draw_line(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, col: [f32; 4]) {
        let dx = (x2 - x1).abs();
        let dy = (y2 - y1).abs();
        if dy < 0.1 {
            self.fill_rect(x1, y1, x2, y1 + 1.0, col);
        } else if dx < 0.1 {
            self.fill_rect(x1, y1, x1 + 1.0, y2, col);
        } else {
            self.fill_rect(x1, y1, x1 + 2.0, y1 + 2.0, col);
        }
    }

    pub fn set_glow(&mut self, glow: f32) {
        self.glow = glow;
    }

    pub fn set_glass(&mut self, glass: bool) {
        self.is_glass = if glass { 1.0 } else { 0.0 };
    }

    pub fn set_shadow(&mut self, ox: f32, oy: f32, blur: f32) {
        self.shadow_offset = [ox, oy];
        self.shadow_blur = blur;
    }

    pub fn set_offset(&mut self, x: f32, y: f32) {
        self.offset_x = x;
        self.offset_y = y;
    }

    pub fn set_clip(&mut self, x: f32, y: f32, w: f32, h: f32, enabled: bool) {
        let new_clip = if enabled {
            Some([x as u32, y as u32, w as u32, h as u32])
        } else {
            None
        };

        if self.clip_rect != new_clip {
            self.clip_rect = new_clip;

            let current_vertex_count = self.batch_vertices.len() as u32;
            if current_vertex_count > 0 {
                self.batches.push(DrawBatch {
                    start_vertex: current_vertex_count,
                    vertex_count: 0,
                    clip_rect: self.clip_rect,
                });
            } else if let Some(last) = self.batches.last_mut() {
                last.clip_rect = self.clip_rect;
            }
        }
    }

    pub fn draw_text_char(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, uv: [f32; 4], col: [f32; 4]) {
        self.push_quad(x1, y1, x2, y2, 0.0, col, 1.0, uv);
    }

    fn push_quad(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, radius: f32, col: [f32; 4], draw_type: f32, uv: [f32; 4]) {
        if self.batches.is_empty() {
            self.batches.push(DrawBatch {
                start_vertex: 0,
                vertex_count: 0,
                clip_rect: self.clip_rect,
            });
        }
        let ox = self.offset_x;
        let oy = self.offset_y;
        let p_x1 = x1 + ox;
        let p_y1 = y1 + oy;
        let p_x2 = x2 + ox;
        let p_y2 = y2 + oy;

        let rect_params = [p_x1, p_y1, x2 - x1, y2 - y1];
        let glow_val = if draw_type > 0.5 { 0.0 } else { self.glow };

        let v1 = Vertex {
            pos: [p_x1, p_y1],
            uv: [uv[0], uv[1]],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: glow_val,
            is_glass: self.is_glass,
            shadow_offset: self.shadow_offset,
            shadow_softness: self.shadow_blur,
            padding: 0.0,
        };
        let v2 = Vertex {
            pos: [p_x2, p_y1],
            uv: [uv[2], uv[1]],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: glow_val,
            is_glass: self.is_glass,
            shadow_offset: self.shadow_offset,
            shadow_softness: self.shadow_blur,
            padding: 0.0,
        };
        let v3 = Vertex {
            pos: [p_x2, p_y2],
            uv: [uv[2], uv[3]],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: glow_val,
            is_glass: self.is_glass,
            shadow_offset: self.shadow_offset,
            shadow_softness: self.shadow_blur,
            padding: 0.0,
        };
        let v4 = Vertex {
            pos: [p_x1, p_y2],
            uv: [uv[0], uv[3]],
            color: col,
            params: rect_params,
            radius,
            draw_type,
            glow: glow_val,
            is_glass: self.is_glass,
            shadow_offset: self.shadow_offset,
            shadow_softness: self.shadow_blur,
            padding: 0.0,
        };

        // Triangles: v1, v2, v3, v1, v3, v4
        self.batch_vertices.push(v1);
        self.batch_vertices.push(v2);
        self.batch_vertices.push(v3);

        self.batch_vertices.push(v1);
        self.batch_vertices.push(v3);
        self.batch_vertices.push(v4);

        if let Some(last) = self.batches.last_mut() {
            last.vertex_count += 6;
        }
    }

    pub fn render(&mut self, view: &wgpu::TextureView) {
        if self.batch_vertices.is_empty() {
            return;
        }

        // Upload batch vertex buffer
        let vertex_buffer = self.device.create_buffer_init(&wgpu::util::BufferInitDescriptor {
            label: Some("Batch Vertex Buffer"),
            contents: bytemuck::cast_slice(&self.batch_vertices),
            usage: wgpu::BufferUsages::VERTEX,
        });

        // wgpu command encoder
        let mut encoder = self.device.create_command_encoder(&wgpu::CommandEncoderDescriptor {
            label: Some("Render Encoder"),
        });

        // Pass 1: Render background particles to FBO
        {
            let mut render_pass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                label: Some("Pass 1 (FBO Render)"),
                color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                    view: &self.fbo_view,
                    resolve_target: None,
                    ops: wgpu::Operations {
                        load: wgpu::LoadOp::Clear(wgpu::Color {
                            r: 0.04,
                            g: 0.04,
                            b: 0.06,
                            a: 1.0,
                        }),
                        store: wgpu::StoreOp::Store,
                    },
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
            });

            render_pass.set_pipeline(&self.render_pipeline);
            render_pass.set_bind_group(0, &self.main_bind_group, &[]);
            render_pass.set_vertex_buffer(0, vertex_buffer.slice(..));
            
            for batch in &self.batches {
                if let Some(clip) = batch.clip_rect {
                    let clip_x = ((clip[0] as f32 * self.scale_factor) as u32).min(self.surface_config.width);
                    let clip_y = ((clip[1] as f32 * self.scale_factor) as u32).min(self.surface_config.height);
                    let clip_w = ((clip[2] as f32 * self.scale_factor) as u32).min(self.surface_config.width - clip_x);
                    let clip_h = ((clip[3] as f32 * self.scale_factor) as u32).min(self.surface_config.height - clip_y);
                    if clip_w > 0 && clip_h > 0 {
                        render_pass.set_scissor_rect(clip_x, clip_y, clip_w, clip_h);
                    } else {
                        render_pass.set_scissor_rect(0, 0, self.surface_config.width, self.surface_config.height);
                    }
                } else {
                    render_pass.set_scissor_rect(0, 0, self.surface_config.width, self.surface_config.height);
                }
                render_pass.draw(batch.start_vertex..batch.start_vertex + batch.vertex_count, 0..1);
            }
        }

        // Pass 2: Gaussian Blur FBO Texture (4 iterations)
        for i in 0..2 {
            // Horizontal blur
            {
                let mut render_pass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                    label: Some(&format!("Pass 2.{} - Horizontal Blur", i)),
                    color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                        view: &self.pingpong_views[0],
                        resolve_target: None,
                        ops: wgpu::Operations {
                            load: wgpu::LoadOp::Clear(wgpu::Color::TRANSPARENT),
                            store: wgpu::StoreOp::Store,
                        },
                    })],
                    depth_stencil_attachment: None,
                    timestamp_writes: None,
                    occlusion_query_set: None,
                });

                render_pass.set_pipeline(&self.blur_horizontal_pipeline);
                if i == 0 {
                    render_pass.set_bind_group(0, &self.blur_horizontal_bind_group, &[]);
                } else {
                    render_pass.set_bind_group(0, &self.blur_horizontal_pingpong_bind_group, &[]);
                }
                render_pass.draw(0..4, 0..1);
            }

            // Vertical blur
            {
                let mut render_pass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                    label: Some(&format!("Pass 2.{} - Vertical Blur", i)),
                    color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                        view: &self.pingpong_views[1],
                        resolve_target: None,
                        ops: wgpu::Operations {
                            load: wgpu::LoadOp::Clear(wgpu::Color::TRANSPARENT),
                            store: wgpu::StoreOp::Store,
                        },
                    })],
                    depth_stencil_attachment: None,
                    timestamp_writes: None,
                    occlusion_query_set: None,
                });

                render_pass.set_pipeline(&self.blur_vertical_pipeline);
                render_pass.set_bind_group(0, &self.blur_vertical_bind_group, &[]);
                render_pass.draw(0..4, 0..1);
            }
        }

        // Pass 3: Draw the entire UI batch onto the main Screen View
        {
            let mut render_pass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                label: Some("Pass 3 (Screen Composite)"),
                color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                    view,
                    resolve_target: None,
                    ops: wgpu::Operations {
                        load: wgpu::LoadOp::Clear(wgpu::Color {
                            r: 0.04,
                            g: 0.04,
                            b: 0.06,
                            a: 1.0,
                        }),
                        store: wgpu::StoreOp::Store,
                    },
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
            });

            render_pass.set_pipeline(&self.render_pipeline);
            render_pass.set_bind_group(0, &self.main_bind_group, &[]);
            render_pass.set_vertex_buffer(0, vertex_buffer.slice(..));

            for batch in &self.batches {
                if let Some(clip) = batch.clip_rect {
                    let clip_x = ((clip[0] as f32 * self.scale_factor) as u32).min(self.surface_config.width);
                    let clip_y = ((clip[1] as f32 * self.scale_factor) as u32).min(self.surface_config.height);
                    let clip_w = ((clip[2] as f32 * self.scale_factor) as u32).min(self.surface_config.width - clip_x);
                    let clip_h = ((clip[3] as f32 * self.scale_factor) as u32).min(self.surface_config.height - clip_y);
                    if clip_w > 0 && clip_h > 0 {
                        render_pass.set_scissor_rect(clip_x, clip_y, clip_w, clip_h);
                    } else {
                        render_pass.set_scissor_rect(0, 0, self.surface_config.width, self.surface_config.height);
                    }
                } else {
                    render_pass.set_scissor_rect(0, 0, self.surface_config.width, self.surface_config.height);
                }
                render_pass.draw(batch.start_vertex..batch.start_vertex + batch.vertex_count, 0..1);
            }
        }

        self.queue.submit(std::iter::once(encoder.finish()));
        self.batch_vertices.clear();
        self.batches.clear();

        // Reset state
        self.glow = 0.0;
        self.is_glass = 0.0;
        self.shadow_offset = [0.0, 0.0];
        self.shadow_blur = 0.0;
        self.offset_x = 0.0;
        self.offset_y = 0.0;
        self.clip_rect = None;
    }
}
