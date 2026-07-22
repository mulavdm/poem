// Contract tests for the shared map view maths. These run on desktop CI with no
// GPU, window or platform SDK, which is the point: every presenter projects and
// lights the scene through this one implementation, so pinning it here pins all
// of them (framework TDD §30.1 shared unit tests, §40.3 backend divergence).

#include "poem/map_gpu.h"
#include "poem/map_view.h"

#include <algorithm>
#include <cmath>
#include <cstring>
#include <vector>
#include <cstdio>
#include <stdexcept>
#include <string>

namespace {

using namespace poem::mapview;

void Require(bool condition, const std::string& what) {
    if (!condition) throw std::runtime_error("map view assertion failed: " + what);
}

bool Near(double value, double expected, double tolerance) {
    return std::fabs(value - expected) <= tolerance;
}

// A north-up, unpitched view over Amsterdam filling an 800x600 viewport.
View FlatView() {
    return Make(52.3676, 4.9041, 14, /*bearing*/ 0, /*pitch*/ 0, /*previewScale*/ 1,
                /*left*/ 0, /*top*/ 0, /*right*/ 800, /*bottom*/ 600);
}


// MapShadowFactorReference mirrors poem::mapshader's MapShadowFactor so the
// shadow decision is pinned by a test even though the shader runs on the GPU.
// Keep in step with the shared shader source.
float MapShadowFactorReference(float storedDepth, float fragmentDepth, float bias, float strength) {
    if (fragmentDepth > 1.0f || fragmentDepth < 0.0f) return 1.0f;
    const float lit = fragmentDepth - bias <= storedDepth ? 1.0f : 0.0f;
    return 1.0f - strength * (1.0f - lit);
}

void ProjectsCameraToViewportCentre() {
    const auto view = FlatView();
    const auto point = Project(view, view.centerX, view.centerY, 0);
    Require(Near(point.x, 400, 0.5), "camera projects to viewport centre x");
    Require(Near(point.y, 300, 0.5), "camera projects to viewport centre y");
}

void HorizontalWrapTakesShortestPath() {
    // A point just across the antimeridian must project near the camera, not a
    // world away: the ±.5 wrap is what keeps geometry from flying off screen.
    auto view = Make(0, 179.9, 6, 0, 0, 1, 0, 0, 800, 600);
    const auto wrapped = Project(view, /*x just past 180deg*/ 0.0002, view.centerY, 0);
    Require(std::fabs(wrapped.x - 400) < 800, "antimeridian wrap stays near the camera");
}

void ElevationRaisesAndNears() {
    const auto view = Make(52.3676, 4.9041, 16, 0, 55, 1, 0, 0, 800, 600);
    const auto ground = Project(view, view.centerX, view.centerY, 0);
    const auto roof = Project(view, view.centerX, view.centerY, 40);
    Require(roof.y < ground.y, "a raised point draws higher on screen when pitched");
    Require(Depth(view, view.centerX, view.centerY, 40) > Depth(view, view.centerX, view.centerY, 0),
            "depth grows toward the camera with elevation");
}

void SunDirectionIsUnitAndOriented() {
    const auto view = FlatView();
    const auto sun = SunDirection(view, /*azimuth*/ 0, /*elevation*/ 0);
    Require(Near(std::sqrt(sun.x * sun.x + sun.y * sun.y + sun.z * sun.z), 1.0, 1e-9), "sun vector is unit length");
    // Due north at the horizon: y grows southward, so north is -y.
    Require(Near(sun.x, 0, 1e-9), "north sun has no east component");
    Require(sun.y < 0, "north sun points toward -y");

    const auto overhead = SunDirection(view, 0, 90);
    Require(Near(overhead.z, 1.0, 1e-9), "an overhead sun points straight up");
}

void ShadingSpansAmbientToFull() {
    const auto view = FlatView();
    const auto overhead = SunDirection(view, 0, 90);
    // A ground-plane (upward normal) triangle under an overhead sun is fully lit.
    const double span = 0.0001;
    const float lit = ShadeTriangle(view, overhead,
                                    view.centerX, view.centerY, 0,
                                    view.centerX + span, view.centerY, 0,
                                    view.centerX, view.centerY + span, 0);
    Require(Near(lit, 1.0f, 1e-5), "flat ground under an overhead sun is fully lit");

    // A vertical wall is edge-on to an overhead sun: ambient only.
    const float wall = ShadeTriangle(view, overhead,
                                     view.centerX, view.centerY, 0,
                                     view.centerX + span, view.centerY, 0,
                                     view.centerX, view.centerY, 30);
    Require(Near(wall, static_cast<float>(kDefaultAmbient), 1e-5), "a wall edge-on to the sun falls back to ambient");

    // Never darker than ambient, never brighter than full, whatever the sun.
    for (double azimuth = 0; azimuth < 360; azimuth += 30) {
        const auto sun = SunDirection(view, azimuth, 20);
        const float shade = ShadeTriangle(view, sun,
                                          view.centerX, view.centerY, 0,
                                          view.centerX + span, view.centerY, 0,
                                          view.centerX, view.centerY, 30);
        Require(shade >= static_cast<float>(kDefaultAmbient) - 1e-5f && shade <= 1.0f + 1e-5f,
                "shading stays within [ambient, 1]");
    }
}

void DegenerateFaceIsUnshaded() {
    const auto view = FlatView();
    const auto sun = SunDirection(view, 180, 30);
    const float shade = ShadeTriangle(view, sun,
                                      view.centerX, view.centerY, 0,
                                      view.centerX, view.centerY, 0,
                                      view.centerX, view.centerY, 0);
    Require(Near(shade, 1.0f, 1e-6), "a zero-area face is left unshaded rather than dividing by zero");
}

void FogGrowsWithDistanceAndThinsWithHeight() {
    const auto flat = FlatView();
    Require(FogFactor(flat, flat.centerX + 0.01, flat.centerY, 0, 1.5) == 0.0f,
            "a top-down view is never hazed");

    const auto pitched = Make(52.3676, 4.9041, 15, 0, 60, 1, 0, 0, 800, 600);
    Require(FogFactor(pitched, pitched.centerX, pitched.centerY, 0, 0) == 0.0f, "density 0 disables fog");

    // Roughly 250 m and 2 km of ground distance. Distances this ordinary must
    // not already be saturated, which a pixel-based density was.
    const float nearHaze = FogFactor(pitched, pitched.centerX + 0.00001, pitched.centerY, 0, 1.5);
    const float farHaze = FogFactor(pitched, pitched.centerX + 0.00008, pitched.centerY, 0, 1.5);
    Require(farHaze > nearHaze, "fog grows with ground distance");
    Require(nearHaze >= 0.0f && farHaze <= 1.0f, "fog stays within [0,1]");
    Require(nearHaze < 0.9f, "nearby ground is not already saturated");

    // Ground vs a tower top at the same place: the tower rises out of the haze.
    const float ground = FogFactor(pitched, pitched.centerX + 0.00008, pitched.centerY, 0, 1.5);
    const float tower = FogFactor(pitched, pitched.centerX + 0.00008, pitched.centerY, 300, 1.5);
    Require(tower < ground, "fog thins with altitude");
}

void FogMixIsABlend() {
    Require(Near(MixFog(0.2f, 0.9f, 0.0f), 0.2, 1e-6), "no fog leaves the colour alone");
    Require(Near(MixFog(0.2f, 0.9f, 1.0f), 0.9, 1e-6), "full fog reaches the haze colour");
    Require(Near(MixFog(0.0f, 1.0f, 0.5f), 0.5, 1e-6), "half fog is halfway");
}

void DepthSortOnlyWhenPitched() {
    Require(!NeedsDepthSort(0), "top-down needs no painter sort");
    Require(!NeedsDepthSort(0.5), "a hair off top-down still needs no sort");
    Require(NeedsDepthSort(45), "a pitched camera needs the painter sort");
}

// The GPU map path derives its uniforms and batch selection from shared code so
// every backend feeds its shaders identically; these pin that contract.
void GpuUniformsMatchTheView() {
    const auto view = Make(52.3676, 4.9041, 15, 30, 55, 1, 0, 0, 800, 600);
    poem::mapgpu::Lighting lighting{};
    lighting.sunAzimuth = 120;
    lighting.sunElevation = 25;
    lighting.fogDensity = 1.15f;
    const auto uniforms = poem::mapgpu::MakeUniforms(view, lighting, 800, 600, 7, 9);

    // The camera centre is split hi/lo; recombining must recover it to double
    // precision, which is the whole reason the split exists.
    const double recombinedX = static_cast<double>(uniforms.center[0]) + static_cast<double>(uniforms.center[2]);
    const double recombinedY = static_cast<double>(uniforms.center[1]) + static_cast<double>(uniforms.center[3]);
    Require(Near(recombinedX, view.centerX, 1e-12), "hi+lo recovers camera centre x");
    Require(Near(recombinedY, view.centerY, 1e-12), "hi+lo recovers camera centre y");

    Require(Near(uniforms.world[0], view.worldPixels, 1.0), "worldPixels forwarded");
    Require(Near(uniforms.pitch[2], view.cameraDistance, 1e-3), "cameraDistance forwarded");
    Require(Near(uniforms.rect[0], (view.left + view.right) * .5, 1e-6), "viewport centre x");
    Require(Near(uniforms.rect[2], 7, 1e-6) && Near(uniforms.rect[3], 9, 1e-6), "insets forwarded");
    Require(Near(uniforms.sunAmbient[3], kDefaultAmbient, 1e-6), "ambient forwarded");

    // Sun must match the shared direction the CPU path uses.
    const auto sun = SunDirection(view, lighting.sunAzimuth, lighting.sunElevation);
    Require(Near(uniforms.sunAmbient[0], sun.x, 1e-6) && Near(uniforms.sunAmbient[1], sun.y, 1e-6) &&
                Near(uniforms.sunAmbient[2], sun.z, 1e-6),
            "sun direction matches the shared calculation");

    // Fog constants come from the shared header, never hard-coded in a shader.
    Require(Near(uniforms.fogParams[0], kFogReferenceMeters, 1e-6), "fog reference metres forwarded");
    Require(Near(uniforms.fogParams[1], kFogScaleHeightMeters, 1e-6), "fog scale height forwarded");
    Require(Near(uniforms.fogParams[2], view.pitchSin, 1e-6), "fog pitch gate forwarded");
}

void GpuBatchSelectionOwnsUntexturedTriangles() {
    poem::protocol::MapDrawBatch ground{};
    ground.primitive = poem::protocol::MapPrimitive::Triangles;
    Require(poem::mapgpu::IsGpuBatch(ground), "untextured triangles are GPU batches");

    poem::protocol::MapDrawBatch label = ground;
    label.textureHash[3] = 7;
    Require(!poem::mapgpu::IsGpuBatch(label), "textured quads stay on the overlay");

    poem::protocol::MapDrawBatch dots{};
    dots.primitive = poem::protocol::MapPrimitive::Points;
    Require(!poem::mapgpu::IsGpuBatch(dots), "points stay on the overlay");

    poem::protocol::MapDrawBatch lines{};
    lines.primitive = poem::protocol::MapPrimitive::Lines;
    Require(!poem::mapgpu::IsGpuBatch(lines), "lines stay on the overlay");

    Require(poem::mapgpu::HasGpuBatches({dots, ground}), "a scene with ground has GPU batches");
    Require(!poem::mapgpu::HasGpuBatches({dots, lines, label}), "an overlay-only scene has none");
}


// Markers are expanded into depth-tested billboards by shared code, so both
// presenters place and size them identically.
void MarkerExpansionBuildsBillboards() {
    // Two cartography vertices (stride 32): float3 pos, RGBA8, float width.
    std::vector<std::uint8_t> vertices(2 * poem::mapgpu::kVertexStride, 0);
    auto writeVertex = [&](std::size_t index, float x, float y, float z, std::uint8_t red, float width, std::uint32_t symbol) {
        const std::size_t base = index * poem::mapgpu::kVertexStride;
        std::memcpy(vertices.data() + base, &x, 4);
        std::memcpy(vertices.data() + base + 4, &y, 4);
        std::memcpy(vertices.data() + base + 8, &z, 4);
        vertices[base + poem::mapgpu::kColorOffset] = red;
        vertices[base + poem::mapgpu::kColorOffset + 3] = 255;
        std::memcpy(vertices.data() + base + poem::mapgpu::kWidthOffset, &width, 4);
        std::memcpy(vertices.data() + base + poem::mapgpu::kSymbolOffset, &symbol, 4);
    };
    writeVertex(0, 0.25f, 0.5f, 0.0f, 200, 8.0f, 4);
    writeVertex(1, 0.30f, 0.6f, 12.0f, 100, 1.0f, 0); // width below the floor

    std::vector<std::uint8_t> indices(2 * 4, 0);
    const std::uint32_t zero = 0, one = 1;
    std::memcpy(indices.data(), &zero, 4);
    std::memcpy(indices.data() + 4, &one, 4);

    poem::protocol::MapDrawBatch draw{};
    draw.primitive = poem::protocol::MapPrimitive::Points;
    draw.count = 2;

    std::vector<poem::mapgpu::MarkerVertex> markers;
    poem::mapgpu::AppendMarkerVertices(markers, draw, vertices, indices);
    Require(markers.size() == 12, "each marker becomes two triangles");

    // World position and colour are carried through; the quad is centred on it.
    Require(Near(markers[0].x, 0.25, 1e-6) && Near(markers[0].y, 0.5, 1e-6),
            "billboard corners keep the marker world position");
    Require(markers[0].rgba[0] == 200, "marker colour is carried through");
    Require(Near(markers[0].radius, 8.0, 1e-6), "radius comes from the vertex width");
    Require(Near(markers[0].symbol, 4.0, 1e-6), "semantic POI symbol is carried through");
    Require(Near(markers[6].radius, poem::mapgpu::kMinMarkerRadius, 1e-6),
            "a too-small width is lifted to the minimum radius");
    Require(Near(markers[6].z, 12.0, 1e-6), "elevation is preserved so raised markers occlude correctly");

    // The six corners must cover the quad, not collapse to a point.
    float minCorner = 1.0f, maxCorner = -1.0f;
    for (int i = 0; i < 6; ++i) {
        minCorner = std::min(minCorner, markers[i].cornerX);
        maxCorner = std::max(maxCorner, markers[i].cornerX);
    }
    Require(Near(minCorner, -1.0, 1e-6) && Near(maxCorner, 1.0, 1e-6), "corners span the unit quad");
}

void MarkerExpansionRejectsOutOfRangeIndices() {
    std::vector<std::uint8_t> vertices(poem::mapgpu::kVertexStride, 0);
    std::vector<std::uint8_t> indices(4, 0);
    const std::uint32_t wild = 9999; // points past the end of the vertex buffer
    std::memcpy(indices.data(), &wild, 4);

    poem::protocol::MapDrawBatch draw{};
    draw.primitive = poem::protocol::MapPrimitive::Points;
    draw.count = 1;

    std::vector<poem::mapgpu::MarkerVertex> markers;
    poem::mapgpu::AppendMarkerVertices(markers, draw, vertices, indices);
    Require(markers.empty(), "an out-of-range index is skipped rather than read out of bounds");

    // A count beyond the index buffer must stop, not run off the end.
    draw.count = 64;
    markers.clear();
    poem::mapgpu::AppendMarkerVertices(markers, draw, vertices, indices);
    Require(markers.empty(), "a count past the index buffer stops safely");
}

void MarkerBatchSelection() {
    poem::protocol::MapDrawBatch points{};
    points.primitive = poem::protocol::MapPrimitive::Points;
    Require(poem::mapgpu::IsMarkerBatch(points), "untextured points are markers");
    Require(!poem::mapgpu::IsGpuBatch(points), "markers are not ground/extrusion batches");

    poem::protocol::MapDrawBatch textured = points;
    textured.textureHash[1] = 5;
    Require(!poem::mapgpu::IsMarkerBatch(textured), "textured points stay on the overlay");

    poem::protocol::MapDrawBatch ground{};
    ground.primitive = poem::protocol::MapPrimitive::Triangles;
    Require(!poem::mapgpu::IsMarkerBatch(ground), "triangles are not markers");
    Require(poem::mapgpu::HasMarkerBatches({ground, points}), "a scene with points has markers");
    Require(!poem::mapgpu::HasMarkerBatches({ground}), "a scene without points has none");
}


// The shadow frame is shared, so both presenters render and sample cascades in
// the same light space.
void ShadowSetupBuildsAnOrthonormalLightFrame() {
    const auto view = Make(52.3676, 4.9041, 16, 25, 55, 1, 0, 0, 800, 600);
    const auto sun = SunDirection(view, 120, 30);

    Require(poem::mapgpu::MakeShadowSetup(view, sun, 0).count == 0, "zero cascades disables shadows");
    Require(poem::mapgpu::MakeShadowSetup(view, sun, 5).count == poem::mapgpu::kMaxShadowCascades,
            "cascade count is clamped to the maximum");

    const auto setup = poem::mapgpu::MakeShadowSetup(view, sun, 2);
    Require(setup.count == 2, "two cascades requested and granted");

    auto length = [](const float v[3]) { return std::sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2]); };
    auto dot = [](const float a[3], const float b[3]) { return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]; };
    Require(Near(length(setup.right), 1.0, 1e-5), "right is unit length");
    Require(Near(length(setup.up), 1.0, 1e-5), "up is unit length");
    Require(Near(length(setup.forward), 1.0, 1e-5), "forward is unit length");
    Require(std::fabs(dot(setup.right, setup.up)) < 1e-5, "right and up are perpendicular");
    Require(std::fabs(dot(setup.right, setup.forward)) < 1e-5, "right and forward are perpendicular");
    Require(std::fabs(dot(setup.up, setup.forward)) < 1e-5, "up and forward are perpendicular");

    // Light travels away from the sun, so forward opposes the sun direction.
    const float sunVector[3] = {static_cast<float>(sun.x), static_cast<float>(sun.y), static_cast<float>(sun.z)};
    Require(dot(setup.forward, sunVector) < -0.99, "forward points away from the sun");

    // Each cascade reaches further than the last, trading resolution for range.
    Require(setup.radius[0] > 0, "cascade 0 has a positive extent");
    Require(Near(setup.radius[1], setup.radius[0] * poem::mapgpu::kCascadeGrowth, 1e-3),
            "each cascade grows by the documented factor");
}

void ShadowSetupHandlesAnOverheadSun() {
    // A sun straight up makes the usual up x forward cross product degenerate;
    // the frame must stay finite and orthonormal rather than producing NaNs.
    const auto view = FlatView();
    const auto overhead = SunDirection(view, 0, 90);
    const auto setup = poem::mapgpu::MakeShadowSetup(view, overhead, 1);
    Require(setup.count == 1, "an overhead sun still yields a cascade");
    for (int axis = 0; axis < 3; ++axis) {
        Require(std::isfinite(setup.right[axis]) && std::isfinite(setup.up[axis]) &&
                    std::isfinite(setup.forward[axis]),
                "an overhead sun produces a finite light frame");
    }
    auto length = [](const float v[3]) { return std::sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2]); };
    Require(Near(length(setup.right), 1.0, 1e-5), "fallback right stays unit length");
}

void ShadowFactorDarkensOnlyOccludedSurfaces() {
    // Nothing was rendered into the map there: stored stays at the far value.
    Require(Near(MapShadowFactorReference(1.0f, 0.4f, 0.0025f, 0.45f), 1.0, 1e-6),
            "a surface the light reaches is fully lit");
    // The caster itself: equal depth, kept lit by the bias rather than
    // shadowing itself into acne.
    Require(Near(MapShadowFactorReference(0.4f, 0.4f, 0.0025f, 0.45f), 1.0, 1e-6),
            "a caster does not shadow itself");
    // Behind an occluder: darkened, but only by strength, never to black.
    Require(Near(MapShadowFactorReference(0.3f, 0.5f, 0.0025f, 0.45f), 0.55, 1e-6),
            "an occluded surface darkens by the strength, not to black");
}

void RunAll() {
    ProjectsCameraToViewportCentre();
    HorizontalWrapTakesShortestPath();
    ElevationRaisesAndNears();
    SunDirectionIsUnitAndOriented();
    ShadingSpansAmbientToFull();
    DegenerateFaceIsUnshaded();
    FogGrowsWithDistanceAndThinsWithHeight();
    FogMixIsABlend();
    DepthSortOnlyWhenPitched();
    GpuUniformsMatchTheView();
    GpuBatchSelectionOwnsUntexturedTriangles();
    MarkerExpansionBuildsBillboards();
    MarkerExpansionRejectsOutOfRangeIndices();
    MarkerBatchSelection();
    ShadowSetupBuildsAnOrthonormalLightFrame();
    ShadowSetupHandlesAnOverheadSun();
    ShadowFactorDarkensOnlyOccludedSurfaces();
}

} // namespace

int main() {
    try {
        RunAll();
    } catch (const std::exception& error) {
        // Report which contract broke; a bare terminate tells you nothing.
        std::fprintf(stderr, "%s\n", error.what());
        return 1;
    }
    return 0;
}
