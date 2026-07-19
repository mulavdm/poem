#include "native_app.h"

#include <functional>
#include <stdexcept>
#include <string>

namespace {

void Require(bool condition) {
    if (!condition) throw std::runtime_error("contract assertion failed");
}

void Reject(const std::string& value) {
    try {
        (void)poem::host::ParseMetadata(value);
    } catch (const std::exception&) {
        return;
    }
    throw std::runtime_error("malformed metadata was accepted");
}

} // namespace

int main() {
    const auto metadata = poem::host::ParseMetadata(
        R"({"identity":"POEM.Counter","title":"Counter","width":480,"height":320})");
    Require(metadata.identity == "POEM.Counter");
    Require(metadata.title == L"Counter");
    Require(metadata.width == 480 && metadata.height == 320);

    Reject("");
    Reject(R"({"identity":"bad/path","title":"Counter","width":480,"height":320})");
    Reject(R"({"identity":"POEM.Counter","title":"","width":480,"height":320})");
    Reject(R"({"identity":"POEM.Counter","title":"Counter","width":0,"height":320})");
    Reject(R"({"identity":"POEM.Counter","title":"Counter","width":480junk,"height":320})");
    Reject(R"({"identity":"POEM.Counter","title":"Counter","width":480})");
    return 0;
}
