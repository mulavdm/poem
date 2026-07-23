#pragma once

// Native-side frame timing, shared by every host.
//
// Until this existed nothing in the C++ hosts timed frame CPU work at all —
// the only clocks present were for gestures, fling, and audio — so the
// migration plan's "native framework CPU work below 4.17 ms at p95" budget had
// no measurement behind it. Samples are retained raw and reduced to
// percentiles on demand, because a mean cannot answer a percentile gate.
//
// This lives in shared/poem so the D3D11 and GLES presenters report the same
// channels under the same percentile definition. A p95 that meant one thing on
// Windows and another on Android would make the comparison worthless, which is
// precisely the divergence the shared runtime exists to prevent.

#include <chrono>
#include <cstdint>
#include <mutex>
#include <string>
#include <unordered_map>
#include <vector>

namespace poem::perf {

// Matches maxPerfSamples in pkg/render/perf.go so both sides of the boundary
// summarize the same size window.
constexpr std::size_t kMaxSamples = 4096;

struct PhaseStats {
    std::string name;
    std::uint32_t count = 0;
    double meanMS = 0;
    double p50MS = 0;
    double p95MS = 0;
    double p99MS = 0;
    double maxMS = 0;
};

class Recorder {
  public:
    void Record(const std::string& channel, double milliseconds);
    // Snapshot returns one entry per channel, ordered by channel name so the
    // wire encoding is stable across calls.
    std::vector<PhaseStats> Snapshot() const;
    void Reset();

  private:
    mutable std::mutex mutex_;
    std::unordered_map<std::string, std::vector<double>> samples_;
};

Recorder& Global();

// ScopedTimer records the lifetime of the scope it is declared in.
class ScopedTimer {
  public:
    explicit ScopedTimer(std::string channel)
        : channel_(std::move(channel)), start_(std::chrono::steady_clock::now()) {}
    ~ScopedTimer() {
        const auto elapsed = std::chrono::steady_clock::now() - start_;
        Global().Record(channel_, std::chrono::duration<double, std::milli>(elapsed).count());
    }
    ScopedTimer(const ScopedTimer&) = delete;
    ScopedTimer& operator=(const ScopedTimer&) = delete;

  private:
    std::string channel_;
    std::chrono::steady_clock::time_point start_;
};

} // namespace poem::perf
