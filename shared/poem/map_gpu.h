#pragma once

// Shared GPU map plumbing: which batches the GPU path owns, and the uniform
// block feeding poem::mapshader. Both live here so no backend re-derives them
// (framework TDD §4.1 shared core, §40.3 backend divergence).

#include "poem/map_view.h"
#include "poem/protocol.h"

#include <algorithm>
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

// kVertexStride is the cartography vertex stride: float3 position + RGBA8
// colour, then width and feature id which the GPU path does not read.
inline constexpr unsigned int kVertexStride = 32;
inline constexpr unsigned int kColorOffset = 12;

} // namespace poem::mapgpu
