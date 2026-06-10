struct Globals {
    projection: mat4x4<f32>,
    screen_size: vec2<f32>,
}

struct MapData {
    cells: array<u32, 4096>,
}

@group(0) @binding(0) var<uniform> globals: Globals;
@group(0) @binding(1) var text_atlas: texture_2d<f32>;
@group(0) @binding(2) var text_sampler: sampler;
@group(0) @binding(3) var blurred_bg: texture_2d<f32>;
@group(0) @binding(4) var<storage, read> map_data: MapData;

struct VertexInput {
    @location(0) pos: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) color: vec4<f32>,
    @location(3) rect_params: vec4<f32>,
    @location(4) radius: f32,
    @location(5) draw_type: f32,
    @location(6) glow: f32,
    @location(7) is_glass: f32,
    @location(8) shadow_offset: vec2<f32>,
    @location(9) shadow_softness: f32,
    @location(10) padding: f32,
}

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) frag_pos: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) color: vec4<f32>,
    @location(3) @interpolate(flat) rect_params: vec4<f32>,
    @location(4) @interpolate(flat) radius: f32,
    @location(5) @interpolate(flat) draw_type: f32,
    @location(6) @interpolate(flat) glow: f32,
    @location(7) @interpolate(flat) is_glass: f32,
    @location(8) @interpolate(flat) shadow_offset: vec2<f32>,
    @location(9) @interpolate(flat) shadow_softness: f32,
    @location(10) @interpolate(flat) padding: f32,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.clip_position = globals.projection * vec4<f32>(in.pos, 0.0, 1.0);
    out.frag_pos = in.pos;
    out.uv = in.uv;
    out.color = in.color;
    out.rect_params = in.rect_params;
    out.radius = in.radius;
    out.draw_type = in.draw_type;
    out.glow = in.glow;
    out.is_glass = in.is_glass;
    out.shadow_offset = in.shadow_offset;
    out.shadow_softness = in.shadow_softness;
    out.padding = in.padding;
    return out;
}

fn sd_rounded_rect(p: vec2<f32>, b: vec2<f32>, r: f32) -> f32 {
    let q = abs(p) - b + vec2<f32>(r, r);
    return length(max(q, vec2<f32>(0.0))) + min(max(q.x, q.y), 0.0) - r;
}

fn get_grid_val(x: i32, y: i32) -> u32 {
    if (x < 0 || x >= 64 || y < 0 || y >= 64) {
        return 1u;
    }
    return map_data.cells[u32(y * 64 + x)];
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    if (in.radius == -999.0) {
        // VIEWPORT RAYCASTER PASS!
        let posX = in.shadow_offset.x;
        let posY = in.shadow_offset.y;
        let angle = in.shadow_softness;
        
        let col_ratio = (in.frag_pos.x - in.rect_params.x) / in.rect_params.z;
        let cameraX = 2.0 * col_ratio - 1.0;
        
        let dirX = cos(angle);
        let dirY = sin(angle);
        let planeX = -dirY * 0.66;
        let planeY = dirX * 0.66;

        let rayDirX = dirX + planeX * cameraX;
        let rayDirY = dirY + planeY * cameraX;
        
        var mapX = i32(posX);
        var mapY = i32(posY);

        var deltaDistX = 1e30;
        if (rayDirX != 0.0) {
            deltaDistX = abs(1.0 / rayDirX);
        }
        var deltaDistY = 1e30;
        if (rayDirY != 0.0) {
            deltaDistY = abs(1.0 / rayDirY);
        }

        var stepX = 0;
        var sideDistX = 0.0;
        if (rayDirX < 0.0) {
            stepX = -1;
            sideDistX = (posX - f32(mapX)) * deltaDistX;
        } else {
            stepX = 1;
            sideDistX = (f32(mapX) + 1.0 - posX) * deltaDistX;
        }

        var stepY = 0;
        var sideDistY = 0.0;
        if (rayDirY < 0.0) {
            stepY = -1;
            sideDistY = (posY - f32(mapY)) * deltaDistY;
        } else {
            stepY = 1;
            sideDistY = (f32(mapY) + 1.0 - posY) * deltaDistY;
        }

        var hit = 0u;
        var side = 0;
        var maxSteps = 40;
        for (var step = 0; step < maxSteps; step = step + 1) {
            if (hit > 0u) {
                break;
            }
            if (sideDistX < sideDistY) {
                sideDistX = sideDistX + deltaDistX;
                mapX = mapX + stepX;
                side = 0;
            } else {
                sideDistY = sideDistY + deltaDistY;
                mapY = mapY + stepY;
                side = 1;
            }
            
            if (mapX < 0 || mapX >= 64 || mapY < 0 || mapY >= 64) {
                break;
            }
            
            let cell = get_grid_val(mapX, mapY);
            if (cell == 1u) {
                hit = cell;
            }
        }
        
        var perpWallDist = 0.0;
        if (side == 0) {
            perpWallDist = (f32(mapX) - posX + (1.0 - f32(stepX)) / 2.0) / rayDirX;
        } else {
            perpWallDist = (f32(mapY) - posY + (1.0 - f32(stepY)) / 2.0) / rayDirY;
        }

        if (perpWallDist <= 0.05) {
            perpWallDist = 0.05;
        }
        
        let wall_h = in.rect_params.w * 0.78 / perpWallDist;
        let y_center = in.rect_params.y + in.rect_params.w / 2.0 + 30.0;
        let draw_start = y_center - wall_h / 2.0;
        let draw_end = y_center + wall_h / 2.0;
        var wall_x = 0.0;
        if (side == 0) {
            wall_x = posY + perpWallDist * rayDirY;
        } else {
            wall_x = posX + perpWallDist * rayDirX;
        }
        wall_x = fract(wall_x);
        
        if (in.frag_pos.y >= draw_start && in.frag_pos.y <= draw_end) {
            // Warm toy-plastic habitat walls, tinted by the game-provided room accent.
            let accent = max(in.color.rgb, vec3<f32>(0.35, 0.18, 0.08));
            var wall_color = mix(vec3<f32>(0.95, 0.42, 0.18), accent, 0.35);
            if (side == 1) {
                wall_color = wall_color * 0.72;
            }
            let depth_shading = 1.5 / (1.0 + perpWallDist * 0.08);
            wall_color = wall_color * clamp(depth_shading, 0.0, 1.0);

            let stripe = step(0.92, fract((f32(mapX) + f32(mapY) + in.frag_pos.y * 0.015) * 0.5));
            let rib = 1.0 - smoothstep(0.0, 0.08, abs(wall_x - 0.5));
            let panel = smoothstep(0.18, 0.0, abs(fract(wall_x * 2.0) - 0.5));
            let viewport_v = clamp((in.frag_pos.y - draw_start) / max(draw_end - draw_start, 1.0), 0.0, 1.0);
            let base_strip = smoothstep(0.82, 1.0, viewport_v);
            let top_gloss = smoothstep(0.22, 0.0, viewport_v) * smoothstep(0.20, 0.0, abs(wall_x - 0.18));
            wall_color = mix(wall_color, wall_color + vec3<f32>(0.16, 0.10, 0.03), stripe * 0.35);
            wall_color = mix(wall_color, wall_color + vec3<f32>(0.12, 0.09, 0.06), rib * 0.32);
            wall_color = mix(wall_color, wall_color + vec3<f32>(0.08, 0.05, 0.04), panel * 0.22);
            wall_color = mix(wall_color, vec3<f32>(0.34, 0.16, 0.10), base_strip * 0.55);
            wall_color = mix(wall_color, vec3<f32>(1.0, 0.82, 0.52), top_gloss * 0.45);

            if (in.frag_pos.y < draw_start + 2.0 || in.frag_pos.y > draw_end - 2.0) {
                let edge_color = vec3<f32>(1.0, 0.58, 0.18);
                return vec4<f32>(edge_color * clamp(depth_shading, 0.0, 1.0), 1.0);
            }
            return vec4<f32>(wall_color, 1.0);
        } else if (in.frag_pos.y < draw_start) {
            // Ceiling
            let dist = (in.rect_params.w * 0.39) / (y_center - in.frag_pos.y);
            let x_3d = posX + dirX * dist + planeX * cameraX * dist;
            let y_3d = posY + dirY * dist + planeY * cameraX * dist;
            let grid_x = fract(x_3d);
            let grid_y = fract(y_3d);
            let arch = 1.0 - smoothstep(0.2, 0.95, abs(col_ratio));
            let ribbing = smoothstep(0.06, 0.0, abs(fract(x_3d * 0.7 + y_3d * 0.7) - 0.5));
            if (grid_x < 0.035 || grid_y < 0.035) {
                let tube_glow = vec3<f32>(0.35, 0.12, 0.34) * (1.0 / (1.0 + dist * 0.1));
                return vec4<f32>(tube_glow, 1.0);
            }
            let ceil_base = mix(vec3<f32>(0.12, 0.08, 0.14), vec3<f32>(0.24, 0.12, 0.20), arch * 0.55);
            let ceil_col = mix(ceil_base, ceil_base + vec3<f32>(0.10, 0.08, 0.05), ribbing * 0.22);
            return vec4<f32>(ceil_col, 1.0);
        } else {
            // Floor
            let dist = (in.rect_params.w * 0.39) / (in.frag_pos.y - y_center);
            let x_3d = posX + dirX * dist + planeX * cameraX * dist;
            let y_3d = posY + dirY * dist + planeY * cameraX * dist;
            let grid_x = fract(x_3d);
            let grid_y = fract(y_3d);
            let tile = step(0.5, fract((floor(x_3d) + floor(y_3d)) * 0.5));
            var floor_col = mix(vec3<f32>(0.22, 0.12, 0.20), vec3<f32>(0.30, 0.18, 0.16), tile * 0.35);
            let run_lane = smoothstep(0.16, 0.0, abs(fract(x_3d * 0.5) - 0.5));
            let dust_motes = smoothstep(0.94, 1.0, fract(sin(dot(vec2<f32>(floor(x_3d * 3.0), floor(y_3d * 3.0)), vec2<f32>(12.9898, 78.233))) * 43758.5453));
            if (grid_x < 0.035 || grid_y < 0.035) {
                let floor_glow = vec3<f32>(0.75, 0.32, 0.12) * (1.0 / (1.0 + dist * 0.1));
                return vec4<f32>(floor_glow, 1.0);
            }
            floor_col = mix(floor_col, floor_col + vec3<f32>(0.08, 0.05, 0.02), run_lane * 0.18);
            floor_col = mix(floor_col, vec3<f32>(0.75, 0.66, 0.42), dust_motes * 0.08);
            return vec4<f32>(floor_col, 1.0);
        }
    }

    if (in.radius == -998.0 || in.radius == -997.0 || in.radius == -996.0 || in.radius == -995.0 || in.radius == -994.0 || in.radius == -993.0 || in.radius == -992.0 || in.radius == -991.0) {
        // Z-BUFFER RAYCAST DEPTH CHECK FOR SPRITES!
        let posX = in.uv.x;
        let posY = in.uv.y;
        let angle = in.shadow_softness;
        
        let scale_factor = in.clip_position.x / max(in.frag_pos.x, 0.001);
        let logical_screen_width = globals.screen_size.x / scale_factor;
        let col_ratio = (in.frag_pos.x - 12.0) / (logical_screen_width - 24.0);
        let cameraX = 2.0 * col_ratio - 1.0;
        
        let dirX = cos(angle);
        let dirY = sin(angle);
        let planeX = -dirY * 0.66;
        let planeY = dirX * 0.66;

        let rayDirX = dirX + planeX * cameraX;
        let rayDirY = dirY + planeY * cameraX;
        
        var mapX = i32(posX);
        var mapY = i32(posY);

        var deltaDistX = 1e30;
        if (rayDirX != 0.0) {
            deltaDistX = abs(1.0 / rayDirX);
        }
        var deltaDistY = 1e30;
        if (rayDirY != 0.0) {
            deltaDistY = abs(1.0 / rayDirY);
        }

        var stepX = 0;
        var sideDistX = 0.0;
        if (rayDirX < 0.0) {
            stepX = -1;
            sideDistX = (posX - f32(mapX)) * deltaDistX;
        } else {
            stepX = 1;
            sideDistX = (f32(mapX) + 1.0 - posX) * deltaDistX;
        }

        var stepY = 0;
        var sideDistY = 0.0;
        if (rayDirY < 0.0) {
            stepY = -1;
            sideDistY = (posY - f32(mapY)) * deltaDistY;
        } else {
            stepY = 1;
            sideDistY = (f32(mapY) + 1.0 - posY) * deltaDistY;
        }

        var hit = 0u;
        var side = 0;
        var maxSteps = 40;
        for (var step = 0; step < maxSteps; step = step + 1) {
            if (hit > 0u) {
                break;
            }
            if (sideDistX < sideDistY) {
                sideDistX = sideDistX + deltaDistX;
                mapX = mapX + stepX;
                side = 0;
            } else {
                sideDistY = sideDistY + deltaDistY;
                mapY = mapY + stepY;
                side = 1;
            }
            
            if (mapX < 0 || mapX >= 64 || mapY < 0 || mapY >= 64) {
                break;
            }
            
            let cell = get_grid_val(mapX, mapY);
            if (cell == 1u) {
                hit = cell;
            }
        }
        
        var perpWallDist = 0.0;
        if (side == 0) {
            perpWallDist = (f32(mapX) - posX + (1.0 - f32(stepX)) / 2.0) / rayDirX;
        } else {
            perpWallDist = (f32(mapY) - posY + (1.0 - f32(stepY)) / 2.0) / rayDirY;
        }

        if (perpWallDist <= 0.05) {
            perpWallDist = 0.05;
        }
        
        // If the sprite's depth is behind the wall, clip the fragment!
        if (in.padding > perpWallDist) {
            discard;
        }
        
        // Procedural Sprite Drawing
        let p = (in.frag_pos - (in.rect_params.xy + in.rect_params.zw * 0.5)) / (in.rect_params.zw * 0.5);
        if (in.radius == -998.0) {
            // Sunflower seed.
            let d = length(p);
            if (d > 0.8) {
                discard;
            }
            let normal = vec3<f32>(p.xy, sqrt(1.0 - clamp(dot(p.xy, p.xy), 0.0, 1.0)));
            let light_dir = normalize(vec3<f32>(0.5, 0.5, 1.0));
            let diffuse = max(dot(normal, light_dir), 0.0);
            let ambient = 0.3;
            let final_col = vec3<f32>(1.0, 0.8, 0.0) * (diffuse + ambient);
            return vec4<f32>(final_col, 1.0);
        } else if (in.radius == -997.0) {
            // Robotic vacuum hazard.
            let d = length(p);
            if (d > 0.9) {
                discard;
            }
            var body_color = in.color.rgb;
            let ring = abs(d - 0.7);
            if (ring < 0.05) {
                body_color = body_color * 0.5;
            }
            
            let normal = vec3<f32>(p.xy, sqrt(1.0 - clamp(dot(p.xy, p.xy), 0.0, 1.0)));
            let light_dir = normalize(vec3<f32>(0.3, 0.7, 0.9));
            let diffuse = max(dot(normal, light_dir), 0.0);
            let specular = pow(max(dot(reflect(-light_dir, normal), vec3<f32>(0.0, 0.0, 1.0)), 0.0), 8.0);
            
            var final_col = body_color * (diffuse + 0.2) + vec3<f32>(specular * 0.4);
            
            // Flashing red siren on top center of disk
            if (length(p - vec2<f32>(0.0, -0.3)) < 0.2) {
                final_col = vec3<f32>(1.0, 0.0, 0.0);
            }
            return vec4<f32>(final_col, 1.0);
        } else if (in.radius == -996.0) {
            // Snack stash jar with a bright lid and seed glow.
            if (abs(p.x) > 0.62 || p.y < -0.78 || p.y > 0.82) {
                discard;
            }
            var jar_col = vec3<f32>(1.0, 0.72, 0.12);
            if (p.y < -0.48) {
                jar_col = vec3<f32>(0.95, 0.20, 0.32);
            }
            let glass_edge = smoothstep(0.46, 0.62, abs(p.x));
            let shine = smoothstep(0.18, 0.0, abs(p.x + 0.25)) * smoothstep(0.62, -0.2, p.y);
            jar_col = mix(jar_col, vec3<f32>(1.0, 0.95, 0.55), shine * 0.55);
            jar_col = mix(jar_col, vec3<f32>(1.0, 0.45, 0.08), glass_edge * 0.35);
            return vec4<f32>(jar_col, 0.96);
        } else if (in.radius == -995.0) {
            // Floating route cue chevron.
            let chevron = abs(abs(p.x) - (0.22 + p.y * 0.42));
            if (p.y < -0.55 || p.y > 0.55 || chevron > 0.16) {
                discard;
            }
            let pulse_col = mix(in.color.rgb, vec3<f32>(1.0, 0.95, 0.2), 0.35);
            return vec4<f32>(pulse_col, 0.85);
        } else if (in.radius == -994.0) {
            // Plastic tube ring joint.
            let d = length(p);
            if (d < 0.45 || d > 0.95) {
                discard;
            }
            let band = smoothstep(0.02, 0.0, abs(d - 0.70));
            let tube_col = mix(vec3<f32>(0.96, 0.48, 0.20), vec3<f32>(1.0, 0.84, 0.60), band * 0.65);
            return vec4<f32>(tube_col, 0.92);
        } else if (in.radius == -993.0) {
            // Soft nest bedding mound.
            let mound = (p.x * p.x) / 0.90 + ((p.y + 0.18) * (p.y + 0.18)) / 0.46;
            if (mound > 1.0 || p.y > 0.55) {
                discard;
            }
            let seedleck = smoothstep(0.94, 1.0, fract(sin(dot(vec2<f32>(floor((p.x + 1.0) * 5.0), floor((p.y + 1.0) * 5.0)), vec2<f32>(91.77, 21.13))) * 12511.331));
            var nest_col = vec3<f32>(0.83, 0.66, 0.34);
            nest_col = mix(nest_col, vec3<f32>(0.96, 0.84, 0.52), seedleck * 0.25);
            return vec4<f32>(nest_col, 0.95);
        } else if (in.radius == -992.0) {
            // Seed bowl.
            let rim = abs(length(vec2<f32>(p.x, p.y + 0.12)) - 0.62);
            if (p.y > 0.46 || abs(p.x) > 0.74 || ((p.y + 0.12) > 0.0 && rim > 0.12)) {
                discard;
            }
            var bowl_col = vec3<f32>(0.20, 0.78, 0.96);
            if (p.y < -0.05) {
                bowl_col = vec3<f32>(1.0, 0.82, 0.18);
            }
            return vec4<f32>(bowl_col, 0.96);
        } else {
            // Exercise wheel gate.
            let ring = abs(length(p) - 0.76);
            let spoke = smoothstep(0.05, 0.0, abs(p.x)) + smoothstep(0.05, 0.0, abs(p.y));
            if (ring > 0.10 && spoke < 0.9) {
                discard;
            }
            let gate_col = mix(vec3<f32>(0.92, 0.26, 0.30), vec3<f32>(1.0, 0.82, 0.22), spoke * 0.45);
            return vec4<f32>(gate_col, 0.90);
        }
    }

    if (in.draw_type < 0.5) { // SHAPE
        // Direct Bypass for flat filled rectangles to optimize performance and prevent GPU compiler quirks
        if (in.radius <= 0.0 && in.glow <= 0.0 && in.shadow_softness <= 0.0 && in.is_glass <= 0.0) {
            return in.color;
        }

        let b = in.rect_params.zw * 0.5;
        let center = in.rect_params.xy + b;
        let p = in.frag_pos - center;
        let d = sd_rounded_rect(p, b, in.radius);
        
        // Shadow Calculation
        var shadow_alpha = 0.0;
        let softness = max(in.shadow_softness, 0.001);
        if (in.shadow_softness > 0.0) {
            let shadow_p = in.frag_pos - (center + in.shadow_offset);
            let shadow_d = sd_rounded_rect(shadow_p, b, in.radius);
            shadow_alpha = (1.0 - smoothstep(-softness, softness, shadow_d)) * 0.6;
        }
        
        let shape_alpha = 1.0 - smoothstep(-1.0, 1.0, d);
        let glow_alpha = exp(-max(0.0, d) * (10.0 - in.glow)) * (in.glow / 10.0);
        let final_alpha = max(shape_alpha, glow_alpha);
        
        var base_color = in.color.rgb;
        if (in.is_glass > 0.5) {
            let screen_uv = in.clip_position.xy / globals.screen_size;
            let blurred = textureSample(blurred_bg, text_sampler, screen_uv).rgb;
            let final_color = mix(blurred, base_color, in.color.a);
            return mix(vec4<f32>(0.0, 0.0, 0.0, shadow_alpha), vec4<f32>(final_color, 1.0), shape_alpha);
        } else {
            let shadow_col = vec4<f32>(0.0, 0.0, 0.0, shadow_alpha * 0.8);
            let shape_col = vec4<f32>(base_color, in.color.a * final_alpha);
            return mix(shadow_col, shape_col, shape_alpha);
        }
    } else { // TEXT
        let atlas_size = vec2<f32>(textureDimensions(text_atlas));
        let atlas_coord = vec2<i32>(clamp(in.uv * atlas_size, vec2<f32>(0.0, 0.0), atlas_size - vec2<f32>(1.0, 1.0)));
        let alpha = textureLoad(text_atlas, atlas_coord, 0).r;
        return vec4<f32>(in.color.rgb, in.color.a * alpha);
    }
}
