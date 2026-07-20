#pragma once

// GLES2 presenter renderer for the POEM v2 draw-command protocol — the
// Android sibling of cpp_sidecar's RendererD3D11. It interprets commands with
// the same semantics (SDF rounded rects, glyph quads from the Go-supplied
// atlas, scissor clipping, offset translation, shadow/glow companion quads)
// so a frame renders the same on either presenter.

#include <GLES2/gl2.h>
#include <cstdint>
#include <string>
#include <unordered_map>
#include <vector>

#include "poem/protocol.h"

namespace poem {

class RendererGLES {
  public:
    // scale converts the engine's logical pixels to physical surface pixels
    // (the display density), exactly as the D3D11 presenter's scaleX/scaleY do
    // for DPI on Windows.
    bool Init(int width, int height, float scale);
    void Resize(int width, int height);
    // SetInset positions the content area inside the surface (physical px):
    // system bars overlap the surface edges, so drawing and clipping shift by
    // the content rect origin while the engine lays out inset-free.
    void SetInset(int x, int y);
    void UploadAtlas(const protocol::InitEngine& init);
    void ApplyMapScene(const protocol::MapSceneDelta& scene);
    void Render(const protocol::RenderFrame& frame);

  private:
	struct RetainedMapScene;
    struct Vertex {
        float x, y;
        float u, v;
        float r, g, b, a;
        float rectX, rectY, rectW, rectH;
        float drawType, glow, isGlass, radius;
    };
    struct GlyphInfo {
        float u1, v1, u2, v2;
        int width, height, advance;
    };
    struct DrawRange {
        std::uint32_t start;
        std::uint32_t count;
        bool clipEnabled;
        int clipX, clipY, clipW, clipH;
        unsigned int imageTexture = 0; // 0 = atlas; else a DrawImage texture
		std::string mapViewportID;
		float mapLeft = 0, mapTop = 0, mapRight = 0, mapBottom = 0;
		float mapScale = 1;
    };

    // ImageEntry caches uploaded DrawImage textures by content hash: the
    // engine re-sends image bytes every frame, and re-uploading a photo-sized
    // RGBA buffer at 60fps would swamp the GPU bus.
    struct ImageEntry {
        unsigned int texture = 0;
        int width = 0, height = 0;
    };

    void AppendQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                    float u1, float v1, float u2, float v2,
                    float r, float g, float b, float a,
                    float drawType, float glow, float isGlass, float radius);
    void BuildGeometry(const protocol::RenderFrame& frame,
                       std::vector<Vertex>& vertices, std::vector<DrawRange>& ranges);
	struct MapGeometryRange { std::uint32_t start = 0, count = 0; std::string textureHash; };
    void AppendMapGeometry(std::vector<Vertex>& vertices, std::vector<MapGeometryRange>& ranges, const std::string& viewportId, float left, float top, float right, float bottom, float previewScale);
	bool EnsureMapGeometryBuffer(const std::string& viewportId, float left, float top, float right, float bottom, float previewScale);
	GLuint EnsureMapTexture(RetainedMapScene& scene, const std::string& hash);
    unsigned int UploadImageCached(const std::vector<std::uint8_t>& rgba, int width, int height);

    int width_ = 0;
    int height_ = 0;
    float scale_ = 1.0f;
    int insetX_ = 0;
    int insetY_ = 0;
    GLint uInset_ = -1;
    GLuint program_ = 0;
    GLuint vbo_ = 0;
    GLuint atlasTexture_ = 0;
    GLint uScreen_ = -1;
    GLint uAtlas_ = -1;
    std::unordered_map<std::uint32_t, GlyphInfo> glyphs_;
    std::unordered_map<std::uint64_t, ImageEntry> images_;
    struct RetainedMapScene {
        std::uint64_t generation = 0;
        protocol::MapCamera camera{};
        // Solar position for the scene (degrees). Extruded geometry is shaded
        // against it; the defaults are the engine's north-west key light.
        float sunAzimuth = 315.0f;
        float sunElevation = 45.0f;
        std::vector<protocol::MapDrawBatch> draws;
        std::unordered_map<std::string, protocol::MapSceneResource> resources;
		GLuint vertexBuffer = 0;
		std::uint32_t vertexCount = 0;
		std::vector<MapGeometryRange> geometryRanges;
		std::unordered_map<std::string, GLuint> textures;
		float left = 0, top = 0, right = 0, bottom = 0;
		float previewScale = 1;
		bool geometryDirty = true;
    };
    std::unordered_map<std::string, RetainedMapScene> mapScenes_;
};

} // namespace poem
