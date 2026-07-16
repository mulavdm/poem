#include "protocol.h"

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
