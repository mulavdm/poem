#pragma once

// Shared GPU map plumbing: which batches the GPU path owns, and the uniform
// block feeding poem::mapshader. Both live here so no backend re-derives them
// (framework TDD §4.1 shared core, §40.3 backend divergence).

#include "poem/map_view.h"
#include "poem/protocol.h"

#include <algorithm>
#include <cstring>
#include <vector>

namespace poem::mapgpu {

// Uniforms mirrors the vec4 slots poem::mapshader reads. Backends upload this
// verbatim (as a cbuffer or as eight vec4 uniforms) and never repack it.
struct Uniforms {
    float center[4];     // hiX hiY loX loY
    float world[4];      // worldPixels cosBearing sinBearing metersToPixels
    float pitch[4];      // pitchCos pitchSin cameraDistance -
    float rect[4];       // viewportCentreX viewportCentreY insetX insetY
    float screen[4];     // screenWidth screenHeight - -
    float sunAmbient[4]; // sunX sunY sunZ ambient
    float fog[4];        // r g b density
    float fogParams[4];  // referenceMetres scaleHeight pitchSin metersToPixels
};

// Lighting is the scene's solar/fog state, straight off the retained scene.
struct Lighting {
    float sunAzimuth = 315.0f;
    float sunElevation = 45.0f;
    float fogDensity = 0.0f;
    float fogRed = 1.0f, fogGreen = 1.0f, fogBlue = 1.0f;
};

// MakeUniforms packs a view plus scene lighting into the shader's slots. The
// camera centre is split hi/lo because mercator coordinates at city zoom lose
// the metre-scale bits when differenced in single precision on the GPU.
inline Uniforms MakeUniforms(const mapview::View& view, const Lighting& lighting,
                             float screenWidth, float screenHeight,
                             float insetX, float insetY) {
    const auto sun = mapview::SunDirection(view, lighting.sunAzimuth, lighting.sunElevation);
    const float centerHiX = static_cast<float>(view.centerX);
    const float centerHiY = static_cast<float>(view.centerY);

    Uniforms uniforms{};
    uniforms.center[0] = centerHiX;
    uniforms.center[1] = centerHiY;
    uniforms.center[2] = static_cast<float>(view.centerX - static_cast<double>(centerHiX));
    uniforms.center[3] = static_cast<float>(view.centerY - static_cast<double>(centerHiY));

    uniforms.world[0] = static_cast<float>(view.worldPixels);
    uniforms.world[1] = static_cast<float>(view.cosBearing);
    uniforms.world[2] = static_cast<float>(view.sinBearing);
    uniforms.world[3] = static_cast<float>(view.metersToPixels);

    uniforms.pitch[0] = static_cast<float>(view.pitchCos);
    uniforms.pitch[1] = static_cast<float>(view.pitchSin);
    uniforms.pitch[2] = static_cast<float>(view.cameraDistance);

    uniforms.rect[0] = static_cast<float>((view.left + view.right) * .5);
    uniforms.rect[1] = static_cast<float>((view.top + view.bottom) * .5);
    uniforms.rect[2] = insetX;
    uniforms.rect[3] = insetY;

    uniforms.screen[0] = screenWidth;
    uniforms.screen[1] = screenHeight;

    uniforms.sunAmbient[0] = static_cast<float>(sun.x);
    uniforms.sunAmbient[1] = static_cast<float>(sun.y);
    uniforms.sunAmbient[2] = static_cast<float>(sun.z);
    uniforms.sunAmbient[3] = static_cast<float>(mapview::kDefaultAmbient);

    uniforms.fog[0] = lighting.fogRed;
    uniforms.fog[1] = lighting.fogGreen;
    uniforms.fog[2] = lighting.fogBlue;
    uniforms.fog[3] = lighting.fogDensity;

    uniforms.fogParams[0] = static_cast<float>(mapview::kFogReferenceMeters);
    uniforms.fogParams[1] = static_cast<float>(mapview::kFogScaleHeightMeters);
    uniforms.fogParams[2] = static_cast<float>(view.pitchSin);
    uniforms.fogParams[3] = static_cast<float>(view.metersToPixels);
    return uniforms;
}

// IsGpuBatch reports whether the GPU map path owns a batch: untextured
// triangles, i.e. ground fills and building extrusions. Dots, lines and
// textured label quads stay on the composited overlay.
inline bool IsGpuBatch(const protocol::MapDrawBatch& draw) {
    if (draw.primitive != protocol::MapPrimitive::Triangles) return false;
    return std::none_of(draw.textureHash.begin(), draw.textureHash.end(),
                        [](std::uint8_t value) { return value != 0; });
}

// HasGpuBatches avoids binding the map pipeline for a scene with nothing in it.
inline bool HasGpuBatches(const std::vector<protocol::MapDrawBatch>& draws) {
    return std::any_of(draws.begin(), draws.end(), IsGpuBatch);
}

// Cartography vertex layout (stride 32): float3 position, RGBA8 colour, float
// width, uint64 feature id, pad. Ground/extrusion batches read only position
// and colour; markers also need width, which carries their radius.
inline constexpr unsigned int kVertexStride = 32;
inline constexpr unsigned int kColorOffset = 12;
inline constexpr unsigned int kWidthOffset = 16;

struct SourceVertex {
    float x = 0, y = 0, z = 0;
    std::uint8_t rgba[4] = {0, 0, 0, 0};
    float width = 0;
};

// ReadSourceVertex decodes one cartography vertex. Returns false when the index
// falls outside the buffer, so malformed input cannot read out of bounds.
inline bool ReadSourceVertex(const std::vector<std::uint8_t>& bytes, std::uint32_t index, SourceVertex& out) {
    const std::size_t offset = static_cast<std::size_t>(index) * kVertexStride;
    if (offset + kVertexStride > bytes.size()) return false;
    std::memcpy(&out.x, bytes.data() + offset, 4);
    std::memcpy(&out.y, bytes.data() + offset + 4, 4);
    std::memcpy(&out.z, bytes.data() + offset + 8, 4);
    std::memcpy(out.rgba, bytes.data() + offset + kColorOffset, 4);
    std::memcpy(&out.width, bytes.data() + offset + kWidthOffset, 4);
    return true;
}

// MarkerVertex is one corner of a depth-tested billboard. The world position is
// projected on the GPU (so the marker occludes against buildings) and the corner
// is then offset in screen space, keeping markers a constant on-screen size.
struct MarkerVertex {
    float x = 0, y = 0, z = 0;       // world position (mercator x/y, elevation m)
    float cornerX = 0, cornerY = 0;  // unit quad corner, [-1,1]
    float radius = 0;                // screen pixels
    std::uint8_t rgba[4] = {0, 0, 0, 0};
    float pad = 0;                   // keep a 32-byte stride
};

inline constexpr unsigned int kMarkerStride = 32;
inline constexpr float kMinMarkerRadius = 3.0f;

// IsMarkerBatch reports whether a batch is untextured points, i.e. POI markers.
inline bool IsMarkerBatch(const protocol::MapDrawBatch& draw) {
    if (draw.primitive != protocol::MapPrimitive::Points) return false;
    return std::none_of(draw.textureHash.begin(), draw.textureHash.end(),
                        [](std::uint8_t value) { return value != 0; });
}

inline bool HasMarkerBatches(const std::vector<protocol::MapDrawBatch>& draws) {
    return std::any_of(draws.begin(), draws.end(), IsMarkerBatch);
}

// AppendMarkerVertices expands a point batch into two triangles per marker.
// Expansion happens here, once, rather than in each presenter.
inline void AppendMarkerVertices(std::vector<MarkerVertex>& out,
                                 const protocol::MapDrawBatch& draw,
                                 const std::vector<std::uint8_t>& vertexBytes,
                                 const std::vector<std::uint8_t>& indexBytes) {
    // Two triangles over the unit quad.
    static constexpr float kCornerX[6] = {-1, 1, -1, -1, 1, 1};
    static constexpr float kCornerY[6] = {-1, -1, 1, 1, -1, 1};
    for (std::uint32_t i = 0; i < draw.count; ++i) {
        const std::size_t indexOffset = (static_cast<std::size_t>(draw.first) + i) * 4;
        if (indexOffset + 4 > indexBytes.size()) break;
        std::uint32_t vertexIndex = 0;
        std::memcpy(&vertexIndex, indexBytes.data() + indexOffset, 4);
        SourceVertex source;
        if (!ReadSourceVertex(vertexBytes, vertexIndex, source)) continue;
        const float radius = std::max(kMinMarkerRadius, source.width);
        for (int corner = 0; corner < 6; ++corner) {
            MarkerVertex vertex;
            vertex.x = source.x;
            vertex.y = source.y;
            vertex.z = source.z;
            vertex.cornerX = kCornerX[corner];
            vertex.cornerY = kCornerY[corner];
            vertex.radius = radius;
            std::memcpy(vertex.rgba, source.rgba, 4);
            out.push_back(vertex);
        }
    }
}

} // namespace poem::mapgpu
