#include "audio_aaudio.h"

#include <aaudio/AAudio.h>
#include <android/log.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cmath>
#include <thread>

#define ALOGE(...) __android_log_print(ANDROID_LOG_ERROR, "poem-audio", __VA_ARGS__)

namespace poem {

namespace {

constexpr int kSampleRate = 44100;

// Sample generators mirrored from cpp_sidecar/src/audio.cpp so every
// presenter produces the same acoustic feedback (that file stays untouched:
// its sources are pinned by the sidecar provenance manifest). Windows wraps
// these in a WAV container for winmm; AAudio takes the raw PCM directly.

std::vector<std::int16_t> ToPcm(const std::vector<float>& samples) {
    std::vector<std::int16_t> out;
    out.reserve(samples.size());
    for (float sample : samples) {
        const float clamped = std::max(-1.0f, std::min(1.0f, sample));
        out.push_back(static_cast<std::int16_t>(clamped * 32767.0f));
    }
    return out;
}

std::vector<float> HoverSamples() {
    const float duration = 0.015f;
    const int count = static_cast<int>(kSampleRate * duration);
    std::vector<float> out;
    out.reserve(count);
    for (int i = 0; i < count; ++i) {
        float t = static_cast<float>(i) / static_cast<float>(kSampleRate);
        float env = std::exp(-t * 220.0f);
        float val = std::sin(2.0f * 3.14159265f * 1200.0f * t);
        out.push_back(val * 0.15f * env);
    }
    return out;
}

std::vector<float> ClickSamples() {
    const float duration = 0.12f;
    const int count = static_cast<int>(kSampleRate * duration);
    std::vector<float> out;
    out.reserve(count);
    for (int i = 0; i < count; ++i) {
        float t = static_cast<float>(i) / static_cast<float>(kSampleRate);
        float env = std::exp(-t * 28.0f);
        float v1 = std::sin(2.0f * 3.14159265f * 900.0f * t);
        float v2 = std::sin(2.0f * 3.14159265f * 1800.0f * t) * 0.4f;
        out.push_back(((v1 + v2) / 1.4f) * 0.35f * env);
    }
    return out;
}

std::vector<float> SuccessSamples() {
    const float freqs[] = {659.25f, 880.0f, 1109.73f, 1318.51f};
    const float noteDuration = 0.10f;
    const int count = static_cast<int>(kSampleRate * noteDuration * 4.0f);
    std::vector<float> out;
    out.reserve(count);
    for (int i = 0; i < count; ++i) {
        float t = static_cast<float>(i) / static_cast<float>(kSampleRate);
        int idx = static_cast<int>(t / noteDuration);
        if (idx > 3) idx = 3;
        float localT = t - idx * noteDuration;
        float env = std::exp(-localT * 18.0f);
        float val = std::sin(2.0f * 3.14159265f * freqs[idx] * t);
        out.push_back(val * 0.3f * env);
    }
    return out;
}

// activePlays caps concurrent one-shot streams; the engine debounces, but a
// burst (hover storm during a fast drag) should degrade by dropping sounds,
// not by piling up stream threads.
std::atomic<int> activePlays{0};

void PlayBuffer(const std::vector<std::int16_t>* pcm) {
    if (activePlays.fetch_add(1) >= 4) {
        activePlays.fetch_sub(1);
        return;
    }
    std::thread([pcm] {
        AAudioStreamBuilder* builder = nullptr;
        if (AAudio_createStreamBuilder(&builder) != AAUDIO_OK) {
            activePlays.fetch_sub(1);
            return;
        }
        AAudioStreamBuilder_setFormat(builder, AAUDIO_FORMAT_PCM_I16);
        AAudioStreamBuilder_setChannelCount(builder, 1);
        AAudioStreamBuilder_setSampleRate(builder, kSampleRate);
        // (setUsage would tag these as sonification, but it needs API 28 and
        // the presenter targets minSdk 26; the default usage is fine.)
        AAudioStream* stream = nullptr;
        const auto openResult = AAudioStreamBuilder_openStream(builder, &stream);
        AAudioStreamBuilder_delete(builder);
        if (openResult != AAUDIO_OK || !stream) {
            ALOGE("AAudio open failed: %s", AAudio_convertResultToText(openResult));
            activePlays.fetch_sub(1);
            return;
        }
        if (AAudioStream_requestStart(stream) == AAUDIO_OK) {
            // Blocking write of the whole clip, then let the tail drain.
            const auto frames = static_cast<int32_t>(pcm->size());
            AAudioStream_write(stream, pcm->data(), frames, 500L * 1000 * 1000);
            const auto burst = AAudioStream_getFramesPerBurst(stream);
            const std::int64_t tailNanos =
                (burst > 0 ? (std::int64_t)burst : 512) * 4 * 1000000000LL / kSampleRate;
            std::this_thread::sleep_for(std::chrono::nanoseconds(tailNanos));
            AAudioStream_requestStop(stream);
        }
        AAudioStream_close(stream);
        activePlays.fetch_sub(1);
    }).detach();
}

} // namespace

AudioEngineAAudio::AudioEngineAAudio() {
    hover_ = ToPcm(HoverSamples());
    click_ = ToPcm(ClickSamples());
    success_ = ToPcm(SuccessSamples());
}

void AudioEngineAAudio::Play(protocol::SoundType type) {
    switch (type) {
    case protocol::SoundType::Hover: PlayBuffer(&hover_); break;
    case protocol::SoundType::Click: PlayBuffer(&click_); break;
    case protocol::SoundType::Success: PlayBuffer(&success_); break;
    }
}

} // namespace poem
