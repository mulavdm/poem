#include "renderer_gles.h"

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
		entry.second.geometryDirty = true;
	}
    glGenTextures(1, &atlasTexture_);
    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
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
		} else {
			retained.resources[key] = resource;
			const auto texture = retained.textures.find(key);
			if (texture != retained.textures.end()) { glDeleteTextures(1, &texture->second); retained.textures.erase(texture); }
		}
    }
    retained.generation = scene.generation;
    retained.camera = scene.camera;
    retained.sunAzimuth = scene.sunAzimuth;
    retained.sunElevation = scene.sunElevation;
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
        const double latitude = std::max(-85.05112878, std::min(85.05112878, scene.camera.latitude));
        const double centerX = (scene.camera.longitude + 180.0) / 360.0;
        const double latSin = std::sin(latitude * M_PI / 180.0);
        const double centerY = .5 - std::log((1.0 + latSin) / (1.0 - latSin)) / (4.0 * M_PI);
		const double worldPixels = 512.0 * std::pow(2.0, scene.camera.zoom) * std::max(.01f, previewScale);
        const double angle = -scene.camera.bearing * M_PI / 180.0, cs = std::cos(angle), sn = std::sin(angle);
		const double pitch = scene.camera.pitch * M_PI / 180.0, pitchCos = std::cos(pitch), pitchSin = std::sin(pitch);
        const double metersToPixels = worldPixels / (40075016.68557849 * std::max(.01, std::cos(scene.camera.latitude * M_PI / 180.0)));
		const double cameraDistance = std::max(1.0, static_cast<double>(bottom-top)*.5/std::tan(M_PI/8.0));
		auto screen = [&](float x, float y, float elevation) { double dx=x-centerX;if(dx>.5)dx-=1;else if(dx<-.5)dx+=1;const double dy=y-centerY,localX=(dx*cs-dy*sn)*worldPixels,localY=(dx*sn+dy*cs)*worldPixels,localZ=elevation*metersToPixels,projectedY=localY*pitchCos-localZ*pitchSin,depth=localY*pitchSin+localZ*pitchCos,perspective=cameraDistance/std::max(cameraDistance*.05,cameraDistance-depth);return std::array<float,2>{static_cast<float>((left+right)*.5+localX*perspective),static_cast<float>((top+bottom)*.5+projectedY*perspective)};};
		// Lighting works in the same local pixel space the projection builds
		// from: +x right (after bearing), +y south, +z up. Only extruded
		// batches (draw.depthTest, set from the style's Extrude) are shaded, so
		// flat land/water keep exactly the colours the style asked for.
		auto localSpace = [&](float x, float y, float elevation) {
			double dx = x - centerX; if (dx > .5) dx -= 1; else if (dx < -.5) dx += 1;
			const double dy = y - centerY;
			return std::array<double, 3>{(dx*cs - dy*sn)*worldPixels, (dx*sn + dy*cs)*worldPixels, elevation*metersToPixels};
		};
		// Sun azimuth is clockwise from north; map y grows southward, so north
		// is -y. The bearing rotation matches the one applied to positions.
		const double sunAz = scene.sunAzimuth * M_PI/180.0, sunEl = scene.sunElevation * M_PI/180.0;
		const double sunEast = std::cos(sunEl)*std::sin(sunAz), sunNorth = std::cos(sunEl)*std::cos(sunAz);
		double sunX = sunEast*cs + sunNorth*sn, sunY = sunEast*sn - sunNorth*cs, sunZ = std::sin(sunEl);
		{
			const double length = std::sqrt(sunX*sunX + sunY*sunY + sunZ*sunZ);
			if (length > 1e-9) { sunX/=length; sunY/=length; sunZ/=length; }
		}
		constexpr double kAmbient = 0.45; // unlit faces stay readable, never black
		auto shadeTriangle = [&](const MapVertex& a, const MapVertex& b, const MapVertex& c) {
			const auto pa = localSpace(a.x,a.y,a.z), pb = localSpace(b.x,b.y,b.z), pc = localSpace(c.x,c.y,c.z);
			const double ux=pb[0]-pa[0], uy=pb[1]-pa[1], uz=pb[2]-pa[2];
			const double vx=pc[0]-pa[0], vy=pc[1]-pa[1], vz=pc[2]-pa[2];
			double nx=uy*vz-uz*vy, ny=uz*vx-ux*vz, nz=ux*vy-uy*vx;
			const double length = std::sqrt(nx*nx+ny*ny+nz*nz);
			if (length < 1e-9) return 1.0f;
			const double lambert = std::max(0.0, (nx*sunX + ny*sunY + nz*sunZ)/length);
			return static_cast<float>(kAmbient + (1.0-kAmbient)*lambert);
		};
		auto applyShade = [](MapVertex& v, float shade) { v.r*=shade; v.g*=shade; v.b*=shade; };
		// Camera-space depth, matching the projection: it grows toward the
		// camera (the perspective divide uses cameraDistance - depth).
		auto depthOf = [&](float x, float y, float elevation) {
			double dx = x - centerX; if (dx > .5) dx -= 1; else if (dx < -.5) dx += 1;
			const double dy = y - centerY;
			return (dx*sn + dy*cs)*worldPixels*pitchSin + elevation*metersToPixels*pitchCos;
		};

        for (const auto& draw : scene.draws) {
			const auto rangeStart = static_cast<std::uint32_t>(vertices.size());
			const bool textured = std::any_of(draw.textureHash.begin(), draw.textureHash.end(), [](std::uint8_t value) { return value != 0; });
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
                        faces.push_back(Face{screen(a.x,a.y,a.z),screen(b.x,b.y,b.z),screen(c.x,c.y,c.z),a,b,c,
                                             (depthOf(a.x,a.y,a.z)+depthOf(b.x,b.y,b.z)+depthOf(c.x,c.y,c.z))/3.0});
                    }
                    // Depth grows toward the camera, so ascending draws far
                    // first. Looking straight down there is nothing to occlude,
                    // so skip the sort — it is the expensive part of a rebuild
                    // (this batch alone is ~167k faces) and pan/zoom rebuilds
                    // run per frame.
                    if (scene.camera.pitch > 1.0f) {
                        std::sort(faces.begin(), faces.end(), [](const Face& l, const Face& r){ return l.depth < r.depth; });
                    }
                    for (const auto& face : faces) {
                        vertices.push_back(solid(face.pa[0],face.pa[1],face.a));
                        vertices.push_back(solid(face.pb[0],face.pb[1],face.b));
                        vertices.push_back(solid(face.pc[0],face.pc[1],face.c));
                    }
                } else {
                for (std::uint32_t i=0; i+2<draw.count; i+=3) { MapVertex a{},b{},c{}; if(!readVertex(verticesIt->second,indexAt(i),a)||!readVertex(verticesIt->second,indexAt(i+1),b)||!readVertex(verticesIt->second,indexAt(i+2),c)) continue; auto pa=screen(a.x,a.y,a.z),pb=screen(b.x,b.y,b.z),pc=screen(c.x,c.y,c.z); a.a*=draw.opacity;b.a*=draw.opacity;c.a*=draw.opacity; vertices.push_back(solid(pa[0],pa[1],a));vertices.push_back(solid(pb[0],pb[1],b));vertices.push_back(solid(pc[0],pc[1],c)); }
                }
            } else if (draw.primitive == protocol::MapPrimitive::Lines) {
                for (std::uint32_t i=0; i+1<draw.count; i+=2) { MapVertex a{},b{}; if(!readVertex(verticesIt->second,indexAt(i),a)||!readVertex(verticesIt->second,indexAt(i+1),b)) continue; auto pa=screen(a.x,a.y,a.z),pb=screen(b.x,b.y,b.z); float dx=pb[0]-pa[0],dy=pb[1]-pa[1],length=std::sqrt(dx*dx+dy*dy); if(length<.01f) continue; float half=std::max(1.0f,a.width)*.5f,nx=-dy/length*half,ny=dx/length*half; a.a*=draw.opacity; vertices.push_back(solid(pa[0]+nx,pa[1]+ny,a));vertices.push_back(solid(pb[0]+nx,pb[1]+ny,a));vertices.push_back(solid(pa[0]-nx,pa[1]-ny,a));vertices.push_back(solid(pa[0]-nx,pa[1]-ny,a));vertices.push_back(solid(pb[0]+nx,pb[1]+ny,a));vertices.push_back(solid(pb[0]-nx,pb[1]-ny,a)); }
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
    glClear(GL_COLOR_BUFFER_BIT);
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
			if (!EnsureMapGeometryBuffer(range.mapViewportID, range.mapLeft, range.mapTop, range.mapRight, range.mapBottom, range.mapScale)) continue;
			auto& scene = mapScenes_.at(range.mapViewportID);
			bindVertices(scene.vertexBuffer);
			if (range.clipEnabled) {
				glEnable(GL_SCISSOR_TEST);
				glScissor(range.clipX + insetX_, height_ - (range.clipY + insetY_ + range.clipH), range.clipW, range.clipH);
			} else glDisable(GL_SCISSOR_TEST);
			for (const auto& geometryRange : scene.geometryRanges) {
				const auto texture = EnsureMapTexture(scene, geometryRange.textureHash);
				if (texture == 0) continue;
				glBindTexture(GL_TEXTURE_2D, texture);
				glDrawArrays(GL_TRIANGLES, static_cast<GLint>(geometryRange.start), static_cast<GLsizei>(geometryRange.count));
			}
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
