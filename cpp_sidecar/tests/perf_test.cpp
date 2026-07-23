// Contract tests for the native timing recorder. These run with no GPU, window
// or platform SDK: the recorder is pure computation over a sample ring, and its
// percentile ranks must agree with the Go tracker's percentileSorted so a
// native p95 and a Go p95 mean the same thing on either side of the boundary.

#include "poem/perf.h"

#include <cstdio>
#include <stdexcept>
#include <string>
#include <vector>

namespace {

void Require(bool condition, const std::string& what) {
    if (!condition) throw std::runtime_error("perf assertion failed: " + what);
}

bool Near(double value, double expected, double tolerance) {
    const double delta = value > expected ? value - expected : expected - value;
    return delta <= tolerance;
}

poem::perf::PhaseStats Find(const std::vector<poem::perf::PhaseStats>& stats, const std::string& name) {
    for (const auto& entry : stats) {
        if (entry.name == name) return entry;
    }
    throw std::runtime_error("no phase named " + name);
}

// 1..100 makes every nearest-rank percentile checkable by inspection.
void RanksMatchNearestRank() {
    poem::perf::Recorder recorder;
    for (int i = 1; i <= 100; ++i) {
        recorder.Record("frame", static_cast<double>(i));
    }
    const auto stats = Find(recorder.Snapshot(), "frame");
    Require(stats.count == 100, "count");
    Require(Near(stats.meanMS, 50.5, 1e-9), "mean");
    // Nearest-rank over 0-based indices: round(p/100 * (n-1)).
    Require(Near(stats.p50MS, 51.0, 1e-9), "p50");
    Require(Near(stats.p95MS, 95.0, 1e-9), "p95");
    Require(Near(stats.p99MS, 99.0, 1e-9), "p99");
    Require(Near(stats.maxMS, 100.0, 1e-9), "max");
}

// Percentiles must not be skewed by insertion order.
void OrderIndependent() {
    poem::perf::Recorder ascending;
    poem::perf::Recorder descending;
    for (int i = 1; i <= 100; ++i) ascending.Record("frame", static_cast<double>(i));
    for (int i = 100; i >= 1; --i) descending.Record("frame", static_cast<double>(i));
    const auto a = Find(ascending.Snapshot(), "frame");
    const auto d = Find(descending.Snapshot(), "frame");
    Require(Near(a.p95MS, d.p95MS, 1e-9), "p95 order independence");
    Require(Near(a.p50MS, d.p50MS, 1e-9), "p50 order independence");
}

// The ring must retain the most recent window, not the first samples seen.
void RingRetainsNewest() {
    poem::perf::Recorder recorder;
    const std::size_t overflow = poem::perf::kMaxSamples + 500;
    for (std::size_t i = 0; i < overflow; ++i) {
        // Everything before the final window is 1 ms; the window itself is 9 ms.
        recorder.Record("frame", i < 500 ? 1.0 : 9.0);
    }
    const auto stats = Find(recorder.Snapshot(), "frame");
    Require(stats.count == poem::perf::kMaxSamples, "ring is bounded");
    Require(Near(stats.meanMS, 9.0, 1e-9), "oldest samples evicted");
}

void ChannelsAreIndependentAndSorted() {
    poem::perf::Recorder recorder;
    recorder.Record("present", 4.0);
    recorder.Record("decode_frame", 1.0);
    recorder.Record("apply_map_scene", 2.0);
    const auto stats = recorder.Snapshot();
    Require(stats.size() == 3, "three channels");
    Require(stats[0].name == "apply_map_scene", "sorted by name");
    Require(stats[1].name == "decode_frame", "sorted by name");
    Require(stats[2].name == "present", "sorted by name");
    Require(Near(Find(stats, "present").p95MS, 4.0, 1e-9), "channels do not mix");
}

// Negative and non-finite durations would poison a percentile; a clock that
// jumps backwards must not be able to report a negative p95.
void RejectsInvalidSamples() {
    poem::perf::Recorder recorder;
    recorder.Record("frame", 5.0);
    recorder.Record("frame", -1.0);
    recorder.Record("frame", std::nan(""));
    const auto stats = Find(recorder.Snapshot(), "frame");
    Require(stats.count == 1, "invalid samples rejected");
    Require(Near(stats.p95MS, 5.0, 1e-9), "p95 unaffected");
}

void ResetClears() {
    poem::perf::Recorder recorder;
    recorder.Record("frame", 5.0);
    recorder.Reset();
    Require(recorder.Snapshot().empty(), "reset clears every channel");
}

void EmptyRecorderReportsNothing() {
    poem::perf::Recorder recorder;
    Require(recorder.Snapshot().empty(), "no channels before any sample");
}

} // namespace

int main() {
    try {
        RanksMatchNearestRank();
        OrderIndependent();
        RingRetainsNewest();
        ChannelsAreIndependentAndSorted();
        RejectsInvalidSamples();
        ResetClears();
        EmptyRecorderReportsNothing();
    } catch (const std::exception& error) {
        std::printf("FAIL: %s\n", error.what());
        return 1;
    }
    std::printf("native perf recorder contract: OK\n");
    return 0;
}
