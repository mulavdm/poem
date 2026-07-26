#pragma once
#include "poem/protocol.h"
#include "poem/realtime_viewport.h"
#include <algorithm>
#include <cmath>

namespace poem {
struct ResolvedRealtimeViewport {
    realtime::Rect rect{};
    const protocol::DrawCommand* command{};
};

inline ResolvedRealtimeViewport ResolveRealtimeViewport(const protocol::RenderFrame& frame,int physicalWidth,int physicalHeight) {
    ResolvedRealtimeViewport result;
    if(frame.width<=0||frame.height<=0||physicalWidth<=0||physicalHeight<=0)return result;
    float offsetX=0,offsetY=0;int clipX=0,clipY=0,clipW=frame.width,clipH=frame.height;bool clipped=false;
    const float sx=static_cast<float>(physicalWidth)/frame.width,sy=static_cast<float>(physicalHeight)/frame.height;
    for(const auto& command:frame.commands){
        if(command.type==protocol::DrawCommandType::SetOffset){offsetX=command.val1;offsetY=command.val2;continue;}
        if(command.type==protocol::DrawCommandType::SetClip){clipped=command.flag;if(clipped){clipX=command.x1;clipY=command.y1;clipW=command.w;clipH=command.h;}continue;}
        if(command.type!=protocol::DrawCommandType::DrawRealtimeViewport)continue;
        result.command=&command;int left=static_cast<int>(std::lround((command.x1+offsetX)*sx));
        int top=static_cast<int>(std::lround((command.y1+offsetY)*sy));int right=static_cast<int>(std::lround((command.x2+offsetX)*sx));
        int bottom=static_cast<int>(std::lround((command.y2+offsetY)*sy));
        if(clipped){left=std::max(left,static_cast<int>(std::lround(clipX*sx)));top=std::max(top,static_cast<int>(std::lround(clipY*sy)));
            right=std::min(right,static_cast<int>(std::lround((clipX+clipW)*sx)));bottom=std::min(bottom,static_cast<int>(std::lround((clipY+clipH)*sy)));}
        left=std::clamp(left,0,physicalWidth);top=std::clamp(top,0,physicalHeight);right=std::clamp(right,0,physicalWidth);bottom=std::clamp(bottom,0,physicalHeight);
        if(right>left&&bottom>top)result.rect={left,top,right-left,bottom-top};return result;
    }
    return result;
}
}
