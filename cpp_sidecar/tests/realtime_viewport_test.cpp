#include "poem/realtime_viewport.h"
#include <stdexcept>
using namespace poem::realtime;
static Result Init(void*, const FrameInput*) { return Result::ok; }
static Result Render(void*, const FrameInput*) { return Result::ok; }
static void Resize(void*, Rect) {}
static void Event(void*) {}
static Result Semantics(void*, SemanticSnapshot*) { return Result::ok; }
static void Require(bool value) { if (!value) throw std::runtime_error("viewport contract"); }
int main() {
    Exports value{sizeof(Exports),kABIVersion,nullptr,Init,Render,Resize,Event,Event,Semantics};
    Require(Validate(&value)==Result::ok);
    value.abiVersion++; Require(Validate(&value)==Result::unsupported_version);
    value.abiVersion=kABIVersion; value.render=nullptr;
    Require(Validate(&value)==Result::invalid_argument);
    SemanticSnapshot snapshot{sizeof(SemanticSnapshot),nullptr,kMaxSemanticBytes+1};
    Require(ValidateSnapshot(&snapshot)==Result::invalid_argument);
}

