#include "renderer_gles.h"

#include <android/log.h>
#include <algorithm>
#include <cstring>
#include <string>

#define RLOGE(...) __android_log_print(ANDROID_LOG_ERROR, "poem-gles", __VA_ARGS__)

namespace poem {

namespace {

// UTF-8 decode, mirrored from cpp_sidecar's presenter so both interpret
// DrawCommand.text identically.
std::vector<std::uint32_t> DecodeUtf8(const std::string& text) {
    std::vector<std::uint32_t> codepoints;
    codepoints.reserve(text.size());
    for (std::size_t index = 0; index < text.size();) {
        const auto first = static_cast<std::uint8_t>(text[index]);
        std::uint32_t codepoint = 0;
        std::size_t length = 0;
        if (first < 0x80) {
            codepoint = first;
            length = 1;
        } else if ((first & 0xE0) == 0xC0) {
            codepoint = first & 0x1F;
            length = 2;
        } else if ((first & 0xF0) == 0xE0) {
            codepoint = first & 0x0F;
            length = 3;
        } else if ((first & 0xF8) == 0xF0) {
            codepoint = first & 0x07;
            length = 4;
        } else {
            ++index;
            continue;
        }
        if (index + length > text.size()) break;
        bool valid = true;
        for (std::size_t i = 1; i < length; ++i) {
            const auto follow = static_cast<std::uint8_t>(text[index + i]);
            if ((follow & 0xC0) != 0x80) {
                valid = false;
                break;
            }
            codepoint = (codepoint << 6) | (follow & 0x3F);
        }
        if (valid) codepoints.push_back(codepoint);
        index += length;
    }
    return codepoints;
}

const char* kVertexShader = R"(
attribute vec2 aPos;
attribute vec2 aUV;
attribute vec4 aColor;
attribute vec4 aRect;
attribute vec4 aMisc; // drawType, glow, isGlass, radius
uniform vec2 uScreen;
uniform vec2 uInset;
varying vec2 vUV;
varying vec4 vColor;
varying vec4 vRect;
varying vec4 vMisc;
varying vec2 vPixel;
void main() {
    vUV = aUV;
    vColor = aColor;
    vRect = aRect;
    vMisc = aMisc;
    vPixel = aPos;
    vec2 p = aPos + uInset;
    vec2 ndc = vec2(p.x / uScreen.x * 2.0 - 1.0, 1.0 - p.y / uScreen.y * 2.0);
    gl_Position = vec4(ndc, 0.0, 1.0);
}
)";

// drawType: 0 = SDF shape, 1 = text glyph. The rounded-box SDF matches the
// sibling presenters: distance from the rect (shrunk by radius), smoothed
// over ~1px for antialiasing; glow widens the falloff.
const char* kFragmentShader = R"(
precision mediump float;
uniform sampler2D uAtlas;
varying vec2 vUV;
varying vec4 vColor;
varying vec4 vRect;
varying vec4 vMisc;
varying vec2 vPixel;
void main() {
    if (vMisc.x > 1.5) {
        gl_FragColor = texture2D(uAtlas, vUV) * vColor;
        return;
    }
    if (vMisc.x > 0.5) {
        vec4 sample = texture2D(uAtlas, vUV);
        gl_FragColor = vec4(vColor.rgb, vColor.a * sample.a);
        return;
    }
    vec2 center = vRect.xy + vRect.zw * 0.5;
    vec2 halfSize = vRect.zw * 0.5;
    float radius = min(vMisc.w, min(halfSize.x, halfSize.y));
    vec2 p = abs(vPixel - center) - (halfSize - vec2(radius));
    float dist = length(max(p, 0.0)) + min(max(p.x, p.y), 0.0) - radius;
    float soft = max(vMisc.y, 1.0);
    float alpha = 1.0 - smoothstep(-soft, soft * 0.5, dist);
    gl_FragColor = vec4(vColor.rgb, vColor.a * alpha);
}
)";

GLuint Compile(GLenum type, const char* source) {
    GLuint shader = glCreateShader(type);
    glShaderSource(shader, 1, &source, nullptr);
    glCompileShader(shader);
    GLint ok = GL_FALSE;
    glGetShaderiv(shader, GL_COMPILE_STATUS, &ok);
    if (!ok) {
        char log[512];
        glGetShaderInfoLog(shader, sizeof(log), nullptr, log);
        RLOGE("shader compile failed: %s", log);
        glDeleteShader(shader);
        return 0;
    }
    return shader;
}

} // namespace

bool RendererGLES::Init(int width, int height, float scale) {
    width_ = width;
    height_ = height;
    scale_ = scale > 0.0f ? scale : 1.0f;

    GLuint vs = Compile(GL_VERTEX_SHADER, kVertexShader);
    GLuint fs = Compile(GL_FRAGMENT_SHADER, kFragmentShader);
    if (!vs || !fs) return false;
    program_ = glCreateProgram();
    glAttachShader(program_, vs);
    glAttachShader(program_, fs);
    glBindAttribLocation(program_, 0, "aPos");
    glBindAttribLocation(program_, 1, "aUV");
    glBindAttribLocation(program_, 2, "aColor");
    glBindAttribLocation(program_, 3, "aRect");
    glBindAttribLocation(program_, 4, "aMisc");
    glLinkProgram(program_);
    GLint linked = GL_FALSE;
    glGetProgramiv(program_, GL_LINK_STATUS, &linked);
    glDeleteShader(vs);
    glDeleteShader(fs);
    if (!linked) {
        char log[512];
        glGetProgramInfoLog(program_, sizeof(log), nullptr, log);
        RLOGE("program link failed: %s", log);
        return false;
    }
    uScreen_ = glGetUniformLocation(program_, "uScreen");
    uAtlas_ = glGetUniformLocation(program_, "uAtlas");
    uInset_ = glGetUniformLocation(program_, "uInset");

    glGenBuffers(1, &vbo_);
    glGenTextures(1, &atlasTexture_);
    glEnable(GL_BLEND);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
    return true;
}

void RendererGLES::Resize(int width, int height) {
    width_ = width;
    height_ = height;
}

void RendererGLES::SetInset(int x, int y) {
    insetX_ = x;
    insetY_ = y;
}

void RendererGLES::UploadAtlas(const protocol::InitEngine& init) {
    if (init.atlasWidth <= 0 || init.atlasHeight <= 0) return;
    const std::size_t expected =
        static_cast<std::size_t>(init.atlasWidth) * static_cast<std::size_t>(init.atlasHeight) * 4;
    if (init.atlasPixels.size() < expected) {
        RLOGE("atlas pixel buffer too small: %zu < %zu", init.atlasPixels.size(), expected);
        return;
    }
    glBindTexture(GL_TEXTURE_2D, atlasTexture_);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, init.atlasWidth, init.atlasHeight, 0, GL_RGBA,
                 GL_UNSIGNED_BYTE, init.atlasPixels.data());
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);

    glyphs_.clear();
    for (const auto& ch : init.chars) {
        glyphs_[static_cast<std::uint32_t>(ch.r)] =
            GlyphInfo{ch.u1, ch.v1, ch.u2, ch.v2, ch.width, ch.height, ch.advance};
    }
}

void RendererGLES::AppendQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                              float u1, float v1, float u2, float v2,
                              float r, float g, float b, float a,
                              float drawType, float glow, float isGlass, float radius) {
    const Vertex tl{x1, y1, u1, v1, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    const Vertex tr{x2, y1, u2, v1, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    const Vertex bl{x1, y2, u1, v2, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    const Vertex br{x2, y2, u2, v2, r, g, b, a, x1, y1, x2 - x1, y2 - y1, drawType, glow, isGlass, radius};
    vertices.push_back(tl);
    vertices.push_back(tr);
    vertices.push_back(bl);
    vertices.push_back(bl);
    vertices.push_back(tr);
    vertices.push_back(br);
}

void RendererGLES::BuildGeometry(const protocol::RenderFrame& frame,
                                 std::vector<Vertex>& vertices, std::vector<DrawRange>& ranges) {
    float currentGlow = 0.0f;
    float currentGlass = 0.0f;
    float shadowOx = 0.0f, shadowOy = 0.0f, shadowBlur = 0.0f;
    float offsetX = 0.0f, offsetY = 0.0f;
    bool clipEnabled = false;
    int clipX = 0, clipY = 0, clipW = 0, clipH = 0;

    unsigned int rangeImage = 0; // texture for the quad being pushed (0 = atlas)
    auto pushRange = [&](std::uint32_t start, std::uint32_t count) {
        if (!ranges.empty()) {
            auto& last = ranges.back();
            if (last.start + last.count == start && last.clipEnabled == clipEnabled &&
                last.imageTexture == rangeImage &&
                last.clipX == clipX && last.clipY == clipY && last.clipW == clipW && last.clipH == clipH) {
                last.count += count;
                return;
            }
        }
        ranges.push_back(DrawRange{start, count, clipEnabled, clipX, clipY, clipW, clipH, rangeImage});
    };

    for (const auto& cmd : frame.commands) {
        switch (cmd.type) {
        case protocol::DrawCommandType::SetGlow:
            currentGlow = cmd.val1 * scale_;
            continue;
        case protocol::DrawCommandType::SetGlass:
            currentGlass = cmd.flag ? 1.0f : 0.0f;
            continue;
        case protocol::DrawCommandType::SetShadow:
            shadowOx = cmd.val1 * scale_;
            shadowOy = cmd.val2 * scale_;
            shadowBlur = cmd.val3 * scale_;
            continue;
        case protocol::DrawCommandType::SetOffset:
            offsetX = cmd.val1 * scale_;
            offsetY = cmd.val2 * scale_;
            continue;
        case protocol::DrawCommandType::SetClip:
            clipEnabled = cmd.flag;
            if (clipEnabled) {
                clipX = static_cast<int>(cmd.x1 * scale_);
                clipY = static_cast<int>(cmd.y1 * scale_);
                clipW = static_cast<int>(cmd.w * scale_);
                clipH = static_cast<int>(cmd.h * scale_);
            }
            continue;
        default:
            break;
        }

        const auto start = static_cast<std::uint32_t>(vertices.size());
        const float cr = cmd.r / 255.0f;
        const float cg = cmd.g / 255.0f;
        const float cb = cmd.b / 255.0f;
        const float ca = cmd.a / 255.0f;
        const float x1 = static_cast<float>(cmd.x1) * scale_ + offsetX;
        const float y1 = static_cast<float>(cmd.y1) * scale_ + offsetY;
        const float x2 = static_cast<float>(cmd.x2) * scale_ + offsetX;
        const float y2 = static_cast<float>(cmd.y2) * scale_ + offsetY;

        // Companion quads first so the element draws over its own effects,
        // mirroring the D3D11 presenter's ordering.
        if ((shadowOx != 0.0f || shadowOy != 0.0f) &&
            cmd.type != protocol::DrawCommandType::DrawText && cmd.radius >= 0) {
            AppendQuad(vertices, x1 + shadowOx, y1 + shadowOy, x2 + shadowOx, y2 + shadowOy,
                       0, 0, 1, 1, 0.0f, 0.0f, 0.0f, ca * 0.25f,
                       0.0f, std::max(1.0f, shadowBlur), 0.0f, static_cast<float>(cmd.radius) * scale_);
        }
        if (currentGlow > 0.0f && cmd.type != protocol::DrawCommandType::DrawText && cmd.radius >= 0) {
            AppendQuad(vertices, x1 - currentGlow, y1 - currentGlow, x2 + currentGlow, y2 + currentGlow,
                       0, 0, 1, 1, cr, cg, cb, ca * 0.15f,
                       0.0f, currentGlow, 0.0f, static_cast<float>(cmd.radius) * scale_ + currentGlow);
        }

        switch (cmd.type) {
        case protocol::DrawCommandType::DrawRoundedRect:
        case protocol::DrawCommandType::FillRect: {
            // Negative radii select raycaster/billboard demo modes on desktop;
            // this presenter renders them as plain rects.
            const float radius = (cmd.type == protocol::DrawCommandType::FillRect)
                                     ? 0.0f
                                     : static_cast<float>(std::max(0, cmd.radius)) * scale_;
            AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, cr, cg, cb, ca,
                       0.0f, currentGlow, currentGlass, radius);
            break;
        }
        case protocol::DrawCommandType::DrawLine: {
            const float thickness = 1.5f * scale_;
            const float minX = std::min(x1, x2) - thickness * 0.5f;
            const float maxX = std::max(x1, x2) + thickness * 0.5f;
            const float minY = std::min(y1, y2) - thickness * 0.5f;
            const float maxY = std::max(y1, y2) + thickness * 0.5f;
            AppendQuad(vertices, minX, minY, maxX, maxY, 0, 0, 1, 1, cr, cg, cb, ca, 0.0f, 0.0f, 0.0f, 0.0f);
            break;
        }
        case protocol::DrawCommandType::DrawText: {
            float penX = x1;
            const float baselineY = y1;
            const float textScale = cmd.val1 > 0.0f ? cmd.val1 : 1.0f;
            for (const auto codepoint : DecodeUtf8(cmd.text)) {
                auto it = glyphs_.find(codepoint);
                if (it == glyphs_.end()) {
                    it = glyphs_.find(static_cast<std::uint32_t>('?'));
                    if (it == glyphs_.end()) {
                        penX += 8.0f * textScale * scale_;
                        continue;
                    }
                }
                const auto& glyph = it->second;
                const float glyphWidth = glyph.width * textScale * scale_;
                const float glyphHeight = glyph.height * textScale * scale_;
                const float gy1 = baselineY - glyphHeight * 0.78f;
                AppendQuad(vertices, penX, gy1, penX + glyphWidth, gy1 + glyphHeight,
                           glyph.u1, glyph.v1, glyph.u2, glyph.v2, cr, cg, cb, ca,
                           1.0f, 0.0f, 0.0f, 0.0f);
                penX += glyph.advance * textScale * scale_;
            }
            break;
        }
        case protocol::DrawCommandType::DrawImage: {
            // cmd.w/h carry the pixel dimensions, cmd.bytes the RGBA data.
            const auto texture = UploadImageCached(cmd.bytes, cmd.w, cmd.h);
            if (texture == 0) break;
            rangeImage = texture;
            // drawType 2: sample the bound image texture across the quad.
            AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, 1, 1, 1, 1, 2.0f, 0.0f, 0.0f, 0.0f);
            break;
        }
        default:
            break;
        }

        const auto end = static_cast<std::uint32_t>(vertices.size());
        if (end > start) pushRange(start, end - start);
        rangeImage = 0;
    }
}

void RendererGLES::Render(const protocol::RenderFrame& frame) {
    std::vector<Vertex> vertices;
    std::vector<DrawRange> ranges;
    BuildGeometry(frame, vertices, ranges);

    glViewport(0, 0, width_, height_);
    glDisable(GL_SCISSOR_TEST);
    glClearColor(0.04f, 0.05f, 0.07f, 1.0f);
    glClear(GL_COLOR_BUFFER_BIT);
    if (vertices.empty()) return;

    glUseProgram(program_);
    glUniform2f(uScreen_, static_cast<float>(width_), static_cast<float>(height_));
    glActiveTexture(GL_TEXTURE0);
    glBindTexture(GL_TEXTURE_2D, atlasTexture_);
    glUniform1i(uAtlas_, 0);
    glUniform2f(uInset_, static_cast<float>(insetX_), static_cast<float>(insetY_));

    glBindBuffer(GL_ARRAY_BUFFER, vbo_);
    glBufferData(GL_ARRAY_BUFFER, static_cast<GLsizeiptr>(vertices.size() * sizeof(Vertex)),
                 vertices.data(), GL_STREAM_DRAW);
    const auto stride = static_cast<GLsizei>(sizeof(Vertex));
    glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(0));
    glVertexAttribPointer(1, 2, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(8));
    glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(16));
    glVertexAttribPointer(3, 4, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(32));
    glVertexAttribPointer(4, 4, GL_FLOAT, GL_FALSE, stride, reinterpret_cast<void*>(48));
    for (int i = 0; i <= 4; ++i) glEnableVertexAttribArray(i);

    for (const auto& range : ranges) {
        glBindTexture(GL_TEXTURE_2D, range.imageTexture != 0 ? range.imageTexture : atlasTexture_);
        if (range.clipEnabled) {
            glEnable(GL_SCISSOR_TEST);
            // Protocol clip rects are top-left origin; GL scissor is bottom-left.
            glScissor(range.clipX + insetX_, height_ - (range.clipY + insetY_ + range.clipH), range.clipW, range.clipH);
        } else {
            glDisable(GL_SCISSOR_TEST);
        }
        glDrawArrays(GL_TRIANGLES, static_cast<GLint>(range.start), static_cast<GLsizei>(range.count));
    }
    glDisable(GL_SCISSOR_TEST);
}

} // namespace poem

namespace poem {

// UploadImageCached returns a texture for the RGBA payload, uploading only on
// first sight of this exact content (FNV-1a over the bytes). The engine
// resends image bytes every frame; the cache turns that into one upload per
// distinct image. Small hard cap with drop-all eviction — UI image working
// sets are tiny, and bookkeeping an LRU would outweigh re-uploading once.
unsigned int RendererGLES::UploadImageCached(const std::vector<std::uint8_t>& rgba, int width, int height) {
    if (width <= 0 || height <= 0 ||
        rgba.size() < static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4) {
        return 0;
    }
    std::uint64_t hash = 1469598103934665603ull;
    for (const auto byte : rgba) {
        hash ^= byte;
        hash *= 1099511628211ull;
    }
    if (auto it = images_.find(hash); it != images_.end()) return it->second.texture;

    if (images_.size() >= 8) {
        for (auto& [key, entry] : images_) glDeleteTextures(1, &entry.texture);
        images_.clear();
    }
    GLuint texture = 0;
    glGenTextures(1, &texture);
    glBindTexture(GL_TEXTURE_2D, texture);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, width, height, 0, GL_RGBA, GL_UNSIGNED_BYTE, rgba.data());
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
    images_[hash] = ImageEntry{texture, width, height};
    return texture;
}

} // namespace poem
