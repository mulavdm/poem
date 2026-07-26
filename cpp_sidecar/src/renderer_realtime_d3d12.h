#pragma once
#include "poem/realtime_viewport.h"
#include "poem/protocol.h"
#include "realtime_overlay.h"
#include <windows.h>
#include <d3d12.h>
#include <dxgi1_6.h>
#include <wrl/client.h>
#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace poem {
class RendererRealtimeD3D12 {
public:
    RendererRealtimeD3D12()=default;
    ~RendererRealtimeD3D12();
    bool Initialize(HWND hwnd,int width,int height,const wchar_t* modulePath,const protocol::InitEngine&);
    bool UpdateFontAtlas(const protocol::InitEngine& value){return overlay_.UpdateAtlas(value);}
    void Resize(int width,int height);
    void Render(const protocol::RenderFrame&);
    void Present();
    void Key(std::uint32_t key,bool down);
    std::vector<protocol::Event> DrainEvents();
    bool CaptureBackbufferRGBA(std::vector<std::uint8_t>&,int&,int&);
    int BackbufferWidth()const{return width_;}
    int BackbufferHeight()const{return height_;}
private:
    static constexpr UINT frameCount_=2;
    using ExportFn=const realtime::Exports*(__cdecl*)();
    bool CreateTargets();
    bool Wait();
    HWND hwnd_{};
    int width_{},height_{};
    UINT index_{};
    std::uint64_t frameID_{},fenceValue_{};
    HMODULE module_{};
    const realtime::Exports* viewport_{};
    HANDLE fenceEvent_{};
    Microsoft::WRL::ComPtr<IDXGIFactory6> factory_;
    Microsoft::WRL::ComPtr<ID3D12Device> device_;
    Microsoft::WRL::ComPtr<ID3D12CommandQueue> queue_;
    Microsoft::WRL::ComPtr<IDXGISwapChain3> swap_;
    Microsoft::WRL::ComPtr<ID3D12DescriptorHeap> rtvHeap_;
    std::array<Microsoft::WRL::ComPtr<ID3D12Resource>,frameCount_> targets_;
    std::array<Microsoft::WRL::ComPtr<ID3D12CommandAllocator>,frameCount_> allocators_;
    Microsoft::WRL::ComPtr<ID3D12GraphicsCommandList> list_;
    Microsoft::WRL::ComPtr<ID3D12Fence> fence_;
    bool frameReady_{};
    bool hasPresentedFrame_{};
    realtime::Rect viewportRect_{};
    std::string viewportTarget_;
    bool viewportFocused_{};
    RealtimeOverlay overlay_;
};
}
