#pragma once
#include "poem/protocol.h"
#include <d3d12.h>
#include <wrl/client.h>
#include <map>
#include <vector>

namespace poem {
class RealtimeOverlay {
public:
    bool UpdateAtlas(const protocol::InitEngine&);
    bool Record(ID3D12Device*,ID3D12GraphicsCommandList*,const protocol::RenderFrame&,int,int,D3D12_RECT);
private:
    struct Vertex{float x,y,u,v,r,g,b,a,texture;};
    bool EnsureGpu(ID3D12Device*,ID3D12GraphicsCommandList*);
    protocol::InitEngine init_;
    std::map<std::uint32_t,protocol::CharInfo> glyphs_;
    Microsoft::WRL::ComPtr<ID3D12RootSignature> root_;
    Microsoft::WRL::ComPtr<ID3D12PipelineState> pipeline_;
    Microsoft::WRL::ComPtr<ID3D12Resource> vertex_,atlas_,atlasUpload_;
    Microsoft::WRL::ComPtr<ID3D12DescriptorHeap> srv_;
    std::size_t capacity_{};
    bool ready_{};
};
}
