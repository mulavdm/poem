// Contract tests for the shared map view maths. These run on desktop CI with no
// GPU, window or platform SDK, which is the point: every presenter projects and
// lights the scene through this one implementation, so pinning it here pins all
// of them (framework TDD §30.1 shared unit tests, §40.3 backend divergence).

#include "poem/map_view.h"

#include <cmath>
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
