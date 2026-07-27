#include "renderer_realtime_d3d12.h"
#include "realtime_coordinates.h"
#include <algorithm>
#include <cmath>
#include <cstdio>
#include <cstring>

using Microsoft::WRL::ComPtr;
namespace poem {
RendererRealtimeD3D12::~RendererRealtimeD3D12(){
    Wait();
    if(viewport_&&viewport_->shutdown)viewport_->shutdown(viewport_->userData);
    if(fenceEvent_)CloseHandle(fenceEvent_);
    // Viewport modules may own language runtimes or worker state; match POEM's
    // Go DLL rule and leave a successfully loaded module resident until exit.
}
bool RendererRealtimeD3D12::Initialize(HWND hwnd,int width,int height,const wchar_t* path,const protocol::InitEngine& init){
    if(!hwnd||width<=0||height<=0||!path||!*path)return false;
    hwnd_=hwnd;width_=width;height_=height;
    module_=LoadLibraryW(path);
    if(!module_){std::fprintf(stderr,"POEM realtime viewport DLL could not be loaded\n");return false;}
    const auto get=reinterpret_cast<ExportFn>(GetProcAddress(module_,"HamsterRealtimeViewportExports"));
    viewport_=get?get():nullptr;
    if(realtime::Validate(viewport_)!=realtime::Result::ok){std::fprintf(stderr,"POEM realtime viewport ABI rejected\n");return false;}
    if(FAILED(CreateDXGIFactory2(0,IID_PPV_ARGS(&factory_))))return false;
    ComPtr<IDXGIAdapter1> adapter;
    for(UINT i=0;factory_->EnumAdapters1(i,&adapter)!=DXGI_ERROR_NOT_FOUND;++i){
        DXGI_ADAPTER_DESC1 desc{};adapter->GetDesc1(&desc);
        if(!(desc.Flags&DXGI_ADAPTER_FLAG_SOFTWARE)&&
           SUCCEEDED(D3D12CreateDevice(adapter.Get(),D3D_FEATURE_LEVEL_12_0,IID_PPV_ARGS(&device_))))break;
        adapter.Reset();
    }
    if(!device_&&FAILED(D3D12CreateDevice(nullptr,D3D_FEATURE_LEVEL_11_0,IID_PPV_ARGS(&device_))))return false;
    D3D12_COMMAND_QUEUE_DESC queueDesc{};
    if(FAILED(device_->CreateCommandQueue(&queueDesc,IID_PPV_ARGS(&queue_))))return false;
    DXGI_SWAP_CHAIN_DESC1 swapDesc{};swapDesc.Width=width_;swapDesc.Height=height_;
    swapDesc.Format=DXGI_FORMAT_R8G8B8A8_UNORM;swapDesc.BufferUsage=DXGI_USAGE_RENDER_TARGET_OUTPUT;
    swapDesc.BufferCount=frameCount_;swapDesc.SwapEffect=DXGI_SWAP_EFFECT_FLIP_DISCARD;swapDesc.SampleDesc.Count=1;
    ComPtr<IDXGISwapChain1> base;
    if(FAILED(factory_->CreateSwapChainForHwnd(queue_.Get(),hwnd_,&swapDesc,nullptr,nullptr,&base))||
       FAILED(base.As(&swap_)))return false;
    D3D12_DESCRIPTOR_HEAP_DESC heap{};heap.NumDescriptors=frameCount_;heap.Type=D3D12_DESCRIPTOR_HEAP_TYPE_RTV;
    if(FAILED(device_->CreateDescriptorHeap(&heap,IID_PPV_ARGS(&rtvHeap_))))return false;
    D3D12_DESCRIPTOR_HEAP_DESC dsvHeapDesc{};dsvHeapDesc.NumDescriptors=1;dsvHeapDesc.Type=D3D12_DESCRIPTOR_HEAP_TYPE_DSV;
    if(FAILED(device_->CreateDescriptorHeap(&dsvHeapDesc,IID_PPV_ARGS(&dsvHeap_))))return false;
    for(auto& allocator:allocators_)
        if(FAILED(device_->CreateCommandAllocator(D3D12_COMMAND_LIST_TYPE_DIRECT,IID_PPV_ARGS(&allocator))))return false;
    if(FAILED(device_->CreateCommandList(0,D3D12_COMMAND_LIST_TYPE_DIRECT,allocators_[0].Get(),nullptr,IID_PPV_ARGS(&list_))))return false;
    list_->Close();
    if(FAILED(device_->CreateFence(0,D3D12_FENCE_FLAG_NONE,IID_PPV_ARGS(&fence_))))return false;
    fenceEvent_=CreateEventW(nullptr,FALSE,FALSE,nullptr);if(!fenceEvent_||!CreateTargets())return false;
    realtime::FrameInput input{sizeof(input),0,0,{0,0,width_,height_},device_.Get(),queue_.Get(),list_.Get(),
        DXGI_FORMAT_R8G8B8A8_UNORM,0,DXGI_FORMAT_D32_FLOAT};
    overlay_.UpdateAtlas(init);
    return viewport_->initialize(viewport_->userData,&input)==realtime::Result::ok;
}
bool RendererRealtimeD3D12::CreateTargets(){
    index_=swap_->GetCurrentBackBufferIndex();auto handle=rtvHeap_->GetCPUDescriptorHandleForHeapStart();
    const auto step=device_->GetDescriptorHandleIncrementSize(D3D12_DESCRIPTOR_HEAP_TYPE_RTV);
    for(UINT i=0;i<frameCount_;++i){if(FAILED(swap_->GetBuffer(i,IID_PPV_ARGS(&targets_[i]))))return false;
        device_->CreateRenderTargetView(targets_[i].Get(),nullptr,handle);handle.ptr+=step;}
    depthTarget_.Reset();
    D3D12_HEAP_PROPERTIES depthHeap{D3D12_HEAP_TYPE_DEFAULT};
    D3D12_RESOURCE_DESC depthDesc{};depthDesc.Dimension=D3D12_RESOURCE_DIMENSION_TEXTURE2D;
    depthDesc.Width=static_cast<UINT64>(width_);depthDesc.Height=static_cast<UINT>(height_);
    depthDesc.DepthOrArraySize=1;depthDesc.MipLevels=1;depthDesc.Format=DXGI_FORMAT_D32_FLOAT;
    depthDesc.SampleDesc.Count=1;depthDesc.Flags=D3D12_RESOURCE_FLAG_ALLOW_DEPTH_STENCIL;
    D3D12_CLEAR_VALUE depthClear{};depthClear.Format=DXGI_FORMAT_D32_FLOAT;depthClear.DepthStencil={1.0f,0};
    if(FAILED(device_->CreateCommittedResource(&depthHeap,D3D12_HEAP_FLAG_NONE,&depthDesc,
        D3D12_RESOURCE_STATE_DEPTH_WRITE,&depthClear,IID_PPV_ARGS(&depthTarget_))))return false;
    device_->CreateDepthStencilView(depthTarget_.Get(),nullptr,dsvHeap_->GetCPUDescriptorHandleForHeapStart());
    return true;
}
bool RendererRealtimeD3D12::Wait(){
    if(!queue_||!fence_)return true;const auto value=++fenceValue_;
    if(FAILED(queue_->Signal(fence_.Get(),value)))return false;
    if(fence_->GetCompletedValue()<value){if(FAILED(fence_->SetEventOnCompletion(value,fenceEvent_)))return false;
        return WaitForSingleObject(fenceEvent_,2000)==WAIT_OBJECT_0;}return true;
}
void RendererRealtimeD3D12::Resize(int width,int height){
    if(width<=0||height<=0||(width==width_&&height==height_)||!Wait())return;
    for(auto& target:targets_)target.Reset();
    if(FAILED(swap_->ResizeBuffers(frameCount_,width,height,DXGI_FORMAT_R8G8B8A8_UNORM,0))) {
        viewport_->deviceLost(viewport_->userData);return;
    }
    width_=width;height_=height;hasPresentedFrame_=false;
    if(CreateTargets())viewport_->resize(viewport_->userData,{0,0,width_,height_});
}
void RendererRealtimeD3D12::Render(const protocol::RenderFrame& frame){
    const auto resolved=ResolveRealtimeViewport(frame,width_,height_);viewportRect_=resolved.rect;
    const protocol::DrawCommand* viewportCommand=resolved.command;
    if(viewportCommand){viewportTarget_=viewportCommand->text;viewportFocused_=viewportCommand->flag;}
    if(viewport_->abiVersion>=realtime::kABIVersion&&viewportCommand&&!viewportCommand->bytes.empty()) {
        realtime::Command command{sizeof(command),viewportCommand->bytes.data(),
            static_cast<std::uint32_t>(viewportCommand->bytes.size())};
        if(realtime::Validate(&command)!=realtime::Result::ok||
           viewport_->submitCommand(viewport_->userData,&command)!=realtime::Result::ok)return;
    }
    index_=swap_->GetCurrentBackBufferIndex();
    if(FAILED(allocators_[index_]->Reset())||FAILED(list_->Reset(allocators_[index_].Get(),nullptr)))return;
    D3D12_RESOURCE_BARRIER barrier{};barrier.Type=D3D12_RESOURCE_BARRIER_TYPE_TRANSITION;
    barrier.Transition={targets_[index_].Get(),D3D12_RESOURCE_BARRIER_ALL_SUBRESOURCES,
        D3D12_RESOURCE_STATE_PRESENT,D3D12_RESOURCE_STATE_RENDER_TARGET};
    list_->ResourceBarrier(1,&barrier);
    auto handle=rtvHeap_->GetCPUDescriptorHandleForHeapStart();
    handle.ptr+=index_*device_->GetDescriptorHandleIncrementSize(D3D12_DESCRIPTOR_HEAP_TYPE_RTV);
    const auto depthHandle=dsvHeap_->GetCPUDescriptorHandleForHeapStart();
    const float clear[]{0.025f,0.04f,0.075f,1};list_->OMSetRenderTargets(1,&handle,FALSE,&depthHandle);
    list_->ClearRenderTargetView(handle,clear,0,nullptr);
    list_->ClearDepthStencilView(depthHandle,D3D12_CLEAR_FLAG_DEPTH,1.0f,0,0,nullptr);
    realtime::FrameInput input{sizeof(input),frameID_++,1.0f/60,viewportRect_,device_.Get(),queue_.Get(),list_.Get(),
        DXGI_FORMAT_R8G8B8A8_UNORM,viewportFocused_?1u:0u,DXGI_FORMAT_D32_FLOAT};
    if(viewportCommand&&viewportRect_.width>0&&viewportRect_.height>0&&
       viewport_->render(viewport_->userData,&input)!=realtime::Result::ok)return;
    const D3D12_VIEWPORT fullViewport{0,0,static_cast<float>(width_),static_cast<float>(height_),0,1};
    const D3D12_RECT fullScissor{0,0,width_,height_};list_->RSSetViewports(1,&fullViewport);list_->RSSetScissorRects(1,&fullScissor);
    list_->OMSetRenderTargets(1,&handle,FALSE,nullptr);
    const D3D12_RECT engineRect{viewportRect_.x,viewportRect_.y,viewportRect_.x+viewportRect_.width,viewportRect_.y+viewportRect_.height};
    if(!overlay_.Record(device_.Get(),list_.Get(),frame,width_,height_,engineRect))return;
    std::swap(barrier.Transition.StateBefore,barrier.Transition.StateAfter);list_->ResourceBarrier(1,&barrier);
    if(FAILED(list_->Close()))return;ID3D12CommandList* lists[]{list_.Get()};queue_->ExecuteCommandLists(1,lists);frameReady_=true;
}
std::vector<protocol::Event> RendererRealtimeD3D12::DrainEvents(){
    std::vector<protocol::Event> events;
    if(!viewport_||viewport_->abiVersion<realtime::kABIVersion||!viewport_->pollEvent)return events;
    std::vector<std::uint8_t> storage(realtime::kMaxMessageBytes);
    for(std::size_t count=0;count<64;++count){
        realtime::EventBuffer buffer{sizeof(buffer),storage.data(),static_cast<std::uint32_t>(storage.size()),0};
        const auto result=viewport_->pollEvent(viewport_->userData,&buffer);
        if(result!=realtime::Result::ok||buffer.byteCount==0)break;
        if(realtime::Validate(&buffer)!=realtime::Result::ok)break;
        protocol::Event event;event.type=protocol::EventType::RealtimeViewport;
        event.target=viewportTarget_;event.bytes.assign(storage.begin(),storage.begin()+buffer.byteCount);
        events.push_back(std::move(event));
    }
    return events;
}
void RendererRealtimeD3D12::Present(){
    if(!frameReady_)return;frameReady_=false;
    const HRESULT hr=swap_->Present(1,0);
    if(FAILED(hr)){viewport_->deviceLost(viewport_->userData);return;}
    hasPresentedFrame_=true;
    Wait();
}
void RendererRealtimeD3D12::Key(std::uint32_t key,bool down){
    if(!viewport_||!viewport_->action||!viewportFocused_)return;
    realtime::Action action{};
    if(key==VK_LEFT||key=='A')action=realtime::Action::moveLeft;
    else if(key==VK_RIGHT||key=='D')action=realtime::Action::moveRight;
    else if(key==VK_RETURN||key==VK_SPACE)action=realtime::Action::confirm;
    else if(key==VK_ESCAPE)action=realtime::Action::cancel;
    else return;
    const realtime::ActionEvent event{sizeof(event),action,down,down?1.0f:0.0f};
    viewport_->action(viewport_->userData,&event);
}
bool RendererRealtimeD3D12::CaptureBackbufferRGBA(std::vector<std::uint8_t>& pixels,int& width,int& height){
    auto fail=[](const char* reason){std::fprintf(stderr,"POEM D3D12 capture failed: %s\n",reason);return false;};
    if(!swap_||!device_||!queue_||!hasPresentedFrame_||width_<=0||height_<=0)return fail("renderer not ready");
    if(!Wait())return fail("initial GPU wait");
    const UINT captureIndex=swap_->GetCurrentBackBufferIndex();
    D3D12_RESOURCE_DESC texture=targets_[captureIndex]->GetDesc();D3D12_PLACED_SUBRESOURCE_FOOTPRINT footprint{};
    UINT rows{};UINT64 rowBytes{},totalBytes{};device_->GetCopyableFootprints(&texture,0,1,0,&footprint,&rows,&rowBytes,&totalBytes);
    D3D12_HEAP_PROPERTIES heap{D3D12_HEAP_TYPE_READBACK};D3D12_RESOURCE_DESC buffer{D3D12_RESOURCE_DIMENSION_BUFFER,0,totalBytes,1,1,1,
        DXGI_FORMAT_UNKNOWN,{1,0},D3D12_TEXTURE_LAYOUT_ROW_MAJOR};ComPtr<ID3D12Resource> readback;
    if(FAILED(device_->CreateCommittedResource(&heap,D3D12_HEAP_FLAG_NONE,&buffer,D3D12_RESOURCE_STATE_COPY_DEST,nullptr,IID_PPV_ARGS(&readback))))return fail("readback allocation");
    if(FAILED(allocators_[captureIndex]->Reset())||FAILED(list_->Reset(allocators_[captureIndex].Get(),nullptr)))return fail("command list reset");
    D3D12_RESOURCE_BARRIER barrier{};barrier.Type=D3D12_RESOURCE_BARRIER_TYPE_TRANSITION;barrier.Transition={targets_[captureIndex].Get(),
        D3D12_RESOURCE_BARRIER_ALL_SUBRESOURCES,D3D12_RESOURCE_STATE_PRESENT,D3D12_RESOURCE_STATE_COPY_SOURCE};list_->ResourceBarrier(1,&barrier);
    D3D12_TEXTURE_COPY_LOCATION source{targets_[captureIndex].Get(),D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX};
    D3D12_TEXTURE_COPY_LOCATION destination{readback.Get(),D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT};destination.PlacedFootprint=footprint;
    list_->CopyTextureRegion(&destination,0,0,0,&source,nullptr);std::swap(barrier.Transition.StateBefore,barrier.Transition.StateAfter);list_->ResourceBarrier(1,&barrier);
    if(FAILED(list_->Close()))return fail("command list close");ID3D12CommandList* lists[]{list_.Get()};queue_->ExecuteCommandLists(1,lists);if(!Wait())return fail("copy GPU wait");
    const std::size_t outputBytes=static_cast<std::size_t>(width_)*height_*4;pixels.resize(outputBytes);std::uint8_t* mapped{};
    const D3D12_RANGE readRange{0,static_cast<SIZE_T>(totalBytes)};if(FAILED(readback->Map(0,&readRange,reinterpret_cast<void**>(&mapped))))return fail("readback map");
    for(int y=0;y<height_;++y)std::memcpy(pixels.data()+static_cast<std::size_t>(y)*width_*4,mapped+footprint.Offset+static_cast<std::size_t>(y)*footprint.Footprint.RowPitch,static_cast<std::size_t>(width_)*4);
    const D3D12_RANGE written{0,0};readback->Unmap(0,&written);width=width_;height=height_;return true;
}
}
