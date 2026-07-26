#include "renderer.h"
#include <windows.h>

namespace poem {
bool Renderer::Initialize(HWND hwnd,int width,int height,const protocol::InitEngine& init) {
    wchar_t path[32768]{};
    if(GetEnvironmentVariableW(L"POEM_REALTIME_VIEWPORT_DLL",path,32768)>0) {
        realtime_=std::make_unique<RendererRealtimeD3D12>();
        if(realtime_->Initialize(hwnd,width,height,path)) return true;
        realtime_.reset();
        return false; // Explicit opt-in must fail loudly, never fall back silently.
    }
    d3d11_=std::make_unique<RendererD3D11>();
    return d3d11_->Initialize(hwnd,width,height,init);
}
bool Renderer::UpdateFontAtlas(const protocol::InitEngine& init){return realtime_?true:d3d11_->UpdateFontAtlas(init);}
void Renderer::Resize(int w,int h){if(realtime_)realtime_->Resize(w,h);else if(d3d11_)d3d11_->Resize(w,h);}
void Renderer::ApplyMapScene(const protocol::MapSceneDelta& s){if(d3d11_)d3d11_->ApplyMapScene(s);}
void Renderer::Render(const protocol::RenderFrame& f){if(realtime_)realtime_->Render(f);else d3d11_->Render(f);}
void Renderer::Present(){if(realtime_)realtime_->Present();else d3d11_->Present();}
void Renderer::RealtimeKey(std::uint32_t key,bool down){if(realtime_)realtime_->Key(key,down);}
bool Renderer::CaptureBackbufferRGBA(std::vector<std::uint8_t>& b,int& w,int& h){
    return realtime_?realtime_->CaptureBackbufferRGBA(b,w,h):d3d11_->CaptureBackbufferRGBA(b,w,h);
}
int Renderer::BackbufferWidth()const{return realtime_?realtime_->BackbufferWidth():d3d11_->BackbufferWidth();}
int Renderer::BackbufferHeight()const{return realtime_?realtime_->BackbufferHeight():d3d11_->BackbufferHeight();}
}
