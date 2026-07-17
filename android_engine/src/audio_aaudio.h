#pragma once

// AAudio playback for the engine's PlaySound protocol messages — the Android
// sibling of cpp_sidecar's AudioEngine (winmm). The three UI sounds are
// synthesized once at construction; Play fires one short-lived output stream
// per sound so overlapping feedback mixes naturally. Rate limiting lives in
// the engine (debounce timestamps in ApplicationState), not here.

#include <cstdint>
#include <vector>

#include "protocol.h"

namespace poem {

class AudioEngineAAudio {
  public:
    AudioEngineAAudio();
    // Play is fire-and-forget and safe from the transport thread.
    void Play(protocol::SoundType type);

  private:
    std::vector<std::int16_t> hover_;
    std::vector<std::int16_t> click_;
    std::vector<std::int16_t> success_;
};

} // namespace poem
