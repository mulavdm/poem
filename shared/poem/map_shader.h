#pragma once

// Canonical GPU map shader maths — the single source of truth for projecting,
// lighting and fogging map geometry on the GPU, shared by every backend.
//
// The projection here mirrors poem::mapview in map_view.h (which the CPU side
// and the tests use); this is the GPU transcription of the same model, kept in
// one place so D3D11, GLES and a future WebGL/WebGPU backend cannot drift from
// each other (framework TDD §15.3 one canonical shader source, §40.3 backend
// divergence).
//
// Only the *entry points* are per-backend: HLSL needs structs with semantics,
// GLSL needs attribute/varying declarations, and each declares its uniforms
// differently. Those are a dozen lines each and call straight into the shared
// functions below, which take everything explicitly and touch no globals.

namespace poem::mapshader {

// Dialect preludes map the generic spellings used by kBody onto each language.
// Names that are already identical in both (max, exp, dot, cross, length, abs)
// are used directly.
inline constexpr const char* kHlslPrelude = R"SHADER(
#define FLOAT float
#define FLOAT3 float3
#define FLOAT4 float4
#define MAKE_FLOAT3 float3
#define MAKE_FLOAT4 float4
#define LERP lerp
#define SATURATE saturate
#define DEPTH_TO_CLIP(n) (n)
)SHADER";

// GLES clip space is [-1,1] in z where D3D is [0,1], so the depth mapping
// differs; everything else is a spelling change.
inline constexpr const char* kGlslPrelude = R"SHADER(
#define FLOAT float
#define FLOAT3 vec3
#define FLOAT4 vec4
#define MAKE_FLOAT3 vec3
#define MAKE_FLOAT4 vec4
#define LERP mix
#define SATURATE(x) clamp((x), 0.0, 1.0)
#define DEPTH_TO_CLIP(n) ((n) * 2.0 - 1.0)
)SHADER";

// kBody is the shared maths. Uniform *values* arrive as explicit parameters so
// this text is independent of how each backend declares its constant storage.
//
//   center   = camera centre, split hi/lo (world units near zoom 15 exceed
//              single-float precision when differenced in-shader)
//   world    = worldPixels, cos/sin bearing, metersToPixels
//   pitch    = pitchCos, pitchSin, cameraDistance
//   rect     = viewport centre x/y, inset x/y
//   screen   = screen width/height
inline constexpr const char* kBody = R"SHADER(
// MapLocal converts a world vertex (mercator x/y, elevation metres) into the
// camera-local pixel space the projection is built from: +x right after
// bearing, +y south, +z up. The wrap keeps geometry near the camera across the
// antimeridian instead of flinging it a world away.
FLOAT3 MapLocal(FLOAT3 pos, FLOAT4 center, FLOAT4 world) {
    FLOAT dx = (pos.x - center.x) - center.z;
    if (dx > 0.5) dx -= 1.0;
    if (dx < -0.5) dx += 1.0;
    FLOAT dy = (pos.y - center.y) - center.w;
    FLOAT localX = (dx * world.y - dy * world.z) * world.x;
    FLOAT localY = (dx * world.z + dy * world.y) * world.x;
    FLOAT localZ = pos.z * world.w;
    return MAKE_FLOAT3(localX, localY, localZ);
}

// MapDepth is camera-space depth: it grows toward the camera, matching
// mapview::Depth so CPU and GPU agree on what is in front.
FLOAT MapDepth(FLOAT3 local, FLOAT4 pitch) {
    return local.y * pitch.y + local.z * pitch.x;
}

// MapClip projects camera-local space to clip space. Depth is normalised over
// the full range the perspective divide can produce, [-19d, +d], so precision
// is spent on real separation rather than saturating into z-ties.
FLOAT4 MapClip(FLOAT3 local, FLOAT4 pitch, FLOAT4 rect, FLOAT4 screen) {
    FLOAT projY = local.y * pitch.x - local.z * pitch.y;
    FLOAT depth = MapDepth(local, pitch);
    FLOAT persp = pitch.z / max(pitch.z * 0.05, pitch.z - depth);
    FLOAT sx = rect.x + local.x * persp + rect.z;
    FLOAT sy = rect.y + projY * persp + rect.w;
    FLOAT nearness = SATURATE((depth + 19.0 * pitch.z) / (20.0 * pitch.z));
    return MAKE_FLOAT4(sx / screen.x * 2.0 - 1.0,
                       1.0 - sy / screen.y * 2.0,
                       DEPTH_TO_CLIP(nearness), 1.0);
}

// MapShade lights a face against the sun. The caller passes screen-space
// derivatives of the interpolated local position: their cross product is the
// true face plane, so no per-vertex normals are needed. abs() lights a face and
// its back identically, which keeps shared walls from flickering.
FLOAT3 MapShade(FLOAT3 color, FLOAT3 dpdx, FLOAT3 dpdy, FLOAT4 sunAmbient) {
    FLOAT3 normal = cross(dpdx, dpdy);
    FLOAT len = length(normal);
    if (len > 0.000001) {
        normal = normal / len;
        FLOAT lambert = abs(dot(normal, sunAmbient.xyz));
        color = color * (sunAmbient.w + (1.0 - sunAmbient.w) * lambert);
    }
    return color;
}

// MapFog is height fog: haze grows with ground distance, thins with altitude so
// towers rise out of it, and fades in with pitch so a top-down map is clear.
//   fog       = colour rgb + density
//   fogParams = referenceMetres, scaleHeight, pitchSin, metersToPixels
FLOAT3 MapFog(FLOAT3 color, FLOAT3 local, FLOAT4 fog, FLOAT4 fogParams) {
    if (fog.w <= 0.0 || fogParams.z <= 0.0) return color;
    FLOAT meters = length(local.xy) / fogParams.w;
    FLOAT elevation = local.z / fogParams.w;
    FLOAT altitude = exp(-max(elevation, 0.0) / fogParams.y);
    FLOAT amount = 1.0 - exp(-fog.w * (meters / fogParams.x) * altitude * fogParams.z);
    return LERP(color, fog.xyz, SATURATE(amount));
}
)SHADER";

} // namespace poem::mapshader
