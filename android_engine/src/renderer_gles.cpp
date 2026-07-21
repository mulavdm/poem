#include "renderer_gles.h"

#include "poem/map_gpu.h"
#include "poem/map_shader.h"
#include "poem/map_view.h"

#include <android/log.h>
#include <algorithm>
#include <cmath>
#include <cstring>
#include <string>
#include <unordered_set>

#define RLOGE(...) __android_log_print(ANDROID_LOG_ERROR, "poem-gles", __VA_ARGS__)

namespace poem {

namespace {
std::string MapHashKey(const std::array<std::uint8_t, 32>& hash) {
    return std::string(reinterpret_cast<const char*>(hash.data()), hash.size());
}
}

namespace {

// UTF-8 decode, mirrored from cpp_sidecar's presenter so both interpret
// DrawCommand.text identically.
std::vector<std::uint32_t> DecodeUtf8(const std::string& text) {
    std::vector<std::uint32_t> codepoints;
    codepoints.reserve(text.size());
    for (std::size_t index = 0; index < text.size();) {
        const auto first = static_cast<std::uint8_t>(text[index]);
        std::uint32_t codepoint = 0;
        std::size_t length = 0;
        if (first < 0x80) {
            codepoint = first;
            length = 1;
        } else if ((first & 0xE0) == 0xC0) {
            codepoint = first & 0x1F;
            length = 2;
        } else if ((first & 0xF0) == 0xE0) {
            codepoint = first & 0x0F;
            length = 3;
        } else if ((first & 0xF8) == 0xF0) {
            codepoint = first & 0x07;
            length = 4;
        } else {
            ++index;
            continue;
        }
        if (index + length > text.size()) break;
        bool valid = true;
        for (std::size_t i = 1; i < length; ++i) {
            const auto follow = static_cast<std::uint8_t>(text[index + i]);
            if ((follow & 0xC0) != 0x80) {
                valid = false;
                break;
            }
            codepoint = (codepoint << 6) | (follow & 0x3F);
        }
        if (valid) codepoints.push_back(codepoint);
        index += length;
    }
    return codepoints;
}

const char* kVertexShader = R"(
attribute vec2 aPos;
attribute vec2 aUV;
attribute vec4 aColor;
attribute vec4 aRect;
attribute vec4 aMisc; // drawType, glow, isGlass, radius
uniform vec2 uScreen;
uniform vec2 uInset;
varying vec2 vUV;
varying vec4 vColor;
varying vec4 vRect;
varying vec4 vMisc;
varying vec2 vPixel;
void main() {
    vUV = aUV;
    vColor = aColor;
    vRect = aRect;
    vMisc = aMisc;
    vPixel = aPos;
    vec2 p = aPos + uInset;
    vec2 ndc = vec2(p.x / uScreen.x * 2.0 - 1.0, 1.0 - p.y / uScreen.y * 2.0);
    gl_Position = vec4(ndc, 0.0, 1.0);
}
)";

// drawType: 0 = SDF shape, 1 = text glyph. The rounded-box SDF matches the
// sibling presenters: distance from the rect (shrunk by radius), smoothed
// over ~1px for antialiasing; glow widens the falloff.
const char* kFragmentShader = R"(
precision mediump float;
uniform sampler2D uAtlas;
varying vec2 vUV;
varying vec4 vColor;
varying vec4 vRect;
varying vec4 vMisc;
varying vec2 vPixel;
void main() {
    if (vMisc.x > 1.5) {
        gl_FragColor = texture2D(uAtlas, vUV) * vColor;
        return;
    }
    if (vMisc.x > 0.5) {
        vec4 sample = texture2D(uAtlas, vUV);
        gl_FragColor = vec4(vColor.rgb, vColor.a * sample.a);
        return;
    }
    vec2 center = vRect.xy + vRect.zw * 0.5;
    vec2 halfSize = vRect.zw * 0.5;
    float radius = min(vMisc.w, min(halfSize.x, halfSize.y));
    vec2 p = abs(vPixel - center) - (halfSize - vec2(radius));
    float dist = length(max(p, 0.0)) + min(max(p.x, p.y), 0.0) - radius;
    float soft = max(vMisc.y, 1.0);
    float alpha = 1.0 - smoothstep(-soft, soft * 0.5, dist);
    gl_FragColor = vec4(vColor.rgb, vColor.a * alpha);
}
)";

// GPU map shaders assembled from the shared canonical source: GLSL prelude,
// the shared maths, then the entry points, which are the only per-backend part
// (GLSL declares attributes/varyings and uniforms differently from HLSL).
std::string MapVertexShaderSource() {
    return std::string(poem::mapshader::kGlslPrelude) + R"(
attribute vec3 aPos;
attribute vec4 aColor;
uniform vec4 uCenter;
uniform vec4 uWorld;
uniform vec4 uPitch;
uniform vec4 uRect;
uniform vec4 uScreen;
varying vec4 vColor;
varying vec3 vLocal;
)" + std::string(poem::mapshader::kBody) + R"(
void main() {
    vLocal = MapLocal(aPos, uCenter, uWorld);
    gl_Position = MapClip(vLocal, uPitch, uRect, uScreen);
    vColor = aColor;
}
)";
}

std::string MapFragmentShaderSource() {
    // Derivatives are core in GLES3; the directive keeps ES 1.00 sources valid.
    return std::string("#extension GL_OES_standard_derivatives : enable\nprecision highp float;\n") +
           std::string(poem::mapshader::kGlslPrelude) + R"(
varying vec4 vColor;
varying vec3 vLocal;
uniform vec4 uSunAmbient;
uniform vec4 uFog;
uniform vec4 uFogParams;
uniform vec4 uDraw; // opacity shaded - -
)" + std::string(poem::mapshader::kBody) + R"(
void main() {
    vec3 color = vColor.rgb;
    if (uDraw.y > 0.5) {
        color = MapShade(color, dFdx(vLocal), dFdy(vLocal), uSunAmbient);
    }
    color = MapFog(color, vLocal, uFog, uFogParams);
    gl_FragColor = vec4(color, vColor.a * uDraw.x);
}
)";
}

std::string MarkerVertexShaderSource() {
    return std::string(poem::mapshader::kGlslPrelude) + R"(
attribute vec3 aPos;
attribute vec3 aCorner; // cornerX cornerY radius
attribute vec4 aColor;
uniform vec4 uCenter;
uniform vec4 uWorld;
uniform vec4 uPitch;
uniform vec4 uRect;
uniform vec4 uScreen;
varying vec4 vColor;
varying vec2 vCorner;
)" + std::string(poem::mapshader::kBody) + R"(
void main() {
    vec3 local = MapLocal(aPos, uCenter, uWorld);
    vec4 clip = MapClip(local, uPitch, uRect, uScreen);
    gl_Position = MapMarkerClip(clip, aCorner.xy, aCorner.z, uScreen);
    vColor = aColor;
    vCorner = aCorner.xy;
}
)";
}

std::string MarkerFragmentShaderSource() {
    return std::string("precision mediump float;\n") + std::string(poem::mapshader::kGlslPrelude) + R"(
varying vec4 vColor;
varying vec2 vCorner;
uniform float uOpacity;
)" + std::string(poem::mapshader::kBody) + R"(
void main() {
    float alpha = MapMarkerAlpha(vCorner);
    if (alpha <= 0.0) discard;
    gl_FragColor = vec4(vColor.rgb, vColor.a * alpha * uOpacity);
}
)";
}

GLuint Compile(GLenum type, const char* source) {
    GLuint shader = glCreateShader(type);
    glShaderSource(shader, 1, &source, nullptr);
    glCompileShader(shader);
    GLint ok = GL_FALSE;
    glGetShaderiv(shader, GL_COMPILE_STATUS, &ok);
    if (!ok) {
        char log[512];
        glGetShaderInfoLog(shader, sizeof(log), nullptr, log);
        RLOGE("shader compile failed: %s", log);
        glDeleteShader(shader);
        return 0;
    }
    return shader;
}

} // namespace

bool RendererGLES::Init(int width, int height, float scale) {
    width_ = width;
    height_ = height;
    scale_ = scale > 0.0f ? scale : 1.0f;

    GLuint vs = Compile(GL_VERTEX_SHADER, kVertexShader);
    GLuint fs = Compile(GL_FRAGMENT_SHADER, kFragmentShader);
    if (!vs || !fs) return false;
    program_ = glCreateProgram();
    glAttachShader(program_, vs);
    glAttachShader(program_, fs);
    glBindAttribLocation(program_, 0, "aPos");
    glBindAttribLocation(program_, 1, "aUV");
    glBindAttribLocation(program_, 2, "aColor");
    glBindAttribLocation(program_, 3, "aRect");
    glBindAttribLocation(program_, 4, "aMisc");
    glLinkProgram(program_);
    GLint linked = GL_FALSE;
    glGetProgramiv(program_, GL_LINK_STATUS, &linked);
    glDeleteShader(vs);
    glDeleteShader(fs);
    if (!linked) {
        char log[512];
        glGetProgramInfoLog(program_, sizeof(log), nullptr, log);
        RLOGE("program link failed: %s", log);
        return false;
    }
    uScreen_ = glGetUniformLocation(program_, "uScreen");
    uAtlas_ = glGetUniformLocation(program_, "uAtlas");
    uInset_ = glGetUniformLocation(program_, "uInset");

    glGenBuffers(1, &vbo_);
	for (auto& entry : mapScenes_) {
		entry.second.vertexBuffer = 0;
		entry.second.vertexCount = 0;
		entry.second.textures.clear();
		// Buffer ids from the previous context are dead; drop, re-upload lazily.
		entry.second.gpuVertexBuffers.clear();
		entry.second.gpuIndexBuffers.clear();
		entry.second.geometryDirty = true;
	}
    glGenTextures(1, &atlasTexture_);
    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);

    // GPU map path. GLES3 is required and negotiated by the host: derivatives
    // and 32-bit indices are core there. The CPU painter path is not a viable
    // fallback for map geometry on mobile (tens of thousands of reprojected
    // triangles per frame), so a failure here is fatal to map rendering and is
    // reported rather than silently degraded.
    const std::string mapVertexSource = MapVertexShaderSource();
    const std::string mapFragmentSource = MapFragmentShaderSource();
    GLuint mapVs = Compile(GL_VERTEX_SHADER, mapVertexSource.c_str());
    GLuint mapFs = Compile(GL_FRAGMENT_SHADER, mapFragmentSource.c_str());
    if (!mapVs || !mapFs) {
        RLOGE("map shader compile failed; map geometry will not render");
        return false;
    }
    mapProgram_ = glCreateProgram();
    glAttachShader(mapProgram_, mapVs);
    glAttachShader(mapProgram_, mapFs);
    glBindAttribLocation(mapProgram_, 0, "aPos");
    glBindAttribLocation(mapProgram_, 1, "aColor");
    glLinkProgram(mapProgram_);
    GLint mapLinked = GL_FALSE;
    glGetProgramiv(mapProgram_, GL_LINK_STATUS, &mapLinked);
    glDeleteShader(mapVs);
    glDeleteShader(mapFs);
    if (!mapLinked) {
        char mapLog[512];
        glGetProgramInfoLog(mapProgram_, sizeof(mapLog), nullptr, mapLog);
        RLOGE("map program link failed: %s", mapLog);
        glDeleteProgram(mapProgram_);
        mapProgram_ = 0;
        return false;
    }
    uMapCenter_ = glGetUniformLocation(mapProgram_, "uCenter");
    uMapWorld_ = glGetUniformLocation(mapProgram_, "uWorld");
    uMapPitch_ = glGetUniformLocation(mapProgram_, "uPitch");
    uMapRect_ = glGetUniformLocation(mapProgram_, "uRect");
    uMapScreen_ = glGetUniformLocation(mapProgram_, "uScreen");
    uMapSunAmbient_ = glGetUniformLocation(mapProgram_, "uSunAmbient");
    uMapFog_ = glGetUniformLocation(mapProgram_, "uFog");
    uMapFogParams_ = glGetUniformLocation(mapProgram_, "uFogParams");
    uMapDraw_ = glGetUniformLocation(mapProgram_, "uDraw");

    const std::string markerVertexSource = MarkerVertexShaderSource();
    const std::string markerFragmentSource = MarkerFragmentShaderSource();
    GLuint markerVs = Compile(GL_VERTEX_SHADER, markerVertexSource.c_str());
    GLuint markerFs = Compile(GL_FRAGMENT_SHADER, markerFragmentSource.c_str());
    if (!markerVs || !markerFs) {
        RLOGE("marker shader compile failed; markers will not render");
        return false;
    }
    markerProgram_ = glCreateProgram();
    glAttachShader(markerProgram_, markerVs);
    glAttachShader(markerProgram_, markerFs);
    glBindAttribLocation(markerProgram_, 0, "aPos");
    glBindAttribLocation(markerProgram_, 1, "aCorner");
    glBindAttribLocation(markerProgram_, 2, "aColor");
    glLinkProgram(markerProgram_);
    GLint markerLinked = GL_FALSE;
    glGetProgramiv(markerProgram_, GL_LINK_STATUS, &markerLinked);
    glDeleteShader(markerVs);
    glDeleteShader(markerFs);
    if (!markerLinked) {
        char markerLog[512];
        glGetProgramInfoLog(markerProgram_, sizeof(markerLog), nullptr, markerLog);
        RLOGE("marker program link failed: %s", markerLog);
        glDeleteProgram(markerProgram_);
        markerProgram_ = 0;
        return false;
    }
    uMarkerCenter_ = glGetUniformLocation(markerProgram_, "uCenter");
    uMarkerWorld_ = glGetUniformLocation(markerProgram_, "uWorld");
    uMarkerPitch_ = glGetUniformLocation(markerProgram_, "uPitch");
    uMarkerRect_ = glGetUniformLocation(markerProgram_, "uRect");
    uMarkerScreen_ = glGetUniformLocation(markerProgram_, "uScreen");
    uMarkerOpacity_ = glGetUniformLocation(markerProgram_, "uOpacity");
    glGenBuffers(1, &markerVbo_);
    glClearDepthf(0.0f); // depth convention: bigger = closer, cleared to far
    return true;
}

void RendererGLES::Resize(int width, int height) {
    width_ = width;
    height_ = height;
}

void RendererGLES::SetInset(int x, int y) {
    insetX_ = x;
    insetY_ = y;
}

void RendererGLES::UploadAtlas(const protocol::InitEngine& init) {
    if (init.atlasWidth <= 0 || init.atlasHeight <= 0) return;
    const std::size_t expected =
        static_cast<std::size_t>(init.atlasWidth) * static_cast<std::size_t>(init.atlasHeight) * 4;
    if (init.atlasPixels.size() < expected) {
        RLOGE("atlas pixel buffer too small: %zu < %zu", init.atlasPixels.size(), expected);
        return;
    }
    glBindTexture(GL_TEXTURE_2D, atlasTexture_);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, init.atlasWidth, init.atlasHeight, 0, GL_RGBA,
                 GL_UNSIGNED_BYTE, init.atlasPixels.data());
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);

    glyphs_.clear();
    for (const auto& ch : init.chars) {
        glyphs_[static_cast<std::uint32_t>(ch.r)] =
            GlyphInfo{ch.u1, ch.v1, ch.u2, ch.v2, ch.width, ch.height, ch.advance};
    }
}

void RendererGLES::ApplyMapScene(const protocol::MapSceneDelta& scene) {
    auto& retained = mapScenes_[scene.viewportId];
    if (scene.generation <= retained.generation) return;
    for (const auto& resource : scene.resources) {
        const auto key = MapHashKey(resource.hash);
        if (resource.operation == protocol::MapResourceOperation::Release) {
			retained.resources.erase(key);
			const auto texture = retained.textures.find(key);
			if (texture != retained.textures.end()) { glDeleteTextures(1, &texture->second); retained.textures.erase(texture); }
			const auto gpuV = retained.gpuVertexBuffers.find(key);
			if (gpuV != retained.gpuVertexBuffers.end()) { glDeleteBuffers(1, &gpuV->second); retained.gpuVertexBuffers.erase(gpuV); }
			const auto gpuI = retained.gpuIndexBuffers.find(key);
			if (gpuI != retained.gpuIndexBuffers.end()) { glDeleteBuffers(1, &gpuI->second); retained.gpuIndexBuffers.erase(gpuI); }
		} else {
			retained.resources[key] = resource;
			const auto texture = retained.textures.find(key);
			if (texture != retained.textures.end()) { glDeleteTextures(1, &texture->second); retained.textures.erase(texture); }
			const auto gpuV = retained.gpuVertexBuffers.find(key);
			if (gpuV != retained.gpuVertexBuffers.end()) { glDeleteBuffers(1, &gpuV->second); retained.gpuVertexBuffers.erase(gpuV); }
			const auto gpuI = retained.gpuIndexBuffers.find(key);
			if (gpuI != retained.gpuIndexBuffers.end()) { glDeleteBuffers(1, &gpuI->second); retained.gpuIndexBuffers.erase(gpuI); }
		}
    }
    retained.generation = scene.generation;
    retained.camera = scene.camera;
    retained.sunAzimuth = scene.sunAzimuth;
    retained.sunElevation = scene.sunElevation;
    retained.fogDensity = scene.fogDensity;
    retained.fogRed = scene.fogRed;
    retained.fogGreen = scene.fogGreen;
    retained.fogBlue = scene.fogBlue;
    retained.draws = scene.draws;
	retained.geometryDirty = true;
	std::unordered_set<std::string> referenced;
	for (const auto& draw : retained.draws) {
		referenced.insert(MapHashKey(draw.vertexHash));
		referenced.insert(MapHashKey(draw.indexHash));
		referenced.insert(MapHashKey(draw.textureHash));
	}
	for (auto resource = retained.resources.begin(); resource != retained.resources.end();) {
		if (referenced.find(resource->first) == referenced.end()) {
			const auto texture = retained.textures.find(resource->first);
			if (texture != retained.textures.end()) { glDeleteTextures(1, &texture->second); retained.textures.erase(texture); }
			const auto gpuV = retained.gpuVertexBuffers.find(resource->first);
			if (gpuV != retained.gpuVertexBuffers.end()) { glDeleteBuffers(1, &gpuV->second); retained.gpuVertexBuffers.erase(gpuV); }
			const auto gpuI = retained.gpuIndexBuffers.find(resource->first);
			if (gpuI != retained.gpuIndexBuffers.end()) { glDeleteBuffers(1, &gpuI->second); retained.gpuIndexBuffers.erase(gpuI); }
			resource = retained.resources.erase(resource);
		}
		else ++resource;
	}
}

bool RendererGLES::EnsureMapGeometryBuffer(const std::string& viewportId, float left, float top, float right, float bottom, float previewScale) {
	auto found = mapScenes_.find(viewportId);
	if (found == mapScenes_.end() || found->second.draws.empty()) return false;
	auto& scene = found->second;
	if (!scene.geometryDirty && scene.vertexBuffer != 0 && scene.vertexCount > 0 && scene.left == left && scene.top == top && scene.right == right && scene.bottom == bottom && scene.previewScale == previewScale) return true;
	std::vector<Vertex> vertices;
	std::vector<MapGeometryRange> ranges;
	AppendMapGeometry(vertices, ranges, viewportId, left, top, right, bottom, previewScale);
	if (vertices.empty()) {
		scene.vertexCount = 0;
		scene.geometryDirty = false;
		return false;
	}
	if (scene.vertexBuffer == 0) glGenBuffers(1, &scene.vertexBuffer);
	glBindBuffer(GL_ARRAY_BUFFER, scene.vertexBuffer);
	glBufferData(GL_ARRAY_BUFFER, static_cast<GLsizeiptr>(vertices.size() * sizeof(Vertex)), vertices.data(), GL_STATIC_DRAW);
	scene.vertexCount = static_cast<std::uint32_t>(vertices.size());
	scene.geometryRanges = std::move(ranges);
	scene.left = left; scene.top = top; scene.right = right; scene.bottom = bottom;
	scene.previewScale = previewScale;
	scene.geometryDirty = false;
	return true;
}

void RendererGLES::AppendMapGeometry(std::vector<Vertex>& vertices, std::vector<MapGeometryRange>& ranges, const std::string& viewportId, float left, float top, float right, float bottom, float previewScale) {
    struct MapVertex { float x, y, z, width, r, g, b, a, u, v, offsetX, offsetY; };
    auto readVertex = [](const protocol::MapSceneResource& resource, std::uint32_t index, MapVertex& out) {
        if (resource.stride < 28 || static_cast<std::uint64_t>(index + 1) * resource.stride > resource.bytes.size()) return false;
        const auto* data = resource.bytes.data() + static_cast<std::size_t>(index) * resource.stride;
        std::memcpy(&out.x, data, 4); std::memcpy(&out.y, data + 4, 4); std::memcpy(&out.z, data + 8, 4); std::memcpy(&out.width, data + 16, 4);
        out.r = data[12] / 255.0f; out.g = data[13] / 255.0f; out.b = data[14] / 255.0f; out.a = data[15] / 255.0f;
		if (resource.stride >= 44) { std::memcpy(&out.u, data + 28, 4); std::memcpy(&out.v, data + 32, 4); std::memcpy(&out.offsetX, data + 36, 4); std::memcpy(&out.offsetY, data + 40, 4); }
        return std::isfinite(out.x) && std::isfinite(out.y) && std::isfinite(out.z) && std::isfinite(out.width);
    };
    auto solid = [](float x, float y, const MapVertex& source) { return Vertex{x, y, 0, 0, source.r, source.g, source.b, source.a, x, y, 1, 1, 0, 0, 0, 0}; };
    for (const auto& entry : mapScenes_) {
        if (entry.first != viewportId) continue;
        const auto& scene = entry.second; if (scene.draws.empty()) continue;
		// Camera transform, solar direction and face shading all come from the
		// shared, platform-neutral module so this presenter cannot drift from
		// the D3D11 one (see shared/poem/map_view.h).
		const auto view = poem::mapview::Make(scene.camera.latitude, scene.camera.longitude, scene.camera.zoom,
		                                      scene.camera.bearing, scene.camera.pitch, previewScale,
		                                      left, top, right, bottom);
		const auto sun = poem::mapview::SunDirection(view, scene.sunAzimuth, scene.sunElevation);
		auto screen = [&](float x, float y, float elevation) {
			const auto point = poem::mapview::Project(view, x, y, elevation);
			return std::array<float, 2>{point.x, point.y};
		};
		// Only extruded batches (draw.depthTest, set from the style's Extrude)
		// are shaded, so flat land/water keep exactly the colours the style asked for.
		auto shadeTriangle = [&](const MapVertex& a, const MapVertex& b, const MapVertex& c) {
			return poem::mapview::ShadeTriangle(view, sun, a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
		};
		auto applyShade = [](MapVertex& v, float shade) { v.r*=shade; v.g*=shade; v.b*=shade; };
		auto depthOf = [&](float x, float y, float elevation) {
			return poem::mapview::Depth(view, x, y, elevation);
		};
		// Height fog is applied per vertex to every map layer, not just
		// extrusions: the ground has to recede too, or buildings fade into a
		// crisp landscape and the depth cue reads as a bug.
		auto applyFog = [&](MapVertex& v) {
			const float factor = poem::mapview::FogFactor(view, v.x, v.y, v.z, scene.fogDensity);
			if (factor <= 0.0f) return;
			v.r = poem::mapview::MixFog(v.r, scene.fogRed, factor);
			v.g = poem::mapview::MixFog(v.g, scene.fogGreen, factor);
			v.b = poem::mapview::MixFog(v.b, scene.fogBlue, factor);
		};

        for (const auto& draw : scene.draws) {
			const auto rangeStart = static_cast<std::uint32_t>(vertices.size());
			const bool textured = std::any_of(draw.textureHash.begin(), draw.textureHash.end(), [](std::uint8_t value) { return value != 0; });
			// Ground/extrusions and markers are both GPU-drawn now; the CPU
			// composite carries only lines and textured label quads.
			if (poem::mapgpu::IsGpuBatch(draw) || poem::mapgpu::IsMarkerBatch(draw)) continue;
            const auto verticesIt = scene.resources.find(MapHashKey(draw.vertexHash)), indicesIt = scene.resources.find(MapHashKey(draw.indexHash));
            if (verticesIt == scene.resources.end() || indicesIt == scene.resources.end() || indicesIt->second.stride != 4 || static_cast<std::uint64_t>(draw.first + draw.count)*4 > indicesIt->second.bytes.size()) continue;
            auto indexAt = [&](std::uint32_t i) { std::uint32_t value{}; std::memcpy(&value, indicesIt->second.bytes.data() + static_cast<std::size_t>(draw.first+i)*4, 4); return value; };
            if (draw.primitive == protocol::MapPrimitive::Triangles && textured) {
				for (std::uint32_t i=0; i+2<draw.count; i+=3) { MapVertex points[3]{}; bool valid=true; for(int p=0;p<3;p++) valid=valid&&readVertex(verticesIt->second,indexAt(i+p),points[p]); if(!valid||verticesIt->second.stride<44) continue; for(auto& point:points){auto at=screen(point.x,point.y,point.z);vertices.push_back(Vertex{at[0]+point.offsetX,at[1]+point.offsetY,point.u,point.v,point.r,point.g,point.b,point.a*draw.opacity,0,0,0,0,1,0,0,0});}}
            } else if (draw.primitive == protocol::MapPrimitive::Triangles) {
                if (draw.depthTest) {
                    // Extruded geometry: the map is CPU-projected to 2D and there
                    // is no depth buffer, so sort triangles back-to-front
                    // (painter's algorithm) or buildings behind paint over the
                    // ones in front and the block reads as translucent. Shade
                    // each face against the sun while we have its 3D positions.
                    struct Face { std::array<float,2> pa, pb, pc; MapVertex a, b, c; double depth; };
                    std::vector<Face> faces;
                    faces.reserve(draw.count / 3);
                    for (std::uint32_t i=0; i+2<draw.count; i+=3) {
                        MapVertex a{},b{},c{};
                        if(!readVertex(verticesIt->second,indexAt(i),a)||!readVertex(verticesIt->second,indexAt(i+1),b)||!readVertex(verticesIt->second,indexAt(i+2),c)) continue;
                        a.a*=draw.opacity; b.a*=draw.opacity; c.a*=draw.opacity;
                        const float shade = shadeTriangle(a,b,c);
                        applyShade(a,shade); applyShade(b,shade); applyShade(c,shade);
                        applyFog(a); applyFog(b); applyFog(c);
                        faces.push_back(Face{screen(a.x,a.y,a.z),screen(b.x,b.y,b.z),screen(c.x,c.y,c.z),a,b,c,
                                             (depthOf(a.x,a.y,a.z)+depthOf(b.x,b.y,b.z)+depthOf(c.x,c.y,c.z))/3.0});
                    }
                    // Depth grows toward the camera, so ascending draws far
                    // first. Looking straight down there is nothing to occlude,
                    // so skip the sort — it is the expensive part of a rebuild
                    // (this batch alone is ~167k faces) and pan/zoom rebuilds
                    // run per frame.
                    if (poem::mapview::NeedsDepthSort(scene.camera.pitch)) {
                        std::sort(faces.begin(), faces.end(), [](const Face& l, const Face& r){ return l.depth < r.depth; });
                    }
                    for (const auto& face : faces) {
                        vertices.push_back(solid(face.pa[0],face.pa[1],face.a));
                        vertices.push_back(solid(face.pb[0],face.pb[1],face.b));
                        vertices.push_back(solid(face.pc[0],face.pc[1],face.c));
                    }
                } else {
                for (std::uint32_t i=0; i+2<draw.count; i+=3) { MapVertex a{},b{},c{}; if(!readVertex(verticesIt->second,indexAt(i),a)||!readVertex(verticesIt->second,indexAt(i+1),b)||!readVertex(verticesIt->second,indexAt(i+2),c)) continue; auto pa=screen(a.x,a.y,a.z),pb=screen(b.x,b.y,b.z),pc=screen(c.x,c.y,c.z); a.a*=draw.opacity;b.a*=draw.opacity;c.a*=draw.opacity; applyFog(a);applyFog(b);applyFog(c); vertices.push_back(solid(pa[0],pa[1],a));vertices.push_back(solid(pb[0],pb[1],b));vertices.push_back(solid(pc[0],pc[1],c)); }
                }
            } else if (draw.primitive == protocol::MapPrimitive::Lines) {
                for (std::uint32_t i=0; i+1<draw.count; i+=2) { MapVertex a{},b{}; if(!readVertex(verticesIt->second,indexAt(i),a)||!readVertex(verticesIt->second,indexAt(i+1),b)) continue; auto pa=screen(a.x,a.y,a.z),pb=screen(b.x,b.y,b.z); float dx=pb[0]-pa[0],dy=pb[1]-pa[1],length=std::sqrt(dx*dx+dy*dy); if(length<.01f) continue; float half=std::max(1.0f,a.width)*.5f,nx=-dy/length*half,ny=dx/length*half; a.a*=draw.opacity; applyFog(a); vertices.push_back(solid(pa[0]+nx,pa[1]+ny,a));vertices.push_back(solid(pb[0]+nx,pb[1]+ny,a));vertices.push_back(solid(pa[0]-nx,pa[1]-ny,a));vertices.push_back(solid(pa[0]-nx,pa[1]-ny,a));vertices.push_back(solid(pb[0]+nx,pb[1]+ny,a));vertices.push_back(solid(pb[0]-nx,pb[1]-ny,a)); }
            } else {
                for(std::uint32_t i=0;i<draw.count;i++){MapVertex p{};if(!readVertex(verticesIt->second,indexAt(i),p))continue;auto at=screen(p.x,p.y,p.z);float r=std::max(3.0f,p.width);AppendQuad(vertices,at[0]-r,at[1]-r,at[0]+r,at[1]+r,0,0,1,1,p.r,p.g,p.b,p.a*draw.opacity,0,0,0,r);}
            }
			const auto rangeCount = static_cast<std::uint32_t>(vertices.size()) - rangeStart;
			if (rangeCount > 0) ranges.push_back(MapGeometryRange{rangeStart, rangeCount, textured ? MapHashKey(draw.textureHash) : std::string{}});
        }
    }
}

GLuint RendererGLES::EnsureMapTexture(RetainedMapScene& scene, const std::string& hash) {
	if (hash.empty()) return atlasTexture_;
	const auto cached = scene.textures.find(hash);
	if (cached != scene.textures.end()) return cached->second;
	const auto found = scene.resources.find(hash);
	if (found == scene.resources.end()) return 0;
	const auto& resource = found->second;
	if (resource.type != protocol::MapResourceType::TextureAlpha || resource.width == 0 || resource.height == 0 || resource.width > 4096 || resource.height > 4096 || resource.bytes.size() != static_cast<std::size_t>(resource.width) * resource.height) return 0;
	GLuint texture = 0; glGenTextures(1, &texture); glBindTexture(GL_TEXTURE_2D, texture);
	glTexImage2D(GL_TEXTURE_2D, 0, GL_ALPHA, resource.width, resource.height, 0, GL_ALPHA, GL_UNSIGNED_BYTE, resource.bytes.data());
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR); glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE); glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
	scene.textures.emplace(hash, texture);
	return texture;
}

void RendererGLES::AppendQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                              float u1, float v1, float u2, float v2,
                              float r, float g, float b, float a,
                              float drawType, float glow, float isGlass, float radius) {
    const Vertex tl{x1, y1, u1, v1, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    const Vertex tr{x2, y1, u2, v1, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    const Vertex bl{x1, y2, u1, v2, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    const Vertex br{x2, y2, u2, v2, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    vertices.push_back(tl);
    vertices.push_back(tr);
    vertices.push_back(bl);
    vertices.push_back(bl);
    vertices.push_back(tr);
    vertices.push_back(br);
}

void RendererGLES::BuildGeometry(const protocol::RenderFrame& frame,
                                 std::vector<Vertex>& vertices, std::vector<DrawRange>& ranges) {
    float currentGlow = 0.0f;
    float currentGlass = 0.0f;
    float shadowOx = 0.0f, shadowOy = 0.0f, shadowBlur = 0.0f;
    float offsetX = 0.0f, offsetY = 0.0f;
    bool clipEnabled = false;
    int clipX = 0, clipY = 0, clipW = 0, clipH = 0;

    unsigned int rangeImage = 0; // texture for the quad being pushed (0 = atlas)
    auto pushRange = [&](std::uint32_t start, std::uint32_t count) {
        if (!ranges.empty()) {
            auto& last = ranges.back();
            if (last.start + last.count == start && last.clipEnabled == clipEnabled &&
                last.imageTexture == rangeImage &&
                last.clipX == clipX && last.clipY == clipY && last.clipW == clipW && last.clipH == clipH) {
                last.count += count;
                return;
            }
        }
        ranges.push_back(DrawRange{start, count, clipEnabled, clipX, clipY, clipW, clipH, rangeImage});
    };

    for (const auto& cmd : frame.commands) {
        switch (cmd.type) {
        case protocol::DrawCommandType::SetGlow:
            currentGlow = cmd.val1 * scale_;
            continue;
        case protocol::DrawCommandType::SetGlass:
            currentGlass = cmd.flag ? 1.0f : 0.0f;
            continue;
        case protocol::DrawCommandType::SetShadow:
            shadowOx = cmd.val1 * scale_;
            shadowOy = cmd.val2 * scale_;
            shadowBlur = cmd.val3 * scale_;
            continue;
        case protocol::DrawCommandType::SetOffset:
            offsetX = cmd.val1 * scale_;
            offsetY = cmd.val2 * scale_;
            continue;
        case protocol::DrawCommandType::SetClip:
            clipEnabled = cmd.flag;
            if (clipEnabled) {
                clipX = static_cast<int>(cmd.x1 * scale_);
                clipY = static_cast<int>(cmd.y1 * scale_);
                clipW = static_cast<int>(cmd.w * scale_);
                clipH = static_cast<int>(cmd.h * scale_);
            }
            continue;
        default:
            break;
        }

        const auto start = static_cast<std::uint32_t>(vertices.size());
		bool retainedMap = false;
        const float cr = cmd.r / 255.0f;
        const float cg = cmd.g / 255.0f;
        const float cb = cmd.b / 255.0f;
        const float ca = cmd.a / 255.0f;
        const float x1 = static_cast<float>(cmd.x1) * scale_ + offsetX;
        const float y1 = static_cast<float>(cmd.y1) * scale_ + offsetY;
        const float x2 = static_cast<float>(cmd.x2) * scale_ + offsetX;
        const float y2 = static_cast<float>(cmd.y2) * scale_ + offsetY;

        // Companion quads first so the element draws over its own effects,
        // mirroring the D3D11 presenter's ordering.
        if ((shadowOx != 0.0f || shadowOy != 0.0f) &&
            cmd.type != protocol::DrawCommandType::DrawText && cmd.radius >= 0) {
            AppendQuad(vertices, x1 + shadowOx, y1 + shadowOy, x2 + shadowOx, y2 + shadowOy,
                       0, 0, 1, 1, 0.0f, 0.0f, 0.0f, ca * 0.25f,
                       0.0f, std::max(1.0f, shadowBlur), 0.0f, static_cast<float>(cmd.radius) * scale_);
        }
        if (currentGlow > 0.0f && cmd.type != protocol::DrawCommandType::DrawText && cmd.radius >= 0) {
            AppendQuad(vertices, x1 - currentGlow, y1 - currentGlow, x2 + currentGlow, y2 + currentGlow,
                       0, 0, 1, 1, cr, cg, cb, ca * 0.15f,
                       0.0f, currentGlow, 0.0f, static_cast<float>(cmd.radius) * scale_ + currentGlow);
        }

        switch (cmd.type) {
        case protocol::DrawCommandType::DrawRoundedRect:
        case protocol::DrawCommandType::FillRect: {
            // Negative radii select raycaster/billboard demo modes on desktop;
            // this presenter renders them as plain rects.
            const float radius = (cmd.type == protocol::DrawCommandType::FillRect)
                                     ? 0.0f
                                     : static_cast<float>(std::max(0, cmd.radius)) * scale_;
            AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, cr, cg, cb, ca,
                       0.0f, currentGlow, currentGlass, radius);
            break;
        }
        case protocol::DrawCommandType::DrawLine: {
            const float thickness = 1.5f * scale_;
            const float minX = std::min(x1, x2) - thickness * 0.5f;
            const float maxX = std::max(x1, x2) + thickness * 0.5f;
            const float minY = std::min(y1, y2) - thickness * 0.5f;
            const float maxY = std::max(y1, y2) + thickness * 0.5f;
            AppendQuad(vertices, minX, minY, maxX, maxY, 0, 0, 1, 1, cr, cg, cb, ca, 0.0f, 0.0f, 0.0f, 0.0f);
            break;
        }
        case protocol::DrawCommandType::DrawText: {
            float penX = x1;
            const float baselineY = y1;
            const float textScale = cmd.val1 > 0.0f ? cmd.val1 : 1.0f;
            for (const auto codepoint : DecodeUtf8(cmd.text)) {
                auto it = glyphs_.find(codepoint);
                if (it == glyphs_.end()) {
                    it = glyphs_.find(static_cast<std::uint32_t>('?'));
                    if (it == glyphs_.end()) {
                        penX += 8.0f * textScale * scale_;
                        continue;
                    }
                }
                const auto& glyph = it->second;
                const float glyphWidth = glyph.width * textScale * scale_;
                const float glyphHeight = glyph.height * textScale * scale_;
                const float gy1 = baselineY - glyphHeight * 0.78f;
                AppendQuad(vertices, penX, gy1, penX + glyphWidth, gy1 + glyphHeight,
                           glyph.u1, glyph.v1, glyph.u2, glyph.v2, cr, cg, cb, ca,
                           1.0f, 0.0f, 0.0f, 0.0f);
                penX += glyph.advance * textScale * scale_;
            }
            break;
        }
        case protocol::DrawCommandType::DrawImage: {
            // cmd.w/h carry the pixel dimensions, cmd.bytes the RGBA data.
            const auto texture = UploadImageCached(cmd.bytes, cmd.w, cmd.h);
            if (texture == 0) break;
            rangeImage = texture;
            // drawType 2: sample the bound image texture across the quad.
            AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, 1, 1, 1, 1, 2.0f, 0.0f, 0.0f, 0.0f);
            break;
        }
        case protocol::DrawCommandType::DrawMapScene: {
			if (const auto scene = mapScenes_.find(cmd.text); scene != mapScenes_.end() && !scene->second.draws.empty()) {
				retainedMap = true;
			} else if (cmd.w > 0 && cmd.h > 0 && !cmd.bytes.empty()) {
                const auto texture = UploadImageCached(cmd.bytes, cmd.w, cmd.h);
                if (texture != 0) {
                    rangeImage = texture;
                    AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, 1, 1, 1, 1, 2.0f, 0, 0, 0);
                }
            }
            break;
        }
        default:
            break;
        }

        const auto end = static_cast<std::uint32_t>(vertices.size());
		if (retainedMap) {
			DrawRange range{0, 0, clipEnabled, clipX, clipY, clipW, clipH, 0};
			range.mapViewportID = cmd.text;
			range.mapLeft = x1; range.mapTop = y1; range.mapRight = x2; range.mapBottom = y2;
			range.mapScale = cmd.val1 > 0 ? cmd.val1 : 1;
			ranges.push_back(std::move(range));
		} else if (end > start) pushRange(start, end - start);
        rangeImage = 0;
    }
}

GLuint RendererGLES::EnsureMapGpuBuffer(RetainedMapScene& scene, const std::string& hash, bool index) {
	auto& cache = index ? scene.gpuIndexBuffers : scene.gpuVertexBuffers;
	const auto cached = cache.find(hash);
	if (cached != cache.end()) return cached->second;
	const auto resource = scene.resources.find(hash);
	if (resource == scene.resources.end() || resource->second.bytes.empty()) return 0;
	GLuint buffer = 0;
	glGenBuffers(1, &buffer);
	if (buffer == 0) return 0;
	const GLenum target = index ? GL_ELEMENT_ARRAY_BUFFER : GL_ARRAY_BUFFER;
	glBindBuffer(target, buffer);
	glBufferData(target, static_cast<GLsizeiptr>(resource->second.bytes.size()), resource->second.bytes.data(), GL_STATIC_DRAW);
	cache.emplace(hash, buffer);
	return buffer;
}

void RendererGLES::DrawGpuMapBatches(RetainedMapScene& scene, const DrawRange& range) {
	if (mapProgram_ == 0) return;
	if (!poem::mapgpu::HasGpuBatches(scene.draws)) return;

	const auto view = poem::mapview::Make(scene.camera.latitude, scene.camera.longitude, scene.camera.zoom,
	                                      scene.camera.bearing, scene.camera.pitch, range.mapScale,
	                                      range.mapLeft, range.mapTop, range.mapRight, range.mapBottom);
	const poem::mapgpu::Lighting lighting{scene.sunAzimuth, scene.sunElevation, scene.fogDensity,
	                                      scene.fogRed, scene.fogGreen, scene.fogBlue};
	const auto uniforms = poem::mapgpu::MakeUniforms(view, lighting, static_cast<float>(width_),
	                                                 static_cast<float>(height_),
	                                                 static_cast<float>(insetX_), static_cast<float>(insetY_));

	glUseProgram(mapProgram_);
	glUniform4fv(uMapCenter_, 1, uniforms.center);
	glUniform4fv(uMapWorld_, 1, uniforms.world);
	glUniform4fv(uMapPitch_, 1, uniforms.pitch);
	glUniform4fv(uMapRect_, 1, uniforms.rect);
	glUniform4fv(uMapScreen_, 1, uniforms.screen);
	glUniform4fv(uMapSunAmbient_, 1, uniforms.sunAmbient);
	glUniform4fv(uMapFog_, 1, uniforms.fog);
	glUniform4fv(uMapFogParams_, 1, uniforms.fogParams);

	for (int i = 2; i <= 4; ++i) glDisableVertexAttribArray(i);
	glEnable(GL_DEPTH_TEST);
	glDepthFunc(GL_GEQUAL);
	glDepthMask(GL_TRUE);

	for (const auto& draw : scene.draws) {
		if (!poem::mapgpu::IsGpuBatch(draw)) continue;
		const GLuint vertexBuffer = EnsureMapGpuBuffer(scene, MapHashKey(draw.vertexHash), false);
		const GLuint indexBuffer = EnsureMapGpuBuffer(scene, MapHashKey(draw.indexHash), true);
		if (vertexBuffer == 0 || indexBuffer == 0) continue;
		glUniform4f(uMapDraw_, draw.opacity, draw.depthTest ? 1.0f : 0.0f, 0.0f, 0.0f);
		glBindBuffer(GL_ARRAY_BUFFER, vertexBuffer);
		glVertexAttribPointer(0, 3, GL_FLOAT, GL_FALSE, poem::mapgpu::kVertexStride, reinterpret_cast<void*>(0));
		glVertexAttribPointer(1, 4, GL_UNSIGNED_BYTE, GL_TRUE, poem::mapgpu::kVertexStride,
		                      reinterpret_cast<void*>(static_cast<std::uintptr_t>(poem::mapgpu::kColorOffset)));
		glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, indexBuffer);
		glDrawElements(GL_TRIANGLES, static_cast<GLsizei>(draw.count), GL_UNSIGNED_INT,
		               reinterpret_cast<void*>(static_cast<std::uintptr_t>(draw.first) * 4));
	}

	glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, 0);
	glDisable(GL_DEPTH_TEST);
	glDepthMask(GL_FALSE);
	for (int i = 2; i <= 4; ++i) glEnableVertexAttribArray(i);
	glUseProgram(program_);
}

void RendererGLES::DrawMapMarkers(RetainedMapScene& scene, const DrawRange& range) {
	if (markerProgram_ == 0) return;
	if (!poem::mapgpu::HasMarkerBatches(scene.draws)) return;

	// Expand every marker batch into billboards using the shared builder.
	std::vector<poem::mapgpu::MarkerVertex> markers;
	for (const auto& draw : scene.draws) {
		if (!poem::mapgpu::IsMarkerBatch(draw)) continue;
		const auto vertexIt = scene.resources.find(MapHashKey(draw.vertexHash));
		const auto indexIt = scene.resources.find(MapHashKey(draw.indexHash));
		if (vertexIt == scene.resources.end() || indexIt == scene.resources.end()) continue;
		poem::mapgpu::AppendMarkerVertices(markers, draw, vertexIt->second.bytes, indexIt->second.bytes);
	}
	if (markers.empty()) return;

	const auto view = poem::mapview::Make(scene.camera.latitude, scene.camera.longitude, scene.camera.zoom,
	                                      scene.camera.bearing, scene.camera.pitch, range.mapScale,
	                                      range.mapLeft, range.mapTop, range.mapRight, range.mapBottom);
	const poem::mapgpu::Lighting lighting{scene.sunAzimuth, scene.sunElevation, scene.fogDensity,
	                                      scene.fogRed, scene.fogGreen, scene.fogBlue};
	const auto uniforms = poem::mapgpu::MakeUniforms(view, lighting, static_cast<float>(width_),
	                                                 static_cast<float>(height_),
	                                                 static_cast<float>(insetX_), static_cast<float>(insetY_));

	glUseProgram(markerProgram_);
	glUniform4fv(uMarkerCenter_, 1, uniforms.center);
	glUniform4fv(uMarkerWorld_, 1, uniforms.world);
	glUniform4fv(uMarkerPitch_, 1, uniforms.pitch);
	glUniform4fv(uMarkerRect_, 1, uniforms.rect);
	glUniform4fv(uMarkerScreen_, 1, uniforms.screen);
	glUniform1f(uMarkerOpacity_, 1.0f);

	glBindBuffer(GL_ARRAY_BUFFER, markerVbo_);
	glBufferData(GL_ARRAY_BUFFER, static_cast<GLsizeiptr>(markers.size() * sizeof(poem::mapgpu::MarkerVertex)),
	             markers.data(), GL_STREAM_DRAW);
	for (int i = 3; i <= 4; ++i) glDisableVertexAttribArray(i);
	glVertexAttribPointer(0, 3, GL_FLOAT, GL_FALSE, poem::mapgpu::kMarkerStride, reinterpret_cast<void*>(0));
	glVertexAttribPointer(1, 3, GL_FLOAT, GL_FALSE, poem::mapgpu::kMarkerStride, reinterpret_cast<void*>(12));
	glVertexAttribPointer(2, 4, GL_UNSIGNED_BYTE, GL_TRUE, poem::mapgpu::kMarkerStride, reinterpret_cast<void*>(24));

	// Read depth so buildings hide markers, but do not write it: markers must
	// not occlude one another.
	glEnable(GL_DEPTH_TEST);
	glDepthFunc(GL_GEQUAL);
	glDepthMask(GL_FALSE);
	glDrawArrays(GL_TRIANGLES, 0, static_cast<GLsizei>(markers.size()));
	glDisable(GL_DEPTH_TEST);

	for (int i = 3; i <= 4; ++i) glEnableVertexAttribArray(i);
	glUseProgram(program_);
}

void RendererGLES::Render(const protocol::RenderFrame& frame) {
	for (auto& entry : mapScenes_) {
		auto& scene = entry.second;
		if (scene.draws.empty() && scene.resources.empty() && scene.vertexBuffer != 0) {
			glDeleteBuffers(1, &scene.vertexBuffer);
			scene.vertexBuffer = 0;
			scene.vertexCount = 0;
			for (const auto& texture : scene.textures) glDeleteTextures(1, &texture.second);
			scene.textures.clear();
		}
	}
    std::vector<Vertex> vertices;
    std::vector<DrawRange> ranges;
    BuildGeometry(frame, vertices, ranges);

    glViewport(0, 0, width_, height_);
    glDisable(GL_SCISSOR_TEST);
    glClearColor(0.04f, 0.05f, 0.07f, 1.0f);
    glDepthMask(GL_TRUE);
    glClear(GL_COLOR_BUFFER_BIT | GL_DEPTH_BUFFER_BIT);
    glDisable(GL_DEPTH_TEST);
	if (ranges.empty()) return;

    glUseProgram(program_);
    glUniform2f(uScreen_, static_cast<float>(width_), static_cast<float>(height_));
    glActiveTexture(GL_TEXTURE0);
    glBindTexture(GL_TEXTURE_2D, atlasTexture_);
    glUniform1i(uAtlas_, 0);
    glUniform2f(uInset_, static_cast<float>(insetX_), static_cast<float>(insetY_));

    const auto stride = static_cast<GLsizei>(sizeof(Vertex));
	auto bindVertices = [&](GLuint buffer) {
		glBindBuffer(GL_ARRAY_BUFFER, buffer);
		glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(0));
		glVertexAttribPointer(1, 2, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(8));
		glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(16));
		glVertexAttribPointer(3, 4, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(32));
		glVertexAttribPointer(4, 4, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(48));
	};
	if (!vertices.empty()) {
		glBindBuffer(GL_ARRAY_BUFFER, vbo_);
		glBufferData(GL_ARRAY_BUFFER, static_cast<GLsizeiptr>(vertices.size() * sizeof(Vertex)), vertices.data(), GL_STREAM_DRAW);
	}
	bindVertices(vbo_);
    for (int i = 0; i <= 4; ++i) glEnableVertexAttribArray(i);

    for (const auto& range : ranges) {
		if (!range.mapViewportID.empty()) {
			const auto sceneIt = mapScenes_.find(range.mapViewportID);
			if (sceneIt == mapScenes_.end()) continue;
			auto& scene = sceneIt->second;
			if (range.clipEnabled) {
				glEnable(GL_SCISSOR_TEST);
				glScissor(range.clipX + insetX_, height_ - (range.clipY + insetY_ + range.clipH), range.clipW, range.clipH);
			} else glDisable(GL_SCISSOR_TEST);
			// Ground and extrusions: GPU-projected from the retained buffers with
			// depth testing (restores the UI program before returning).
			DrawGpuMapBatches(scene, range);
			// Markers: depth-tested billboards, so POIs behind buildings hide.
			DrawMapMarkers(scene, range);
			// Dots, lines and label quads: CPU-composited overlay on top.
			if (EnsureMapGeometryBuffer(range.mapViewportID, range.mapLeft, range.mapTop, range.mapRight, range.mapBottom, range.mapScale) && scene.vertexCount > 0) {
				bindVertices(scene.vertexBuffer);
				for (const auto& geometryRange : scene.geometryRanges) {
					const auto texture = EnsureMapTexture(scene, geometryRange.textureHash);
					if (texture == 0) continue;
					glBindTexture(GL_TEXTURE_2D, texture);
					glDrawArrays(GL_TRIANGLES, static_cast<GLint>(geometryRange.start), static_cast<GLsizei>(geometryRange.count));
				}
			}
			// The GPU path bound its own buffers; restore the UI stream either way.
			bindVertices(vbo_);
			continue;
		}
        glBindTexture(GL_TEXTURE_2D, range.imageTexture != 0 ? range.imageTexture : atlasTexture_);
        if (range.clipEnabled) {
            glEnable(GL_SCISSOR_TEST);
            // Protocol clip rects are top-left origin; GL scissor is bottom-left.
            glScissor(range.clipX + insetX_, height_ - (range.clipY + insetY_ + range.clipH), range.clipW, range.clipH);
        } else {
            glDisable(GL_SCISSOR_TEST);
        }
        glDrawArrays(GL_TRIANGLES, static_cast<GLint>(range.start), static_cast<GLsizei>(range.count));
    }
    glDisable(GL_SCISSOR_TEST);
}

} // namespace poem

namespace poem {

// UploadImageCached returns a texture for the RGBA payload, uploading only on
// first sight of this exact content (FNV-1a over the bytes). The engine
// resends image bytes every frame; the cache turns that into one upload per
// distinct image. Small hard cap with drop-all eviction — UI image working
// sets are tiny, and bookkeeping an LRU would outweigh re-uploading once.
unsigned int RendererGLES::UploadImageCached(const std::vector<std::uint8_t>& rgba, int width, int height) {
    if (width <= 0 || height <= 0 ||
        rgba.size() < static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4) {
        return 0;
    }
    std::uint64_t hash = 1469598103934665603ull;
    for (const auto byte : rgba) {
        hash ^= byte;
        hash *= 1099511628211ull;
    }
    if (auto it = images_.find(hash); it != images_.end()) return it->second.texture;

    if (images_.size() >= 8) {
        for (auto& [key, entry] : images_) glDeleteTextures(1, &entry.texture);
        images_.clear();
    }
    GLuint texture = 0;
    glGenTextures(1, &texture);
    glBindTexture(GL_TEXTURE_2D, texture);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, width, height, 0, GL_RGBA, GL_UNSIGNED_BYTE, rgba.data());
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
    images_[hash] = ImageEntry{texture, width, height};
    return texture;
}

} // namespace poem
