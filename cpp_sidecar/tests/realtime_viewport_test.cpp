#include "poem/realtime_viewport.h"
#include <stdexcept>
using namespace poem::realtime;
static Result Init(void*, const FrameInput*) { return Result::ok; }
static Result Render(void*, const FrameInput*) { return Result::ok; }
static void Resize(void*, Rect) {}
static void Event(void*) {}
static Result Semantics(void*, SemanticSnapshot*) { return Result::ok; }
static Result ActionCallback(void*, const ActionEvent*) { return Result::ok; }
static Result Submit(void*, const Command*) { return Result::ok; }
static Result Poll(void*, EventBuffer*) { return Result::ok; }
static void Require(bool value) { if (!value) throw std::runtime_error("viewport contract"); }
int main() {
    Exports value{sizeof(Exports),kABIVersion,nullptr,Init,Render,Resize,Event,Event,Semantics,ActionCallback,Submit,Poll};
    Require(Validate(&value)==Result::ok);
    value.abiVersion++; Require(Validate(&value)==Result::unsupported_version);
    value.abiVersion=kABIVersion; value.render=nullptr;
    Require(Validate(&value)==Result::invalid_argument);
    value.render=Render; value.submitCommand=nullptr;
    Require(Validate(&value)==Result::invalid_argument);
    value.structSize=offsetof(Exports,submitCommand);
    value.abiVersion=kLegacyABIVersion;
    Require(Validate(&value)==Result::ok);
    SemanticSnapshot snapshot{sizeof(SemanticSnapshot),nullptr,kMaxSemanticBytes+1};
    Require(ValidateSnapshot(&snapshot)==Result::invalid_argument);
    Command command{sizeof(Command),nullptr,kMaxMessageBytes+1};
    Require(Validate(&command)==Result::invalid_argument);
    std::uint8_t storage[8]{};
    EventBuffer event{sizeof(EventBuffer),storage,sizeof(storage),sizeof(storage)+1};
    Require(Validate(&event)==Result::invalid_argument);
}
