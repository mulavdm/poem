#pragma once

#include "protocol.h"

#include <vector>

namespace poem {

class AudioEngine {
  public:
    AudioEngine();
    void Play(protocol::SoundType type);

  private:
    std::vector<std::uint8_t> hover_;
    std::vector<std::uint8_t> click_;
    std::vector<std::uint8_t> success_;
};

} // namespace poem
