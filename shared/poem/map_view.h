#pragma once

// Shared, platform-neutral map view maths: the camera transform every presenter
// uses to place vector geometry, plus the solar shading and depth ordering that
// extruded geometry needs.
//
// This exists because the D3D11 and GLES presenters had independently copied
// the same projection, and the sun shading was about to become a third copy
// when the WebGL bridge caught up. The framework TDD calls that out directly:
// portable behaviour belongs in shared code (§4.1), shared headers must not
// expose platform types (§4.2), and backend divergence is mitigated with "one
// renderer contract, shared validation" (§40.3). Nothing here touches D3D11,
// GLES, EGL, Win32 or JNI — it is arithmetic over values each host already has,
// so every backend lights and orders the scene identically by construction
// rather than by review.
//
// Conventions, once, so the call sites do not restate them:
//   * x/y are normalized Web Mercator (0..1); y grows southward.
//   * elevation is metres; +z is up.
//   * bearing and pitch are degrees; azimuth is clockwise from north.

#include <algorithm>
#include <array>
#include <cmath>

namespace poem::mapview {

inline constexpr double kPi = 3.14159265358979323846;
inline constexpr double kEarthCircumferenceMeters = 40075016.68557849;
inline constexpr double kMaxMercatorLatitude = 85.05112878;
// Unlit faces stay readable rather than going black.
inline constexpr double kDefaultAmbient = 0.45;

// View is the resolved camera transform for one frame. Build it once per scene
// with Make() and pass it to the free functions below.
struct View {
    double centerX = 0, centerY = 0; // camera position, normalized mercator
    double worldPixels = 0;
    double cosBearing = 1, sinBearing = 0;
    double pitchCos = 1, pitchSin = 0;
    double metersToPixels = 0;
    double cameraDistance = 1;
    double left = 0, top = 0, right = 0, bottom = 0; // viewport rect, pixels
};

struct Point {
    float x = 0, y = 0;
};

struct Sun {
    double x = 0, y = 0, z = 1;
};

// Make resolves the camera into a View. previewScale lets a host render the
// scene at a fraction of its logical size.
inline View Make(double latitude, double longitude, double zoom, double bearingDegrees,
                 double pitchDegrees, double previewScale, double left, double top,
                 double right, double bottom) {
    View view;
    const double clamped = std::max(-kMaxMercatorLatitude, std::min(kMaxMercatorLatitude, latitude));
    view.centerX = (longitude + 180.0) / 360.0;
    const double latitudeSin = std::sin(clamped * kPi / 180.0);
    view.centerY = .5 - std::log((1.0 + latitudeSin) / (1.0 - latitudeSin)) / (4.0 * kPi);
    view.worldPixels = 512.0 * std::pow(2.0, zoom) * std::max(.01, previewScale);
    const double angle = -bearingDegrees * kPi / 180.0;
    view.cosBearing = std::cos(angle);
    view.sinBearing = std::sin(angle);
    const double pitch = pitchDegrees * kPi / 180.0;
    view.pitchCos = std::cos(pitch);
    view.pitchSin = std::sin(pitch);
    // Deliberately the unclamped latitude: this scales metres at the camera,
    // not the mercator centre.
    view.metersToPixels = view.worldPixels / (kEarthCircumferenceMeters * std::max(.01, std::cos(latitude * kPi / 180.0)));
    view.cameraDistance = std::max(1.0, (bottom - top) * .5 / std::tan(kPi / 8.0));
    view.left = left;
    view.top = top;
    view.right = right;
    view.bottom = bottom;
    return view;
}

// Local returns the position in the view's local pixel space: +x right (after
// bearing), +y south, +z up. Lighting normals are computed here.
inline std::array<double, 3> Local(const View& view, double x, double y, double elevation) {
    double dx = x - view.centerX;
    if (dx > .5) dx -= 1.0; else if (dx < -.5) dx += 1.0;
    const double dy = y - view.centerY;
    return {(dx * view.cosBearing - dy * view.sinBearing) * view.worldPixels,
            (dx * view.sinBearing + dy * view.cosBearing) * view.worldPixels,
            elevation * view.metersToPixels};
}

// Depth grows toward the camera; the perspective divide uses
// cameraDistance - depth, so larger means nearer.
inline double Depth(const View& view, double x, double y, double elevation) {
    const auto local = Local(view, x, y, elevation);
    return local[1] * view.pitchSin + local[2] * view.pitchCos;
}

// Project maps a world position to a screen point.
inline Point Project(const View& view, double x, double y, double elevation) {
    const auto local = Local(view, x, y, elevation);
    const double projectedY = local[1] * view.pitchCos - local[2] * view.pitchSin;
    const double depth = local[1] * view.pitchSin + local[2] * view.pitchCos;
    const double perspective = view.cameraDistance / std::max(view.cameraDistance * .05, view.cameraDistance - depth);
    return Point{static_cast<float>((view.left + view.right) * .5 + local[0] * perspective),
                 static_cast<float>((view.top + view.bottom) * .5 + projectedY * perspective)};
}

// SunDirection converts a solar azimuth/elevation into a unit vector in the
// view's local space. Azimuth is clockwise from north and y grows southward, so
// north is -y; the bearing rotation matches the one applied to positions.
inline Sun SunDirection(const View& view, double azimuthDegrees, double elevationDegrees) {
    const double azimuth = azimuthDegrees * kPi / 180.0;
    const double elevation = elevationDegrees * kPi / 180.0;
    const double east = std::cos(elevation) * std::sin(azimuth);
    const double north = std::cos(elevation) * std::cos(azimuth);
    Sun sun{east * view.cosBearing + north * view.sinBearing,
            east * view.sinBearing - north * view.cosBearing,
            std::sin(elevation)};
    const double length = std::sqrt(sun.x * sun.x + sun.y * sun.y + sun.z * sun.z);
    if (length > 1e-9) {
        sun.x /= length;
        sun.y /= length;
        sun.z /= length;
    }
    return sun;
}

// ShadeTriangle returns a Lambert brightness multiplier for a face, from its
// three world positions. Degenerate faces return 1 (unshaded) rather than a
// division by zero.
inline float ShadeTriangle(const View& view, const Sun& sun,
                           double ax, double ay, double az,
                           double bx, double by, double bz,
                           double cx, double cy, double cz,
                           double ambient = kDefaultAmbient) {
    const auto a = Local(view, ax, ay, az);
    const auto b = Local(view, bx, by, bz);
    const auto c = Local(view, cx, cy, cz);
    const double ux = b[0] - a[0], uy = b[1] - a[1], uz = b[2] - a[2];
    const double vx = c[0] - a[0], vy = c[1] - a[1], vz = c[2] - a[2];
    const double nx = uy * vz - uz * vy, ny = uz * vx - ux * vz, nz = ux * vy - uy * vx;
    const double length = std::sqrt(nx * nx + ny * ny + nz * nz);
    if (length < 1e-9) return 1.0f;
    const double lambert = std::max(0.0, (nx * sun.x + ny * sun.y + nz * sun.z) / length);
    return static_cast<float>(ambient + (1.0 - ambient) * lambert);
}

// kFogScaleHeightMeters is the altitude over which haze thins by 1/e. Fog is a
// ground effect: towers rise out of it while the far ground dissolves, which is
// what reads as depth on a pitched view.
inline constexpr double kFogScaleHeightMeters = 140.0;

// kFogReferenceMeters is the ground distance at which density 1 produces about
// 63% haze. Fog must accumulate over *world* distance, not pixels: pixel extent
// scales with zoom, so a pixel-based density saturates to solid white within a
// block or two when you zoom in.
inline constexpr double kFogReferenceMeters = 3000.0;

// FogFactor returns how much a point is hazed, 0 (clear) to 1 (fully fogged).
// It grows with ground distance from the camera, thins with altitude, and fades
// in with pitch so a top-down map is never hazed. Density 0 disables it.
inline float FogFactor(const View& view, double x, double y, double elevation, double density) {
    if (!(density > 0) || view.pitchSin <= 0 || view.metersToPixels <= 0) return 0.0f;
    const auto local = Local(view, x, y, elevation);
    const double meters = std::sqrt(local[0] * local[0] + local[1] * local[1]) / view.metersToPixels;
    const double altitude = std::exp(-std::max(0.0, elevation) / kFogScaleHeightMeters);
    const double fog = 1.0 - std::exp(-density * (meters / kFogReferenceMeters) * altitude * view.pitchSin);
    return static_cast<float>(std::min(1.0, std::max(0.0, fog)));
}

// MixFog blends one colour channel toward the haze by factor.
inline float MixFog(float channel, float fogChannel, float factor) {
    return channel + (fogChannel - channel) * factor;
}

// NeedsDepthSort reports whether extruded geometry must be painter-sorted.
// Looking straight down nothing occludes, and the sort is the expensive part of
// a geometry rebuild (a city block is ~10^5 faces), so it is skipped there.
inline bool NeedsDepthSort(double pitchDegrees) { return pitchDegrees > 1.0; }

} // namespace poem::mapview
