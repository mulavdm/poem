#include "../src/realtime_coordinates.h"
#include <iostream>

static void require(bool value, int check) {
    if (!value) {
        std::cerr << "realtime coordinate contract check " << check << " failed\n";
        std::exit(check);
    }
}
int main(){
    using namespace poem;protocol::RenderFrame frame;frame.width=1000;frame.height=600;
    protocol::DrawCommand offset;offset.type=protocol::DrawCommandType::SetOffset;offset.val1=20;offset.val2=10;
    protocol::DrawCommand clip;clip.type=protocol::DrawCommandType::SetClip;clip.flag=true;clip.x1=100;clip.y1=50;clip.w=600;clip.h=400;
    protocol::DrawCommand viewport;viewport.type=protocol::DrawCommandType::DrawRealtimeViewport;viewport.x1=50;viewport.y1=20;viewport.x2=800;viewport.y2=500;
    frame.commands={offset,clip,viewport};
    for(const auto [width,height]:{std::pair{1000,600},std::pair{1250,750},std::pair{1500,900},std::pair{2000,1200}}){
        const auto resolved=ResolveRealtimeViewport(frame,width,height);const float scale=static_cast<float>(width)/1000;
        const auto expectedX=static_cast<int>(std::lround(100*scale));
        const auto expectedY=static_cast<int>(std::lround(50*scale));
        if (!resolved.command || resolved.rect.x != expectedX || resolved.rect.y != expectedY) {
            std::cerr << "resolved origin " << resolved.rect.x << "," << resolved.rect.y
                      << " expected " << expectedX << "," << expectedY << "\n";
            return 1;
        }
        require(resolved.rect.width==static_cast<int>(std::lround(600*scale))&&resolved.rect.height==static_cast<int>(std::lround(400*scale)),2);
    }
    frame.commands.clear();require(!ResolveRealtimeViewport(frame,1000,600).command,3);
    viewport.x1=1200;viewport.x2=1300;frame.commands={viewport};const auto outside=ResolveRealtimeViewport(frame,1000,600);
    require(outside.command&&outside.rect.width==0&&outside.rect.height==0,4);
}
