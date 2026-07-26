#include "realtime_overlay.h"
#include <d3dcompiler.h>
#include <algorithm>
#include <cmath>
#include <cstring>
#ifdef DrawText
#undef DrawText
#endif

using Microsoft::WRL::ComPtr;
namespace poem {
namespace {
constexpr char shader[]=R"(
cbuffer Screen:register(b0){float2 size;} Texture2D atlas:register(t0);SamplerState samp:register(s0);
struct I{float2 p:POSITION;float2 uv:TEXCOORD0;float4 c:COLOR0;float t:TEXCOORD1;};
struct O{float4 p:SV_POSITION;float2 uv:TEXCOORD0;float4 c:COLOR0;float t:TEXCOORD1;};
O VS(I i){O o;o.p=float4(i.p.x/size.x*2-1,1-i.p.y/size.y*2,0,1);o.uv=i.uv;o.c=i.c;o.t=i.t;return o;}
float4 PS(O i):SV_TARGET{float a=i.t>.5?atlas.Sample(samp,i.uv).a:1;return float4(i.c.rgb,i.c.a*a);})";
ComPtr<ID3DBlob> compile(const char* e,const char* t){ComPtr<ID3DBlob>b,x;if(FAILED(D3DCompile(shader,sizeof(shader),nullptr,nullptr,nullptr,e,t,0,0,&b,&x)))return{};return b;}
std::vector<std::uint32_t> utf8(const std::string& s){std::vector<std::uint32_t>o;for(std::size_t i=0;i<s.size();){auto c=(std::uint8_t)s[i];std::uint32_t p;std::size_t n;if(c<128){p=c;n=1;}else if((c&224)==192){p=c&31;n=2;}else if((c&240)==224){p=c&15;n=3;}else{p=c&7;n=4;}if(i+n>s.size()){o.push_back('?');break;}for(std::size_t j=1;j<n;++j)p=(p<<6)|((std::uint8_t)s[i+j]&63);o.push_back(p);i+=n;}return o;}
}
bool RealtimeOverlay::UpdateAtlas(const protocol::InitEngine& value){init_=value;glyphs_.clear();for(const auto& c:value.chars)glyphs_[(std::uint32_t)c.r]=c;ready_=false;return true;}
bool RealtimeOverlay::EnsureGpu(ID3D12Device* d,ID3D12GraphicsCommandList* l){
    if(ready_)return true;if(init_.atlasWidth<=0||init_.atlasHeight<=0||init_.atlasPixels.empty())return false;
    auto vs=compile("VS","vs_5_1"),ps=compile("PS","ps_5_1");if(!vs||!ps)return false;
    D3D12_DESCRIPTOR_RANGE range{D3D12_DESCRIPTOR_RANGE_TYPE_SRV,1};D3D12_ROOT_PARAMETER p[2]{};
    p[0].ParameterType=D3D12_ROOT_PARAMETER_TYPE_32BIT_CONSTANTS;p[0].Constants={0,0,2};p[0].ShaderVisibility=D3D12_SHADER_VISIBILITY_VERTEX;
    p[1].ParameterType=D3D12_ROOT_PARAMETER_TYPE_DESCRIPTOR_TABLE;p[1].DescriptorTable={1,&range};p[1].ShaderVisibility=D3D12_SHADER_VISIBILITY_PIXEL;
    D3D12_STATIC_SAMPLER_DESC sampler{};sampler.Filter=D3D12_FILTER_MIN_MAG_MIP_LINEAR;sampler.AddressU=sampler.AddressV=sampler.AddressW=D3D12_TEXTURE_ADDRESS_MODE_CLAMP;sampler.ShaderVisibility=D3D12_SHADER_VISIBILITY_PIXEL;
    D3D12_ROOT_SIGNATURE_DESC rd{2,p,1,&sampler,D3D12_ROOT_SIGNATURE_FLAG_ALLOW_INPUT_ASSEMBLER_INPUT_LAYOUT};ComPtr<ID3DBlob>b,e;
    if(FAILED(D3D12SerializeRootSignature(&rd,D3D_ROOT_SIGNATURE_VERSION_1,&b,&e))||FAILED(d->CreateRootSignature(0,b->GetBufferPointer(),b->GetBufferSize(),IID_PPV_ARGS(&root_))))return false;
    D3D12_INPUT_ELEMENT_DESC il[]={{"POSITION",0,DXGI_FORMAT_R32G32_FLOAT,0,0},{"TEXCOORD",0,DXGI_FORMAT_R32G32_FLOAT,0,8},{"COLOR",0,DXGI_FORMAT_R32G32B32A32_FLOAT,0,16},{"TEXCOORD",1,DXGI_FORMAT_R32_FLOAT,0,32}};
    D3D12_GRAPHICS_PIPELINE_STATE_DESC pd{};pd.pRootSignature=root_.Get();pd.VS={vs->GetBufferPointer(),vs->GetBufferSize()};pd.PS={ps->GetBufferPointer(),ps->GetBufferSize()};pd.InputLayout={il,4};pd.SampleMask=UINT_MAX;
    pd.RasterizerState.FillMode=D3D12_FILL_MODE_SOLID;pd.RasterizerState.CullMode=D3D12_CULL_MODE_NONE;pd.RasterizerState.DepthClipEnable=TRUE;auto& blend=pd.BlendState.RenderTarget[0];blend.BlendEnable=TRUE;blend.SrcBlend=D3D12_BLEND_SRC_ALPHA;blend.DestBlend=D3D12_BLEND_INV_SRC_ALPHA;blend.BlendOp=D3D12_BLEND_OP_ADD;blend.SrcBlendAlpha=D3D12_BLEND_ONE;blend.DestBlendAlpha=D3D12_BLEND_INV_SRC_ALPHA;blend.BlendOpAlpha=D3D12_BLEND_OP_ADD;blend.RenderTargetWriteMask=15;
    pd.PrimitiveTopologyType=D3D12_PRIMITIVE_TOPOLOGY_TYPE_TRIANGLE;pd.NumRenderTargets=1;pd.RTVFormats[0]=DXGI_FORMAT_R8G8B8A8_UNORM;pd.SampleDesc.Count=1;if(FAILED(d->CreateGraphicsPipelineState(&pd,IID_PPV_ARGS(&pipeline_))))return false;
    D3D12_DESCRIPTOR_HEAP_DESC hd{D3D12_DESCRIPTOR_HEAP_TYPE_CBV_SRV_UAV,1,D3D12_DESCRIPTOR_HEAP_FLAG_SHADER_VISIBLE};if(FAILED(d->CreateDescriptorHeap(&hd,IID_PPV_ARGS(&srv_))))return false;
    D3D12_RESOURCE_DESC td{};td.Dimension=D3D12_RESOURCE_DIMENSION_TEXTURE2D;td.Width=init_.atlasWidth;td.Height=init_.atlasHeight;td.DepthOrArraySize=1;td.MipLevels=1;td.Format=DXGI_FORMAT_R8G8B8A8_UNORM;td.SampleDesc.Count=1;
    D3D12_HEAP_PROPERTIES hp{D3D12_HEAP_TYPE_DEFAULT};if(FAILED(d->CreateCommittedResource(&hp,D3D12_HEAP_FLAG_NONE,&td,D3D12_RESOURCE_STATE_COPY_DEST,nullptr,IID_PPV_ARGS(&atlas_))))return false;
    D3D12_PLACED_SUBRESOURCE_FOOTPRINT fp{};UINT rows{};UINT64 rb{},size{};d->GetCopyableFootprints(&td,0,1,0,&fp,&rows,&rb,&size);D3D12_RESOURCE_DESC bd{D3D12_RESOURCE_DIMENSION_BUFFER,0,size,1,1,1,DXGI_FORMAT_UNKNOWN,{1,0},D3D12_TEXTURE_LAYOUT_ROW_MAJOR};
    hp.Type=D3D12_HEAP_TYPE_UPLOAD;if(FAILED(d->CreateCommittedResource(&hp,D3D12_HEAP_FLAG_NONE,&bd,D3D12_RESOURCE_STATE_GENERIC_READ,nullptr,IID_PPV_ARGS(&atlasUpload_))))return false;std::uint8_t* m{};atlasUpload_->Map(0,nullptr,(void**)&m);
    for(int y=0;y<init_.atlasHeight;++y)std::memcpy(m+fp.Offset+y*fp.Footprint.RowPitch,init_.atlasPixels.data()+y*init_.atlasWidth*4,init_.atlasWidth*4);atlasUpload_->Unmap(0,nullptr);
    D3D12_TEXTURE_COPY_LOCATION dst{atlas_.Get(),D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX},src{atlasUpload_.Get(),D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT};src.PlacedFootprint=fp;l->CopyTextureRegion(&dst,0,0,0,&src,nullptr);
    D3D12_RESOURCE_BARRIER barrier{};barrier.Type=D3D12_RESOURCE_BARRIER_TYPE_TRANSITION;barrier.Transition={atlas_.Get(),D3D12_RESOURCE_BARRIER_ALL_SUBRESOURCES,D3D12_RESOURCE_STATE_COPY_DEST,D3D12_RESOURCE_STATE_PIXEL_SHADER_RESOURCE};l->ResourceBarrier(1,&barrier);
    D3D12_SHADER_RESOURCE_VIEW_DESC sd{};sd.Format=DXGI_FORMAT_R8G8B8A8_UNORM;sd.ViewDimension=D3D12_SRV_DIMENSION_TEXTURE2D;sd.Shader4ComponentMapping=D3D12_DEFAULT_SHADER_4_COMPONENT_MAPPING;sd.Texture2D.MipLevels=1;d->CreateShaderResourceView(atlas_.Get(),&sd,srv_->GetCPUDescriptorHandleForHeapStart());ready_=true;return true;
}
bool RealtimeOverlay::Record(ID3D12Device*d,ID3D12GraphicsCommandList*l,const protocol::RenderFrame& f,int width,int height){
    if(!EnsureGpu(d,l))return false;std::vector<Vertex> vertices;float ox{},oy{};auto quad=[&](float x1,float y1,float x2,float y2,float u1,float v1,float u2,float v2,float r,float g,float b,float a,float t){Vertex q[]={{x1,y1,u1,v1,r,g,b,a,t},{x2,y1,u2,v1,r,g,b,a,t},{x1,y2,u1,v2,r,g,b,a,t},{x1,y2,u1,v2,r,g,b,a,t},{x2,y1,u2,v1,r,g,b,a,t},{x2,y2,u2,v2,r,g,b,a,t}};vertices.insert(vertices.end(),q,q+6);};
    for(const auto& c:f.commands){if(c.type==protocol::DrawCommandType::DrawRealtimeViewport)continue;if(c.type==protocol::DrawCommandType::SetOffset){ox=c.val1;oy=c.val2;continue;}float r=c.r/255.f,g=c.g/255.f,b=c.b/255.f,a=c.a/255.f,x1=c.x1+ox,y1=c.y1+oy,x2=c.x2+ox,y2=c.y2+oy;
        if((c.type==protocol::DrawCommandType::FillRect||c.type==protocol::DrawCommandType::DrawRoundedRect)&&x1<=0&&y1<=0&&x2>=width&&y2>=height)continue;
        if(c.type==protocol::DrawCommandType::FillRect||c.type==protocol::DrawCommandType::DrawRoundedRect)quad(x1,y1,x2,y2,0,0,0,0,r,g,b,a,0);
        else if(c.type==protocol::DrawCommandType::DrawLine){float dx=x2-x1,dy=y2-y1,n=std::sqrt(dx*dx+dy*dy);if(n>.01f){float nx=-dy/n*.75f,ny=dx/n*.75f;quad(x1+nx,y1+ny,x2-nx,y2-ny,0,0,0,0,r,g,b,a,0);}}
        else if(c.type==protocol::DrawCommandType::DrawText){float pen=x1;for(auto cp:utf8(c.text)){auto it=glyphs_.find(cp);if(it==glyphs_.end())it=glyphs_.find('?');if(it==glyphs_.end())continue;auto& gl=it->second;float top=y1-gl.height*.78f;quad(pen,top,pen+gl.width,top+gl.height,gl.u1,gl.v1,gl.u2,gl.v2,r,g,b,a,1);pen+=gl.advance;}}}
    if(vertices.empty())return true;auto bytes=vertices.size()*sizeof(Vertex);if(bytes>capacity_){D3D12_HEAP_PROPERTIES hp{D3D12_HEAP_TYPE_UPLOAD};D3D12_RESOURCE_DESC bd{D3D12_RESOURCE_DIMENSION_BUFFER,0,bytes,1,1,1,DXGI_FORMAT_UNKNOWN,{1,0},D3D12_TEXTURE_LAYOUT_ROW_MAJOR};if(FAILED(d->CreateCommittedResource(&hp,D3D12_HEAP_FLAG_NONE,&bd,D3D12_RESOURCE_STATE_GENERIC_READ,nullptr,IID_PPV_ARGS(&vertex_))))return false;capacity_=bytes;}
    void*m{};vertex_->Map(0,nullptr,&m);std::memcpy(m,vertices.data(),bytes);vertex_->Unmap(0,nullptr);float screen[]{(float)width,(float)height};l->SetPipelineState(pipeline_.Get());l->SetGraphicsRootSignature(root_.Get());l->SetGraphicsRoot32BitConstants(0,2,screen,0);ID3D12DescriptorHeap* heaps[]{srv_.Get()};l->SetDescriptorHeaps(1,heaps);l->SetGraphicsRootDescriptorTable(1,srv_->GetGPUDescriptorHandleForHeapStart());D3D12_VERTEX_BUFFER_VIEW view{vertex_->GetGPUVirtualAddress(),(UINT)bytes,sizeof(Vertex)};l->IASetVertexBuffers(0,1,&view);l->IASetPrimitiveTopology(D3D_PRIMITIVE_TOPOLOGY_TRIANGLELIST);l->DrawInstanced((UINT)vertices.size(),1,0,0);return true;
}
}
