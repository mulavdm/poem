#pragma once
#include "renderer_d3d11.h"
#include "renderer_realtime_d3d12.h"
#include <memory>

namespace poem {
class Renderer {
public:
    bool Initialize(HWND hwnd,int width,int height,const protocol::InitEngine& init);
    bool UpdateFontAtlas(const protocol::InitEngine& init);
    void Resize(int width,int height);
    void ApplyMapScene(const protocol::MapSceneDelta& scene);
    void Render(const protocol::RenderFrame& frame);
    void Present();
    void RealtimeKey(std::uint32_t key,bool down);
    std::vector<protocol::Event> DrainRealtimeEvents();
    bool CaptureBackbufferRGBA(std::vector<std::uint8_t>& rgba,int& width,int& height);
    int BackbufferWidth() const;
    int BackbufferHeight() const;
    bool Realtime() const { return realtime_!=nullptr; }
private:
    std::unique_ptr<RendererD3D11> d3d11_;
    std::unique_ptr<RendererRealtimeD3D12> realtime_;
};
}
