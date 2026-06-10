#include "audio.h"

#include <cmath>
#include <cstdint>
#include <vector>
#include <windows.h>
#include <mmsystem.h>

namespace poem {

namespace {

void AppendWord(std::vector<std::uint8_t>& out, std::uint16_t v) {
    out.push_back(static_cast<std::uint8_t>(v & 0xFF));
    out.push_back(static_cast<std::uint8_t>((v >> 8) & 0xFF));
}

void AppendDWord(std::vector<std::uint8_t>& out, std::uint32_t v) {
    for (int i = 0; i < 4; ++i) {
        out.push_back(static_cast<std::uint8_t>((v >> (8 * i)) & 0xFF));
    }
}

std::vector<std::uint8_t> MakeWave(const std::vector<float>& samples, int sampleRate = 44100) {
    std::vector<std::uint8_t> out;
    const std::uint16_t channels = 1;
    const std::uint16_t bitsPerSample = 16;
    const std::uint32_t byteRate = sampleRate * channels * bitsPerSample / 8;
    const std::uint16_t blockAlign = channels * bitsPerSample / 8;
    const std::uint32_t dataSize = static_cast<std::uint32_t>(samples.size() * 2);

    out.insert(out.end(), {'R', 'I', 'F', 'F'});
    AppendDWord(out, 36 + dataSize);
    out.insert(out.end(), {'W', 'A', 'V', 'E'});
    out.insert(out.end(), {'f', 'm', 't', ' '});
    AppendDWord(out, 16);
    AppendWord(out, 1);
    AppendWord(out, channels);
    AppendDWord(out, sampleRate);
    AppendDWord(out, byteRate);
    AppendWord(out, blockAlign);
    AppendWord(out, bitsPerSample);
    out.insert(out.end(), {'d', 'a', 't', 'a'});
    AppendDWord(out, dataSize);

    for (float sample : samples) {
        const auto clamped = std::max(-1.0f, std::min(1.0f, sample));
        const auto pcm = static_cast<std::int16_t>(clamped * 32767.0f);
        AppendWord(out, static_cast<std::uint16_t>(pcm));
    }
    return out;
}

std::vector<float> HoverSamples() {
    const int sampleRate = 44100;
    const float duration = 0.015f;
    const int count = static_cast<int>(sampleRate * duration);
    std::vector<float> out;
    out.reserve(count);
    for (int i = 0; i < count; ++i) {
        float t = static_cast<float>(i) / static_cast<float>(sampleRate);
        float env = std::exp(-t * 220.0f);
        float val = std::sin(2.0f * 3.14159265f * 1200.0f * t);
        out.push_back(val * 0.15f * env);
    }
    return out;
}

std::vector<float> ClickSamples() {
    const int sampleRate = 44100;
    const float duration = 0.12f;
    const int count = static_cast<int>(sampleRate * duration);
    std::vector<float> out;
    out.reserve(count);
    for (int i = 0; i < count; ++i) {
        float t = static_cast<float>(i) / static_cast<float>(sampleRate);
        float env = std::exp(-t * 28.0f);
        float v1 = std::sin(2.0f * 3.14159265f * 900.0f * t);
        float v2 = std::sin(2.0f * 3.14159265f * 1800.0f * t) * 0.4f;
        out.push_back(((v1 + v2) / 1.4f) * 0.35f * env);
    }
    return out;
}

std::vector<float> SuccessSamples() {
    const int sampleRate = 44100;
    const float freqs[] = {659.25f, 880.0f, 1109.73f, 1318.51f};
    const float noteDuration = 0.10f;
    const int count = static_cast<int>(sampleRate * noteDuration * 4.0f);
    std::vector<float> out;
    out.reserve(count);
    for (int i = 0; i < count; ++i) {
        float t = static_cast<float>(i) / static_cast<float>(sampleRate);
        int idx = static_cast<int>(t / noteDuration);
        if (idx > 3) idx = 3;
        float localT = t - idx * noteDuration;
        float env = std::exp(-localT * 18.0f);
        float val = std::sin(2.0f * 3.14159265f * freqs[idx] * t);
        out.push_back(val * 0.3f * env);
    }
    return out;
}

} // namespace

AudioEngine::AudioEngine() {
    hover_ = MakeWave(HoverSamples());
    click_ = MakeWave(ClickSamples());
    success_ = MakeWave(SuccessSamples());
}

void AudioEngine::Play(protocol::SoundType type) {
    const std::vector<std::uint8_t>* bytes = nullptr;
    switch (type) {
    case protocol::SoundType::Hover: bytes = &hover_; break;
    case protocol::SoundType::Click: bytes = &click_; break;
    case protocol::SoundType::Success: bytes = &success_; break;
    }
    if (!bytes || bytes->empty()) {
        return;
    }
    ::PlaySoundW(reinterpret_cast<LPCWSTR>(bytes->data()), nullptr, SND_MEMORY | SND_ASYNC | SND_NODEFAULT);
}

} // namespace poem
