#pragma once

#include "poem/protocol.h"

#include <d3d11.h>
#include <wrl/client.h>
#include <array>
#include <cstdint>
#include <string>
#include <unordered_map>
#include <vector>

namespace poem {

class RendererD3D11 {
  public:
    RendererD3D11() = default;
    bool Initialize(HWND hwnd, int width, int height, const protocol::InitEngine& init);
    bool UpdateFontAtlas(const protocol::InitEngine& init);
    void Resize(int width, int height);
    void ApplyMapScene(const protocol::MapSceneDelta& scene);
    void Render(const protocol::RenderFrame& frame);
    bool CaptureBackbufferRGBA(std::vector<std::uint8_t>& rgba, int& width, int& height);
    int BackbufferWidth() const { return width_; }
    int BackbufferHeight() const { return height_; }

  private:
	struct RetainedMapScene;
    struct Vertex {
        float x, y;
        float u, v;
        float r, g, b, a;
        float rectX, rectY, rectW, rectH;
        float drawType, glow, isGlass, radius;
        float shadowOffsetX, shadowOffsetY, shadowSoftness, padding;
    };

    struct GlyphInfo {
        float u1, v1, u2, v2;
        int width, height, advance;
    };

    struct RaycasterState {
        bool valid = false;
        float viewportX1 = 0.0f;
        float viewportY1 = 0.0f;
        float viewportX2 = 0.0f;
        float viewportY2 = 0.0f;
        float playerX = 0.0f;
        float playerY = 0.0f;
        float playerAngle = 0.0f;
    };

    struct DrawRange {
        std::uint32_t start = 0;
        std::uint32_t count = 0;
        D3D11_RECT scissor{0, 0, 0, 0};
        bool usesImage = false;
        int imageWidth = 0;
        int imageHeight = 0;
        std::vector<std::uint8_t> imageBytes;
		std::string mapViewportID;
		float mapLeft = 0, mapTop = 0, mapRight = 0, mapBottom = 0;
		float mapScale = 1;
    };

    void BuildGeometry(const protocol::RenderFrame& frame, std::vector<Vertex>& vertices, std::vector<DrawRange>& ranges);
	struct MapGeometryRange { std::uint32_t start = 0, count = 0; std::string textureHash; };
    void AppendMapGeometry(std::vector<Vertex>& vertices, std::vector<MapGeometryRange>& ranges, const std::string& viewportId, float left, float top, float right, float bottom, float previewScale);
	bool EnsureMapGeometryBuffer(const std::string& viewportId, float left, float top, float right, float bottom, float previewScale);
	bool EnsureMapTexture(RetainedMapScene& scene, const std::string& hash);
    void AppendQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                    float u1, float v1, float u2, float v2,
                    float r, float g, float b, float a,
                    float rectX, float rectY, float rectW, float rectH,
                    float drawType, float glow, float isGlass, float radius,
                    float shadowOffsetX, float shadowOffsetY, float shadowSoftness, float padding);
    void AppendLineQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2, float thickness,
                        float r, float g, float b, float a);
    void AppendRect(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                    float r, float g, float b, float a,
                    float drawType, float glow, float isGlass, float radius,
                    float shadowOffsetX, float shadowOffsetY, float shadowSoftness, float padding);
    void AppendTextGlyph(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                         float u1, float v1, float u2, float v2, float r, float g, float b, float a);
    void AppendRaycasterQuad(std::vector<Vertex>& vertices, const protocol::DrawCommand& cmd, float x1, float y1, float x2, float y2,
                             RaycasterState& state);
    void AppendBillboardQuad(std::vector<Vertex>& vertices, const protocol::DrawCommand& cmd, const RaycasterState& state);
    void AppendImageQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2);
    void UpdateRaycasterMap(const std::string& mapText);
    void EnsureVertexCapacity(std::size_t vertexCount);
    bool EnsureImageTexture(int width, int height, const std::vector<std::uint8_t>& bytes);
    bool CreateDeviceAndSwapchain(HWND hwnd, int width, int height);
    bool CreateShaders();
    bool CreateAtlas(const protocol::InitEngine& init);
    bool CreateMapBuffer();
    void RecreateRenderTarget();

    HWND hwnd_ = nullptr;
    int width_ = 0;
    int height_ = 0;
    float scaleFactor_ = 1.0f;

    Microsoft::WRL::ComPtr<ID3D11Device> device_;
    Microsoft::WRL::ComPtr<ID3D11DeviceContext> context_;
    Microsoft::WRL::ComPtr<IDXGISwapChain> swapchain_;
    Microsoft::WRL::ComPtr<ID3D11RenderTargetView> rtv_;
    Microsoft::WRL::ComPtr<ID3D11VertexShader> vertexShader_;
    Microsoft::WRL::ComPtr<ID3D11PixelShader> pixelShader_;
    Microsoft::WRL::ComPtr<ID3D11InputLayout> inputLayout_;
    Microsoft::WRL::ComPtr<ID3D11Buffer> vertexBuffer_;
    Microsoft::WRL::ComPtr<ID3D11Buffer> constantBuffer_;
    Microsoft::WRL::ComPtr<ID3D11Buffer> mapBuffer_;
    Microsoft::WRL::ComPtr<ID3D11BlendState> blendState_;
    Microsoft::WRL::ComPtr<ID3D11RasterizerState> rasterizer_;
    Microsoft::WRL::ComPtr<ID3D11SamplerState> sampler_;
    Microsoft::WRL::ComPtr<ID3D11ShaderResourceView> atlasSrv_;
    Microsoft::WRL::ComPtr<ID3D11ShaderResourceView> mapSrv_;
    Microsoft::WRL::ComPtr<ID3D11Texture2D> imageTexture_;
    Microsoft::WRL::ComPtr<ID3D11ShaderResourceView> imageSrv_;

    std::unordered_map<std::uint32_t, GlyphInfo> glyphs_;
    std::array<std::uint32_t, 64 * 64> mapCells_{};
    std::string mapSignature_;
    int imageTextureWidth_ = 0;
    int imageTextureHeight_ = 0;

    struct RetainedMapScene {
        std::uint64_t generation = 0;
        protocol::MapCamera camera{};
        // Solar position for the scene (degrees). Extruded geometry is shaded
        // against it; the defaults are the engine's north-west key light.
        float sunAzimuth = 315.0f;
        float sunElevation = 45.0f;
        // Height fog: density 0 disables it.
        float fogDensity = 0.0f;
        float fogRed = 1.0f, fogGreen = 1.0f, fogBlue = 1.0f;
        std::vector<protocol::MapDrawBatch> draws;
        std::unordered_map<std::string, protocol::MapSceneResource> resources;
		Microsoft::WRL::ComPtr<ID3D11Buffer> vertexBuffer;
		std::vector<MapGeometryRange> geometryRanges;
		std::unordered_map<std::string, Microsoft::WRL::ComPtr<ID3D11ShaderResourceView>> textures;
		std::uint32_t vertexCount = 0;
		float left = 0, top = 0, right = 0, bottom = 0;
		float previewScale = 1;
    };
    std::unordered_map<std::string, RetainedMapScene> mapScenes_;
};

} // namespace poem
