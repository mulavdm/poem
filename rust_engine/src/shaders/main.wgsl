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
}

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) frag_pos: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) color: vec4<f32>,
    @location(3) rect_params: vec4<f32>,
    @location(4) radius: f32,
    @location(5) draw_type: f32,
    @location(6) glow: f32,
    @location(7) is_glass: f32,
    @location(8) shadow_offset: vec2<f32>,
    @location(9) shadow_softness: f32,
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
    return out;
}

fn sd_rounded_rect(p: vec2<f32>, b: vec2<f32>, r: f32) -> f32 {
    let q = abs(p) - b + vec2<f32>(r, r);
    return length(max(q, vec2<f32>(0.0))) + min(max(q.x, q.y), 0.0) - r;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
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
