#include "poem/perf.h"

#include <algorithm>
#include <cmath>

namespace poem::perf {

namespace {

// Nearest-rank percentile over an ascending vector, matching
// percentileSorted in pkg/render/perf.go so the two sides agree on ranks.
double PercentileSorted(const std::vector<double>& sorted, double percentile) {
    if (sorted.empty()) return 0.0;
    auto index = static_cast<std::ptrdiff_t>(
        percentile / 100.0 * static_cast<double>(sorted.size() - 1) + 0.5);
    index = std::clamp<std::ptrdiff_t>(index, 0, static_cast<std::ptrdiff_t>(sorted.size()) - 1);
    return sorted[static_cast<std::size_t>(index)];
}

} // namespace

void Recorder::Record(const std::string& channel, double milliseconds) {
    if (!std::isfinite(milliseconds) || milliseconds < 0) return;
    std::lock_guard<std::mutex> lock(mutex_);
    auto& series = samples_[channel];
    series.push_back(milliseconds);
    if (series.size() > kMaxSamples) {
        series.erase(series.begin(), series.begin() + static_cast<std::ptrdiff_t>(series.size() - kMaxSamples));
    }
}

std::vector<PhaseStats> Recorder::Snapshot() const {
    std::lock_guard<std::mutex> lock(mutex_);
    std::vector<PhaseStats> out;
    out.reserve(samples_.size());
    for (const auto& [channel, series] : samples_) {
        if (series.empty()) continue;
        std::vector<double> sorted(series);
        std::sort(sorted.begin(), sorted.end());
        double sum = 0.0;
        for (const double value : sorted) sum += value;

        PhaseStats stats;
        stats.name = channel;
        stats.count = static_cast<std::uint32_t>(sorted.size());
        stats.meanMS = sum / static_cast<double>(sorted.size());
        stats.p50MS = PercentileSorted(sorted, 50);
        stats.p95MS = PercentileSorted(sorted, 95);
        stats.p99MS = PercentileSorted(sorted, 99);
        stats.maxMS = sorted.back();
        out.push_back(std::move(stats));
    }
    std::sort(out.begin(), out.end(), [](const PhaseStats& a, const PhaseStats& b) { return a.name < b.name; });
    return out;
}

void Recorder::Reset() {
    std::lock_guard<std::mutex> lock(mutex_);
    samples_.clear();
}

Recorder& Global() {
    static Recorder recorder;
    return recorder;
}

} // namespace poem::perf
