@group(0) @binding(0) var image_tex: texture_2d<f32>;
@group(0) @binding(1) var image_sampler: sampler;

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) tex_coords: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) in_vertex_index: u32) -> VertexOutput {
    var out: VertexOutput;
    var pos = vec2<f32>(0.0, 0.0);
    var uv = vec2<f32>(0.0, 0.0);
    if (in_vertex_index == 0u) {
        pos = vec2<f32>(-1.0, 1.0);
        uv = vec2<f32>(0.0, 0.0);
    } else if (in_vertex_index == 1u) {
        pos = vec2<f32>(-1.0, -1.0);
        uv = vec2<f32>(0.0, 1.0);
    } else if (in_vertex_index == 2u) {
        pos = vec2<f32>(1.0, 1.0);
        uv = vec2<f32>(1.0, 0.0);
    } else {
        pos = vec2<f32>(1.0, -1.0);
        uv = vec2<f32>(1.0, 1.0);
    }
    out.clip_position = vec4<f32>(pos, 0.0, 1.0);
    out.tex_coords = uv;
    return out;
}

@fragment
fn fs_horizontal(in: VertexOutput) -> @location(0) vec4<f32> {
    let size = textureDimensions(image_tex, 0);
    let tex_offset = 1.0 / vec2<f32>(size);
    
    var result = textureSample(image_tex, image_sampler, in.tex_coords).rgb * 0.227027;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(tex_offset.x * 1.0, 0.0)).rgb * 0.1945946;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(tex_offset.x * 1.0, 0.0)).rgb * 0.1945946;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(tex_offset.x * 2.0, 0.0)).rgb * 0.1216216;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(tex_offset.x * 2.0, 0.0)).rgb * 0.1216216;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(tex_offset.x * 3.0, 0.0)).rgb * 0.054054;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(tex_offset.x * 3.0, 0.0)).rgb * 0.054054;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(tex_offset.x * 4.0, 0.0)).rgb * 0.016216;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(tex_offset.x * 4.0, 0.0)).rgb * 0.016216;
    
    return vec4<f32>(result, 1.0);
}

@fragment
fn fs_vertical(in: VertexOutput) -> @location(0) vec4<f32> {
    let size = textureDimensions(image_tex, 0);
    let tex_offset = 1.0 / vec2<f32>(size);
    
    var result = textureSample(image_tex, image_sampler, in.tex_coords).rgb * 0.227027;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(0.0, tex_offset.y * 1.0)).rgb * 0.1945946;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(0.0, tex_offset.y * 1.0)).rgb * 0.1945946;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(0.0, tex_offset.y * 2.0)).rgb * 0.1216216;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(0.0, tex_offset.y * 2.0)).rgb * 0.1216216;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(0.0, tex_offset.y * 3.0)).rgb * 0.054054;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(0.0, tex_offset.y * 3.0)).rgb * 0.054054;
    result += textureSample(image_tex, image_sampler, in.tex_coords + vec2<f32>(0.0, tex_offset.y * 4.0)).rgb * 0.016216;
    result += textureSample(image_tex, image_sampler, in.tex_coords - vec2<f32>(0.0, tex_offset.y * 4.0)).rgb * 0.016216;
    
    return vec4<f32>(result, 1.0);
}
