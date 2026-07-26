#include "poem/protocol.h"

#include <cmath>
#include <cstring>
#include <stdexcept>

namespace poem::protocol {

namespace {

class Reader {
  public:
    explicit Reader(const std::vector<std::uint8_t>& data) : data_(data) {}

    template <typename T>
    T Read() {
        if (offset_ + sizeof(T) > data_.size()) {
            throw std::runtime_error("protocol read overflow");
        }
        T out{};
        std::memcpy(&out, data_.data() + offset_, sizeof(T));
        offset_ += sizeof(T);
        return out;
    }

    std::vector<std::uint8_t> ReadBytes() {
        auto size = Read<std::uint32_t>();
        constexpr std::uint32_t kMaxByteVector = 256u * 1024u * 1024u;
        if (size > kMaxByteVector) {
            throw std::runtime_error("protocol byte vector exceeds limit");
        }
        if (offset_ + size > data_.size()) {
            throw std::runtime_error("protocol byte vector overflow");
        }
        std::vector<std::uint8_t> out(data_.begin() + static_cast<std::ptrdiff_t>(offset_),
                                      data_.begin() + static_cast<std::ptrdiff_t>(offset_ + size));
        offset_ += size;
        return out;
    }

    std::string ReadString() {
        auto bytes = ReadBytes();
        return std::string(bytes.begin(), bytes.end());
    }

    template <std::size_t N>
    std::array<std::uint8_t, N> ReadArray() {
        if (offset_ + N > data_.size()) throw std::runtime_error("protocol array overflow");
        std::array<std::uint8_t, N> out{};
        std::memcpy(out.data(), data_.data() + offset_, N);
        offset_ += N;
        return out;
    }

    void RequireEnd() const {
        if (offset_ != data_.size()) throw std::runtime_error("protocol trailing data");
    }

  private:
    const std::vector<std::uint8_t>& data_;
    std::size_t offset_ = 0;
};

class Writer {
  public:
    template <typename T>
    void Write(T value) {
        const auto* ptr = reinterpret_cast<const std::uint8_t*>(&value);
        data_.insert(data_.end(), ptr, ptr + sizeof(T));
    }

    void WriteBytes(const std::vector<std::uint8_t>& bytes) {
        Write<std::uint32_t>(static_cast<std::uint32_t>(bytes.size()));
        data_.insert(data_.end(), bytes.begin(), bytes.end());
    }

    void WriteString(const std::string& text) {
        Write<std::uint32_t>(static_cast<std::uint32_t>(text.size()));
        data_.insert(data_.end(), text.begin(), text.end());
    }

    std::vector<std::uint8_t> Finish(MessageType type) {
        std::vector<std::uint8_t> out;
        out.insert(out.end(), std::begin(kMagic), std::end(kMagic));
        const auto version = kVersion;
        const auto msgType = static_cast<std::uint16_t>(type);
        const auto* versionPtr = reinterpret_cast<const std::uint8_t*>(&version);
        const auto* typePtr = reinterpret_cast<const std::uint8_t*>(&msgType);
        out.insert(out.end(), versionPtr, versionPtr + sizeof(version));
        out.insert(out.end(), typePtr, typePtr + sizeof(msgType));
        out.insert(out.end(), data_.begin(), data_.end());
        return out;
    }

  private:
    std::vector<std::uint8_t> data_;
};

} // namespace

Envelope DecodeEnvelope(const std::vector<std::uint8_t>& payload) {
    if (payload.size() < 8) {
        throw std::runtime_error("protocol payload too short");
    }
    if (std::memcmp(payload.data(), kMagic, 4) != 0) {
        throw std::runtime_error("protocol magic mismatch");
    }
    std::uint16_t version{};
    std::uint16_t msgType{};
    std::memcpy(&version, payload.data() + 4, sizeof(version));
    std::memcpy(&msgType, payload.data() + 6, sizeof(msgType));
    if (version != kVersion) {
        throw std::runtime_error("protocol version mismatch");
    }
    Envelope env;
    env.type = static_cast<MessageType>(msgType);
    env.body.assign(payload.begin() + 8, payload.end());
    return env;
}

InitEngine DecodeInitEngine(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    InitEngine out;
    out.width = r.Read<std::int32_t>();
    out.height = r.Read<std::int32_t>();
    out.atlasWidth = r.Read<std::int32_t>();
    out.atlasHeight = r.Read<std::int32_t>();
    out.atlasPixels = r.ReadBytes();
    auto count = r.Read<std::uint32_t>();
    out.chars.reserve(count);
    for (std::uint32_t i = 0; i < count; ++i) {
        CharInfo ch;
        ch.r = r.Read<std::int32_t>();
        ch.u1 = r.Read<float>();
        ch.v1 = r.Read<float>();
        ch.u2 = r.Read<float>();
        ch.v2 = r.Read<float>();
        ch.width = r.Read<std::int32_t>();
        ch.height = r.Read<std::int32_t>();
        ch.advance = r.Read<std::int32_t>();
        out.chars.push_back(ch);
    }
    return out;
}

RenderFrame DecodeRenderFrame(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    RenderFrame out;
    out.width = r.Read<std::int32_t>();
    out.height = r.Read<std::int32_t>();
    out.cursor = r.Read<std::uint8_t>();
    auto count = r.Read<std::uint32_t>();
    out.commands.reserve(count);
    for (std::uint32_t i = 0; i < count; ++i) {
        DrawCommand cmd;
        cmd.type = static_cast<DrawCommandType>(r.Read<std::uint8_t>());
        cmd.x1 = r.Read<std::int32_t>();
        cmd.y1 = r.Read<std::int32_t>();
        cmd.x2 = r.Read<std::int32_t>();
        cmd.y2 = r.Read<std::int32_t>();
        cmd.w = r.Read<std::int32_t>();
        cmd.h = r.Read<std::int32_t>();
        cmd.radius = r.Read<std::int32_t>();
        cmd.r = r.Read<std::uint8_t>();
        cmd.g = r.Read<std::uint8_t>();
        cmd.b = r.Read<std::uint8_t>();
        cmd.a = r.Read<std::uint8_t>();
        cmd.text = r.ReadString();
        cmd.bytes = r.ReadBytes();
        cmd.val1 = r.Read<float>();
        cmd.val2 = r.Read<float>();
        cmd.val3 = r.Read<float>();
        cmd.flag = r.Read<std::uint8_t>() != 0;
        out.commands.push_back(std::move(cmd));
    }
    return out;
}

namespace {
MapCamera ReadMapCamera(Reader& r) {
    MapCamera camera;
    camera.latitude = r.Read<double>();
    camera.longitude = r.Read<double>();
    camera.zoom = r.Read<float>();
    camera.bearing = r.Read<float>();
    camera.pitch = r.Read<float>();
    camera.viewportWidth = r.Read<std::uint32_t>();
    camera.viewportHeight = r.Read<std::uint32_t>();
    return camera;
}
}

MapSceneDelta DecodeMapSceneDelta(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    MapSceneDelta out;
    out.viewportId = r.ReadString();
    if (out.viewportId.empty() || out.viewportId.size() > 1024) throw std::runtime_error("invalid map viewport id");
    out.generation = r.Read<std::uint64_t>();
    const auto resourceCount = r.Read<std::uint32_t>();
    if (resourceCount > 16384) throw std::runtime_error("map resource count exceeds limit");
    std::size_t totalBytes = 0;
    out.resources.reserve(resourceCount);
    for (std::uint32_t i = 0; i < resourceCount; ++i) {
        MapSceneResource resource;
        resource.operation = static_cast<MapResourceOperation>(r.Read<std::uint8_t>());
        resource.type = static_cast<MapResourceType>(r.Read<std::uint8_t>());
        if (resource.operation > MapResourceOperation::Release || resource.type > MapResourceType::TextureAlpha) throw std::runtime_error("invalid map resource enum");
        resource.hash = r.ReadArray<32>();
        resource.stride = r.Read<std::uint32_t>();
        resource.width = r.Read<std::uint32_t>();
        resource.height = r.Read<std::uint32_t>();
        resource.bytes = r.ReadBytes();
        totalBytes += resource.bytes.size();
        if (totalBytes > 256u * 1024u * 1024u || (resource.operation == MapResourceOperation::Release && !resource.bytes.empty())) throw std::runtime_error("invalid map resource bytes");
		if (resource.operation == MapResourceOperation::Upload) {
			if (resource.type == MapResourceType::VertexBuffer) {
				if (resource.stride == 0 || resource.stride > 1024 || resource.bytes.empty() || resource.bytes.size() % resource.stride != 0 || resource.width != 0 || resource.height != 0) throw std::runtime_error("invalid map vertex resource");
			} else if (resource.type == MapResourceType::IndexBuffer) {
				if (resource.stride != 4 || resource.bytes.empty() || resource.bytes.size() % 4 != 0 || resource.width != 0 || resource.height != 0) throw std::runtime_error("invalid map index resource");
			} else {
				const std::uint64_t bytesPerPixel = resource.type == MapResourceType::TextureRGBA ? 4 : 1;
				const std::uint64_t expected = static_cast<std::uint64_t>(resource.width) * resource.height * bytesPerPixel;
				if (resource.width == 0 || resource.height == 0 || resource.width > 4096 || resource.height > 4096 || resource.stride != bytesPerPixel || expected != resource.bytes.size()) throw std::runtime_error("invalid map texture resource");
			}
		}
        out.resources.push_back(std::move(resource));
    }
    const auto drawCount = r.Read<std::uint32_t>();
    if (drawCount > 1000000) throw std::runtime_error("map draw count exceeds limit");
    out.draws.reserve(drawCount);
    for (std::uint32_t i = 0; i < drawCount; ++i) {
        MapDrawBatch draw;
        draw.vertexHash = r.ReadArray<32>();
        draw.indexHash = r.ReadArray<32>();
        draw.textureHash = r.ReadArray<32>();
        draw.primitive = static_cast<MapPrimitive>(r.Read<std::uint8_t>());
        if (draw.primitive > MapPrimitive::Points) throw std::runtime_error("invalid map primitive");
        draw.first = r.Read<std::uint32_t>();
        draw.count = r.Read<std::uint32_t>();
        draw.layer = r.Read<std::int32_t>();
        draw.opacity = r.Read<float>();
		if (!std::isfinite(draw.opacity) || draw.opacity < 0 || draw.opacity > 1) throw std::runtime_error("invalid map opacity");
        draw.depthTest = r.Read<std::uint8_t>() != 0;
        out.draws.push_back(draw);
    }
    out.camera = ReadMapCamera(r);
    out.sunAzimuth = r.Read<float>();
    out.sunElevation = r.Read<float>();
    out.fogDensity = r.Read<float>();
    out.fogRed = r.Read<float>();
    out.fogGreen = r.Read<float>();
    out.fogBlue = r.Read<float>();
    out.shadowCascades = r.Read<std::uint8_t>();
    r.RequireEnd();
    return out;
}

MapCamera DecodeMapCamera(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    auto camera = ReadMapCamera(r);
    r.RequireEnd();
    return camera;
}

PlaySound DecodePlaySound(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    PlaySound out;
    out.type = static_cast<SoundType>(r.Read<std::uint8_t>());
    return out;
}

SetImeVisible DecodeSetImeVisible(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    SetImeVisible out;
    out.visible = r.Read<std::uint8_t>() != 0;
    return out;
}

SemanticTree DecodeSemanticTree(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    SemanticTree out;
    out.revision = r.Read<std::uint64_t>();
    const auto count = r.Read<std::uint32_t>();
    if (count > 100000) {
        throw std::runtime_error("semantic node count exceeds limit");
    }
    out.nodes.reserve(count);
    for (std::uint32_t i = 0; i < count; ++i) {
        SemanticNode node;
        node.parent = r.Read<std::int32_t>();
        node.id = r.ReadString();
        node.role = r.ReadString();
        node.name = r.ReadString();
        node.description = r.ReadString();
		node.accessKey = r.ReadString();
        node.value = r.ReadString();
        node.x1 = r.Read<std::int32_t>();
        node.y1 = r.Read<std::int32_t>();
        node.x2 = r.Read<std::int32_t>();
        node.y2 = r.Read<std::int32_t>();
        node.state = r.Read<std::uint32_t>();
		node.hasRange = r.Read<std::uint8_t>() != 0;
		node.rangeMin = r.Read<double>();
		node.rangeMax = r.Read<double>();
		node.smallChange = r.Read<double>();
		node.largeChange = r.Read<double>();
		node.hasText = r.Read<std::uint8_t>() != 0;
		node.selectionStart = r.Read<std::int32_t>();
		node.selectionEnd = r.Read<std::int32_t>();
		node.multiline = r.Read<std::uint8_t>() != 0;
		node.hasCollection = r.Read<std::uint8_t>() != 0;
		node.canSelectMultiple = r.Read<std::uint8_t>() != 0;
		node.selectionRequired = r.Read<std::uint8_t>() != 0;
		node.hasGrid = r.Read<std::uint8_t>() != 0;
		node.gridRows = r.Read<std::int32_t>();
		node.gridColumns = r.Read<std::int32_t>();
		node.hasGridItem = r.Read<std::uint8_t>() != 0;
		node.gridRow = r.Read<std::int32_t>();
		node.gridColumn = r.Read<std::int32_t>();
		node.gridRowSpan = r.Read<std::int32_t>();
		node.gridColumnSpan = r.Read<std::int32_t>();
		node.hasScroll = r.Read<std::uint8_t>() != 0;
		node.hScrollable = r.Read<std::uint8_t>() != 0;
		node.vScrollable = r.Read<std::uint8_t>() != 0;
		node.hScrollPercent = r.Read<double>();
		node.vScrollPercent = r.Read<double>();
		node.hViewSize = r.Read<double>();
		node.vViewSize = r.Read<double>();
		auto readRelationships = [&r](std::vector<std::string>& relationships) {
			const auto relationshipCount = r.Read<std::uint32_t>();
			if (relationshipCount > 256) {
				throw std::runtime_error("semantic relationship count exceeds limit");
			}
			relationships.reserve(relationshipCount);
			for (std::uint32_t relationshipIndex = 0; relationshipIndex < relationshipCount; ++relationshipIndex) {
				relationships.push_back(r.ReadString());
			}
		};
		readRelationships(node.labeledBy);
		readRelationships(node.describedBy);
		readRelationships(node.controls);
		readRelationships(node.flowsTo);
        const auto actionCount = r.Read<std::uint32_t>();
        if (actionCount > 256) {
            throw std::runtime_error("semantic action count exceeds limit");
        }
        node.actions.reserve(actionCount);
        for (std::uint32_t actionIndex = 0; actionIndex < actionCount; ++actionIndex) {
            node.actions.push_back(r.ReadString());
        }
        out.nodes.push_back(std::move(node));
    }
    return out;
}

NativeDebugRequest DecodeNativeDebugRequest(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    NativeDebugRequest out;
    out.captureFrame = r.Read<std::uint8_t>() != 0;
    out.capturePresentedFrame = r.Read<std::uint8_t>() != 0;
    out.captureDesktopFrame = r.Read<std::uint8_t>() != 0;
    out.restoreWindow = r.Read<std::uint8_t>() != 0;
    out.clampToWorkArea = r.Read<std::uint8_t>() != 0;
    out.bringToForeground = r.Read<std::uint8_t>() != 0;
    out.maximizeWindow = r.Read<std::uint8_t>() != 0;
    out.resetPerf = r.Read<std::uint8_t>() != 0;
    return out;
}

NativeDialogRequest DecodeNativeDialogRequest(const std::vector<std::uint8_t>& body) {
    Reader r(body);
    NativeDialogRequest out;
    out.kind = r.ReadString();
    out.title = r.ReadString();
    out.initialDir = r.ReadString();
    return out;
}

std::vector<std::uint8_t> EncodeEventBatch(const EventBatch& batch) {
    Writer w;
    w.Write<std::uint32_t>(static_cast<std::uint32_t>(batch.events.size()));
    for (const auto& ev : batch.events) {
        w.Write<std::uint8_t>(static_cast<std::uint8_t>(ev.type));
        w.Write<std::int32_t>(ev.x);
        w.Write<std::int32_t>(ev.y);
        w.Write<std::int32_t>(ev.button);
        w.Write<std::int32_t>(ev.delta);
        w.Write<std::uint32_t>(ev.keycode);
        w.Write<std::uint32_t>(ev.ch);
        w.Write<std::int32_t>(ev.width);
        w.Write<std::int32_t>(ev.height);
        w.WriteString(ev.text);
		w.WriteString(ev.target);
		w.WriteString(ev.action);
		w.WriteString(ev.value);
		w.Write<std::int32_t>(ev.deltaX);
		w.Write<std::int32_t>(ev.deltaY);
		w.Write<float>(ev.scale);
		w.Write<std::uint8_t>(static_cast<std::uint8_t>(ev.phase));
		w.WriteBytes(ev.bytes);
    }
    return w.Finish(MessageType::EventBatch);
}

std::vector<std::uint8_t> EncodeNativeDebugResponse(const NativeDebugResponse& response) {
    Writer w;
    w.WriteString(response.error);
    w.Write<std::int32_t>(response.dpi);
    w.Write<std::uint8_t>(response.windowVisible ? 1 : 0);
    w.Write<std::uint8_t>(response.windowMinimized ? 1 : 0);
    w.Write<std::uint8_t>(response.windowForeground ? 1 : 0);
    w.Write<std::int32_t>(response.windowLeft);
    w.Write<std::int32_t>(response.windowTop);
    w.Write<std::int32_t>(response.windowRight);
    w.Write<std::int32_t>(response.windowBottom);
    w.Write<std::int32_t>(response.clientWidth);
    w.Write<std::int32_t>(response.clientHeight);
    w.Write<std::int32_t>(response.workLeft);
    w.Write<std::int32_t>(response.workTop);
    w.Write<std::int32_t>(response.workRight);
    w.Write<std::int32_t>(response.workBottom);
    w.Write<std::int32_t>(response.backbufferWidth);
    w.Write<std::int32_t>(response.backbufferHeight);
    w.Write<std::int32_t>(response.frameWidth);
    w.Write<std::int32_t>(response.frameHeight);
    w.WriteBytes(response.frameRgba);
    w.Write<std::uint32_t>(static_cast<std::uint32_t>(response.perfPhases.size()));
    for (const auto& phase : response.perfPhases) {
        w.WriteString(phase.name);
        w.Write<std::uint32_t>(phase.count);
        w.Write<double>(phase.meanMS);
        w.Write<double>(phase.p50MS);
        w.Write<double>(phase.p95MS);
        w.Write<double>(phase.p99MS);
        w.Write<double>(phase.maxMS);
    }
    return w.Finish(MessageType::NativeDebugResponse);
}

std::vector<std::uint8_t> EncodeNativeDialogResponse(const NativeDialogResponse& response) {
    Writer w;
    w.WriteString(response.error);
    w.Write<std::uint8_t>(response.canceled ? 1 : 0);
    w.WriteString(response.path);
    return w.Finish(MessageType::NativeDialogResponse);
}

} // namespace poem::protocol
