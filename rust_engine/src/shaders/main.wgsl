struct Globals {
    projection: mat4x4<f32>,
    screen_size: vec2<f32>,
}

@group(0) @binding(0) var<uniform> globals: Globals;
@group(0) @binding(1) var text_atlas: texture_2d<f32>;
@group(0) @binding(2) var text_sampler: sampler;
@group(0) @binding(3) var blurred_bg: texture_2d<f32>;

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
    if (x == 0 || y == 0 || x == 63 || y == 63) {
        return 1u;
    }
    if (x == 35 && y == 15) {
        return 2u; // Vault!
    }
    if (x % 4 == 0 && y % 4 == 0) {
        if (x < 8 && y < 8) {
            return 0u;
        }
        return 1u;
    }
    if ((x > 10 && x < 54 && y == 32) || (y > 10 && y < 54 && x == 32)) {
        return 1u;
    }
    return 0u;
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
            if (cell > 0u) {
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
        
        if (in.frag_pos.y >= draw_start && in.frag_pos.y <= draw_end) {
            // Wall
            var wall_color = vec3<f32>(0.0, 0.78, 1.0); // Neon cyan
            if (hit == 2u) {
                wall_color = vec3<f32>(0.2, 1.0, 0.0); // Neon green
            }
            if (side == 1) {
                wall_color = wall_color * 0.7; // Side shading
            }
            let depth_shading = 1.5 / (1.0 + perpWallDist * 0.08);
            wall_color = wall_color * clamp(depth_shading, 0.0, 1.0);

            // Edge highlights
            if (in.frag_pos.y < draw_start + 2.0 || in.frag_pos.y > draw_end - 2.0) {
                let edge_color = vec3<f32>(1.0, 0.0, 0.7); // Neon pink
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
            if (grid_x < 0.04 || grid_y < 0.04) {
                let ceil_glow = vec3<f32>(0.0, 0.4, 0.3) * (1.0 / (1.0 + dist * 0.1));
                return vec4<f32>(ceil_glow, 1.0);
            }
            return vec4<f32>(15.0/255.0, 10.0/255.0, 30.0/255.0, 1.0);
        } else {
            // Floor
            let dist = (in.rect_params.w * 0.39) / (in.frag_pos.y - y_center);
            let x_3d = posX + dirX * dist + planeX * cameraX * dist;
            let y_3d = posY + dirY * dist + planeY * cameraX * dist;
            let grid_x = fract(x_3d);
            let grid_y = fract(y_3d);
            if (grid_x < 0.04 || grid_y < 0.04) {
                let floor_glow = vec3<f32>(0.6, 0.0, 0.4) * (1.0 / (1.0 + dist * 0.1));
                return vec4<f32>(floor_glow, 1.0);
            }
            return vec4<f32>(30.0/255.0, 20.0/255.0, 45.0/255.0, 1.0);
        }
    }

    if (in.radius == -998.0 || in.radius == -997.0) {
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
            if (cell > 0u) {
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
            // SUNFLOWER SEEDS! Beautiful normal-mapped golden spheres
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
        } else {
            // ROBOTIC VACUUM ENEMIES! Sleek metallic disk with siren
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
        let alpha = textureSample(text_atlas, text_sampler, in.uv).r;
        return vec4<f32>(in.color.rgb, in.color.a * alpha);
    }
}
