package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	maxProtocolByteVector = 256 << 20
	maxProtocolEvents     = 100_000
	maxProtocolCommands   = 1_000_000
)

func EncodeInitEngine(msg InitEngine) ([]byte, error) {
	return encodeFontPayload(MessageInitEngine, msg)
}

func EncodeFontAtlas(msg InitEngine) ([]byte, error) {
	return encodeFontPayload(MessageFontAtlas, msg)
}

func encodeFontPayload(messageType MessageType, msg InitEngine) ([]byte, error) {
	var body bytes.Buffer
	writeInt32(&body, msg.Width)
	writeInt32(&body, msg.Height)
	writeInt32(&body, msg.AtlasWidth)
	writeInt32(&body, msg.AtlasHeight)
	writeBytes(&body, msg.AtlasPixels)
	writeUint32(&body, uint32(len(msg.Chars)))
	for _, ch := range msg.Chars {
		writeInt32(&body, ch.R)
		writeFloat32(&body, ch.U1)
		writeFloat32(&body, ch.V1)
		writeFloat32(&body, ch.U2)
		writeFloat32(&body, ch.V2)
		writeInt32(&body, ch.Width)
		writeInt32(&body, ch.Height)
		writeInt32(&body, ch.Advance)
	}
	return wrapEnvelope(messageType, body.Bytes()), nil
}

func EncodeRenderFrame(msg RenderFrame) ([]byte, error) {
	var body bytes.Buffer
	writeInt32(&body, msg.Width)
	writeInt32(&body, msg.Height)
	writeByte(&body, msg.Cursor)
	writeUint32(&body, uint32(len(msg.Commands)))
	for _, cmd := range msg.Commands {
		writeByte(&body, byte(cmd.Type))
		writeInt32(&body, cmd.X1)
		writeInt32(&body, cmd.Y1)
		writeInt32(&body, cmd.X2)
		writeInt32(&body, cmd.Y2)
		writeInt32(&body, cmd.W)
		writeInt32(&body, cmd.H)
		writeInt32(&body, cmd.Radius)
		writeByte(&body, cmd.R)
		writeByte(&body, cmd.G)
		writeByte(&body, cmd.B)
		writeByte(&body, cmd.A)
		writeString(&body, cmd.Text)
		writeBytes(&body, cmd.Bytes)
		writeFloat32(&body, cmd.Val1)
		writeFloat32(&body, cmd.Val2)
		writeFloat32(&body, cmd.Val3)
		writeBool(&body, cmd.Flag)
	}
	return wrapEnvelope(MessageRenderFrame, body.Bytes()), nil
}

func EncodePlaySound(msg PlaySound) ([]byte, error) {
	var body bytes.Buffer
	writeByte(&body, byte(msg.Type))
	return wrapEnvelope(MessagePlaySound, body.Bytes()), nil
}
func EncodeWindowCloseResponse(msg WindowCloseResponse) ([]byte, error) {
	var body bytes.Buffer
	writeBool(&body, msg.Allow)
	return wrapEnvelope(MessageWindowCloseResponse, body.Bytes()), nil
}

func EncodeSetImeVisible(msg SetImeVisible) ([]byte, error) {
	var body bytes.Buffer
	writeBool(&body, msg.Visible)
	return wrapEnvelope(MessageSetImeVisible, body.Bytes()), nil
}

func EncodeSemanticTree(msg SemanticTree) ([]byte, error) {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.LittleEndian, msg.Revision)
	writeUint32(&body, uint32(len(msg.Nodes)))
	for _, node := range msg.Nodes {
		writeInt32(&body, node.Parent)
		writeString(&body, node.ID)
		writeString(&body, node.Role)
		writeString(&body, node.Name)
		writeString(&body, node.Description)
		writeString(&body, node.AccessKey)
		writeString(&body, node.Value)
		writeInt32(&body, node.X1)
		writeInt32(&body, node.Y1)
		writeInt32(&body, node.X2)
		writeInt32(&body, node.Y2)
		writeUint32(&body, node.State)
		writeBool(&body, node.HasRange)
		writeFloat64(&body, node.RangeMin)
		writeFloat64(&body, node.RangeMax)
		writeFloat64(&body, node.SmallChange)
		writeFloat64(&body, node.LargeChange)
		writeBool(&body, node.HasText)
		writeInt32(&body, node.SelectionStart)
		writeInt32(&body, node.SelectionEnd)
		writeBool(&body, node.Multiline)
		writeBool(&body, node.HasCollection)
		writeBool(&body, node.CanSelectMultiple)
		writeBool(&body, node.SelectionRequired)
		writeBool(&body, node.HasGrid)
		writeInt32(&body, node.GridRows)
		writeInt32(&body, node.GridColumns)
		writeBool(&body, node.HasGridItem)
		writeInt32(&body, node.GridRow)
		writeInt32(&body, node.GridColumn)
		writeInt32(&body, node.GridRowSpan)
		writeInt32(&body, node.GridColumnSpan)
		writeBool(&body, node.HasScroll)
		writeBool(&body, node.HScrollable)
		writeBool(&body, node.VScrollable)
		writeFloat64(&body, node.HScrollPercent)
		writeFloat64(&body, node.VScrollPercent)
		writeFloat64(&body, node.HViewSize)
		writeFloat64(&body, node.VViewSize)
		for _, relationships := range [][]string{node.LabeledBy, node.DescribedBy, node.Controls, node.FlowsTo} {
			if len(relationships) > 256 {
				return nil, fmt.Errorf("semantic relationship count %d exceeds limit", len(relationships))
			}
			writeUint32(&body, uint32(len(relationships)))
			for _, target := range relationships {
				writeString(&body, target)
			}
		}
		writeUint32(&body, uint32(len(node.Actions)))
		for _, action := range node.Actions {
			writeString(&body, action)
		}
	}
	return wrapEnvelope(MessageSemanticTree, body.Bytes()), nil
}

func DecodeSemanticTree(payload []byte) (SemanticTree, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return SemanticTree{}, err
	}
	if msgType != MessageSemanticTree {
		return SemanticTree{}, fmt.Errorf("unexpected message type %d", msgType)
	}
	r := bytes.NewReader(body)
	var revision uint64
	if err := binary.Read(r, binary.LittleEndian, &revision); err != nil {
		return SemanticTree{}, err
	}
	count, err := readUint32(r)
	if err != nil {
		return SemanticTree{}, err
	}
	if count > 100_000 {
		return SemanticTree{}, fmt.Errorf("semantic node count %d exceeds limit", count)
	}
	nodes := make([]SemanticNode, 0, count)
	for i := uint32(0); i < count; i++ {
		var node SemanticNode
		if node.Parent, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.ID, err = readString(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Role, err = readString(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Name, err = readString(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Description, err = readString(r); err != nil {
			return SemanticTree{}, err
		}
		if node.AccessKey, err = readString(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Value, err = readString(r); err != nil {
			return SemanticTree{}, err
		}
		if node.X1, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Y1, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.X2, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Y2, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.State, err = readUint32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HasRange, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.RangeMin, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.RangeMax, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.SmallChange, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.LargeChange, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HasText, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.SelectionStart, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.SelectionEnd, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.Multiline, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HasCollection, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.CanSelectMultiple, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.SelectionRequired, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HasGrid, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.GridRows, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.GridColumns, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HasGridItem, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.GridRow, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.GridColumn, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.GridRowSpan, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.GridColumnSpan, err = readInt32(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HasScroll, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HScrollable, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.VScrollable, err = readBool(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HScrollPercent, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.VScrollPercent, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.HViewSize, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		if node.VViewSize, err = readFloat64(r); err != nil {
			return SemanticTree{}, err
		}
		relationships := []*[]string{&node.LabeledBy, &node.DescribedBy, &node.Controls, &node.FlowsTo}
		for _, targets := range relationships {
			relationshipCount, relationshipErr := readUint32(r)
			if relationshipErr != nil {
				return SemanticTree{}, relationshipErr
			}
			if relationshipCount > 256 {
				return SemanticTree{}, fmt.Errorf("semantic relationship count %d exceeds limit", relationshipCount)
			}
			for relationshipIndex := uint32(0); relationshipIndex < relationshipCount; relationshipIndex++ {
				target, targetErr := readString(r)
				if targetErr != nil {
					return SemanticTree{}, targetErr
				}
				*targets = append(*targets, target)
			}
		}
		actionCount, actionErr := readUint32(r)
		if actionErr != nil {
			return SemanticTree{}, actionErr
		}
		if actionCount > 256 {
			return SemanticTree{}, fmt.Errorf("semantic action count %d exceeds limit", actionCount)
		}
		for actionIndex := uint32(0); actionIndex < actionCount; actionIndex++ {
			action, actionErr := readString(r)
			if actionErr != nil {
				return SemanticTree{}, actionErr
			}
			node.Actions = append(node.Actions, action)
		}
		nodes = append(nodes, node)
	}
	return SemanticTree{Revision: revision, Nodes: nodes}, nil
}

func EncodeEventBatch(msg EventBatch) ([]byte, error) {
	var body bytes.Buffer
	writeUint32(&body, uint32(len(msg.Events)))
	for _, ev := range msg.Events {
		if len(ev.Text) > 1<<20 || len(ev.Target) > 1<<20 || len(ev.Action) > 1<<20 || len(ev.Value) > 1<<20 || len(ev.Bytes) > 1<<20 {
			return nil, fmt.Errorf("event string exceeds 1 MiB")
		}
		writeByte(&body, byte(ev.Type))
		writeInt32(&body, ev.X)
		writeInt32(&body, ev.Y)
		writeInt32(&body, ev.Button)
		writeInt32(&body, ev.Delta)
		writeUint32(&body, ev.Keycode)
		writeUint32(&body, ev.Char)
		writeInt32(&body, ev.Width)
		writeInt32(&body, ev.Height)
		writeString(&body, ev.Text)
		writeString(&body, ev.Target)
		writeString(&body, ev.Action)
		writeString(&body, ev.Value)
		writeInt32(&body, ev.DeltaX)
		writeInt32(&body, ev.DeltaY)
		writeFloat32(&body, ev.Scale)
		writeByte(&body, byte(ev.Phase))
		writeBytes(&body, ev.Bytes)
	}
	return wrapEnvelope(MessageEventBatch, body.Bytes()), nil
}

func EncodeNativeDebugRequest(msg NativeDebugRequest) ([]byte, error) {
	var body bytes.Buffer
	writeBool(&body, msg.CaptureFrame)
	writeBool(&body, msg.CapturePresentedFrame)
	writeBool(&body, msg.CaptureDesktopFrame)
	writeBool(&body, msg.RestoreWindow)
	writeBool(&body, msg.ClampToWorkArea)
	writeBool(&body, msg.BringToForeground)
	writeBool(&body, msg.MaximizeWindow)
	writeBool(&body, msg.ResetPerf)
	writeInt32(&body, msg.ResizeWidth)
	writeInt32(&body, msg.ResizeHeight)
	return wrapEnvelope(MessageNativeDebugRequest, body.Bytes()), nil
}

func DecodeNativeDebugRequest(payload []byte) (NativeDebugRequest, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	if msgType != MessageNativeDebugRequest {
		return NativeDebugRequest{}, fmt.Errorf("unexpected message type %d", msgType)
	}
	r := bytes.NewReader(body)
	captureFrame, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	capturePresentedFrame, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	captureDesktopFrame, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	restoreWindow, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	clampToWorkArea, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	bringToForeground, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	maximizeWindow, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	resetPerf, err := readBool(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	resizeWidth, err := readInt32(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	resizeHeight, err := readInt32(r)
	if err != nil {
		return NativeDebugRequest{}, err
	}
	return NativeDebugRequest{
		CaptureFrame:          captureFrame,
		CapturePresentedFrame: capturePresentedFrame,
		CaptureDesktopFrame:   captureDesktopFrame,
		RestoreWindow:         restoreWindow,
		ClampToWorkArea:       clampToWorkArea,
		BringToForeground:     bringToForeground,
		MaximizeWindow:        maximizeWindow,
		ResetPerf:             resetPerf,
		ResizeWidth:           resizeWidth,
		ResizeHeight:          resizeHeight,
	}, nil
}

func EncodeNativeDebugResponse(msg NativeDebugResponse) ([]byte, error) {
	var body bytes.Buffer
	writeString(&body, msg.Error)
	writeInt32(&body, msg.DPI)
	writeBool(&body, msg.WindowVisible)
	writeBool(&body, msg.WindowMinimized)
	writeBool(&body, msg.WindowForeground)
	writeInt32(&body, msg.WindowLeft)
	writeInt32(&body, msg.WindowTop)
	writeInt32(&body, msg.WindowRight)
	writeInt32(&body, msg.WindowBottom)
	writeInt32(&body, msg.ClientWidth)
	writeInt32(&body, msg.ClientHeight)
	writeInt32(&body, msg.WorkLeft)
	writeInt32(&body, msg.WorkTop)
	writeInt32(&body, msg.WorkRight)
	writeInt32(&body, msg.WorkBottom)
	writeInt32(&body, msg.BackbufferWidth)
	writeInt32(&body, msg.BackbufferHeight)
	writeInt32(&body, msg.FrameWidth)
	writeInt32(&body, msg.FrameHeight)
	writeBytes(&body, msg.FrameRGBA)
	writeUint32(&body, uint32(len(msg.PerfPhases)))
	for _, phase := range msg.PerfPhases {
		writeString(&body, phase.Name)
		writeUint32(&body, phase.Count)
		writeFloat64(&body, phase.MeanMS)
		writeFloat64(&body, phase.P50MS)
		writeFloat64(&body, phase.P95MS)
		writeFloat64(&body, phase.P99MS)
		writeFloat64(&body, phase.MaxMS)
	}
	return wrapEnvelope(MessageNativeDebugResponse, body.Bytes()), nil
}

func DecodeNativeDebugResponse(payload []byte) (NativeDebugResponse, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return NativeDebugResponse{}, err
	}
	if msgType != MessageNativeDebugResponse {
		return NativeDebugResponse{}, fmt.Errorf("unexpected message type %d", msgType)
	}
	r := bytes.NewReader(body)
	out := NativeDebugResponse{}
	if out.Error, err = readString(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.DPI, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowVisible, err = readBool(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowMinimized, err = readBool(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowForeground, err = readBool(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowLeft, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowTop, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowRight, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WindowBottom, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.ClientWidth, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.ClientHeight, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WorkLeft, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WorkTop, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WorkRight, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.WorkBottom, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.BackbufferWidth, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.BackbufferHeight, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.FrameWidth, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.FrameHeight, err = readInt32(r); err != nil {
		return NativeDebugResponse{}, err
	}
	if out.FrameRGBA, err = readBytes(r); err != nil {
		return NativeDebugResponse{}, err
	}
	phaseCount, err := readUint32(r)
	if err != nil {
		return NativeDebugResponse{}, err
	}
	// One phase per timed native channel; the host currently reports three.
	// The bound keeps a malformed count from provoking a huge allocation.
	const maxPerfPhases = 64
	if phaseCount > maxPerfPhases {
		return NativeDebugResponse{}, fmt.Errorf("native perf phase count %d exceeds limit %d", phaseCount, maxPerfPhases)
	}
	for i := uint32(0); i < phaseCount; i++ {
		var phase NativePerfPhase
		if phase.Name, err = readString(r); err != nil {
			return NativeDebugResponse{}, err
		}
		if phase.Count, err = readUint32(r); err != nil {
			return NativeDebugResponse{}, err
		}
		if phase.MeanMS, err = readFloat64(r); err != nil {
			return NativeDebugResponse{}, err
		}
		if phase.P50MS, err = readFloat64(r); err != nil {
			return NativeDebugResponse{}, err
		}
		if phase.P95MS, err = readFloat64(r); err != nil {
			return NativeDebugResponse{}, err
		}
		if phase.P99MS, err = readFloat64(r); err != nil {
			return NativeDebugResponse{}, err
		}
		if phase.MaxMS, err = readFloat64(r); err != nil {
			return NativeDebugResponse{}, err
		}
		out.PerfPhases = append(out.PerfPhases, phase)
	}
	return out, nil
}

func EncodeNativeDialogRequest(msg NativeDialogRequest) ([]byte, error) {
	var body bytes.Buffer
	writeString(&body, msg.Kind)
	writeString(&body, msg.Title)
	writeString(&body, msg.InitialDir)
	return wrapEnvelope(MessageNativeDialogRequest, body.Bytes()), nil
}

func DecodeNativeDialogRequest(payload []byte) (NativeDialogRequest, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return NativeDialogRequest{}, err
	}
	if msgType != MessageNativeDialogRequest {
		return NativeDialogRequest{}, fmt.Errorf("unexpected message type %d", msgType)
	}
	r := bytes.NewReader(body)
	out := NativeDialogRequest{}
	if out.Kind, err = readString(r); err != nil {
		return NativeDialogRequest{}, err
	}
	if out.Title, err = readString(r); err != nil {
		return NativeDialogRequest{}, err
	}
	if out.InitialDir, err = readString(r); err != nil {
		return NativeDialogRequest{}, err
	}
	return out, nil
}

func EncodeNativeDialogResponse(msg NativeDialogResponse) ([]byte, error) {
	var body bytes.Buffer
	writeString(&body, msg.Error)
	writeBool(&body, msg.Canceled)
	writeString(&body, msg.Path)
	return wrapEnvelope(MessageNativeDialogResponse, body.Bytes()), nil
}

func DecodeNativeDialogResponse(payload []byte) (NativeDialogResponse, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return NativeDialogResponse{}, err
	}
	if msgType != MessageNativeDialogResponse {
		return NativeDialogResponse{}, fmt.Errorf("unexpected message type %d", msgType)
	}
	r := bytes.NewReader(body)
	out := NativeDialogResponse{}
	if out.Error, err = readString(r); err != nil {
		return NativeDialogResponse{}, err
	}
	if out.Canceled, err = readBool(r); err != nil {
		return NativeDialogResponse{}, err
	}
	if out.Path, err = readString(r); err != nil {
		return NativeDialogResponse{}, err
	}
	return out, nil
}

func DecodeEnvelope(payload []byte) (MessageType, []byte, error) {
	if len(payload) < 8 {
		return 0, nil, fmt.Errorf("payload too short: %d", len(payload))
	}
	if string(payload[:4]) != Magic {
		return 0, nil, fmt.Errorf("invalid magic")
	}
	version := binary.LittleEndian.Uint16(payload[4:6])
	if version != Version {
		return 0, nil, fmt.Errorf("unsupported protocol version %d", version)
	}
	msgType := MessageType(binary.LittleEndian.Uint16(payload[6:8]))
	return msgType, payload[8:], nil
}

func DecodeEventBatch(payload []byte) (EventBatch, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return EventBatch{}, err
	}
	if msgType != MessageEventBatch {
		return EventBatch{}, fmt.Errorf("unexpected message type %d", msgType)
	}
	r := bytes.NewReader(body)
	count, err := readUint32(r)
	if err != nil {
		return EventBatch{}, err
	}
	if count > maxProtocolEvents {
		return EventBatch{}, fmt.Errorf("event count %d exceeds limit", count)
	}
	events := make([]Event, 0, count)
	for i := uint32(0); i < count; i++ {
		t, err := readByte(r)
		if err != nil {
			return EventBatch{}, err
		}
		x, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		y, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		button, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		delta, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		keycode, err := readUint32(r)
		if err != nil {
			return EventBatch{}, err
		}
		charCode, err := readUint32(r)
		if err != nil {
			return EventBatch{}, err
		}
		width, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		height, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		text, err := readString(r)
		if err != nil {
			return EventBatch{}, err
		}
		target, err := readString(r)
		if err != nil {
			return EventBatch{}, err
		}
		action, err := readString(r)
		if err != nil {
			return EventBatch{}, err
		}
		value, err := readString(r)
		if err != nil {
			return EventBatch{}, err
		}
		if len(text) > 1<<20 || len(target) > 1<<20 || len(action) > 1<<20 || len(value) > 1<<20 {
			return EventBatch{}, fmt.Errorf("event string exceeds 1 MiB")
		}
		deltaX, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		deltaY, err := readInt32(r)
		if err != nil {
			return EventBatch{}, err
		}
		scale, err := readFloat32(r)
		if err != nil {
			return EventBatch{}, err
		}
		phase, err := readByte(r)
		if err != nil {
			return EventBatch{}, err
		}
		opaque, err := readBytes(r)
		if err != nil || len(opaque) > 1<<20 {
			return EventBatch{}, fmt.Errorf("invalid realtime viewport event payload")
		}
		if len(opaque) == 0 {
			opaque = nil
		}
		events = append(events, Event{
			Type:    EventType(t),
			X:       x,
			Y:       y,
			Button:  button,
			Delta:   delta,
			Keycode: keycode,
			Char:    charCode,
			Width:   width,
			Height:  height,
			Text:    text,
			Target:  target,
			Action:  action,
			Value:   value,
			DeltaX:  deltaX,
			DeltaY:  deltaY,
			Scale:   scale,
			Phase:   GesturePhase(phase),
			Bytes:   opaque,
		})
	}
	return EventBatch{Events: events}, nil
}

func DecodeRenderFrame(payload []byte) (RenderFrame, error) {
	msgType, body, err := DecodeEnvelope(payload)
	if err != nil {
		return RenderFrame{}, err
	}
	if msgType != MessageRenderFrame {
		return RenderFrame{}, fmt.Errorf("unexpected message type %d", msgType)
	}

	r := bytes.NewReader(body)
	width, err := readInt32(r)
	if err != nil {
		return RenderFrame{}, err
	}
	height, err := readInt32(r)
	if err != nil {
		return RenderFrame{}, err
	}
	cursor, err := readByte(r)
	if err != nil {
		return RenderFrame{}, err
	}
	count, err := readUint32(r)
	if err != nil {
		return RenderFrame{}, err
	}
	if count > maxProtocolCommands {
		return RenderFrame{}, fmt.Errorf("draw command count %d exceeds limit", count)
	}

	commands := make([]DrawCommand, 0, count)
	for i := uint32(0); i < count; i++ {
		commandType, err := readByte(r)
		if err != nil {
			return RenderFrame{}, err
		}
		x1, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		y1, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		x2, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		y2, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		w, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		h, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		radius, err := readInt32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		cr, err := readByte(r)
		if err != nil {
			return RenderFrame{}, err
		}
		cg, err := readByte(r)
		if err != nil {
			return RenderFrame{}, err
		}
		cb, err := readByte(r)
		if err != nil {
			return RenderFrame{}, err
		}
		ca, err := readByte(r)
		if err != nil {
			return RenderFrame{}, err
		}
		text, err := readString(r)
		if err != nil {
			return RenderFrame{}, err
		}
		bytesValue, err := readBytes(r)
		if err != nil {
			return RenderFrame{}, err
		}
		val1, err := readFloat32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		val2, err := readFloat32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		val3, err := readFloat32(r)
		if err != nil {
			return RenderFrame{}, err
		}
		flag, err := readBool(r)
		if err != nil {
			return RenderFrame{}, err
		}

		commands = append(commands, DrawCommand{
			Type:   DrawCommandType(commandType),
			X1:     x1,
			Y1:     y1,
			X2:     x2,
			Y2:     y2,
			W:      w,
			H:      h,
			Radius: radius,
			R:      cr,
			G:      cg,
			B:      cb,
			A:      ca,
			Text:   text,
			Bytes:  bytesValue,
			Val1:   val1,
			Val2:   val2,
			Val3:   val3,
			Flag:   flag,
		})
	}

	return RenderFrame{
		Width:    width,
		Height:   height,
		Cursor:   cursor,
		Commands: commands,
	}, nil
}

func wrapEnvelope(msgType MessageType, body []byte) []byte {
	buf := make([]byte, 8+len(body))
	copy(buf[:4], []byte(Magic))
	binary.LittleEndian.PutUint16(buf[4:6], Version)
	binary.LittleEndian.PutUint16(buf[6:8], uint16(msgType))
	copy(buf[8:], body)
	return buf
}

func writeByte(w io.Writer, v byte) { _, _ = w.Write([]byte{v}) }
func writeBool(w io.Writer, v bool) {
	if v {
		writeByte(w, 1)
	} else {
		writeByte(w, 0)
	}
}
func writeUint32(w io.Writer, v uint32) { _ = binary.Write(w, binary.LittleEndian, v) }
func writeInt32(w io.Writer, v int32)   { _ = binary.Write(w, binary.LittleEndian, v) }
func writeFloat32(w io.Writer, v float32) {
	_ = binary.Write(w, binary.LittleEndian, v)
}

func writeFloat64(w io.Writer, v float64) {
	_ = binary.Write(w, binary.LittleEndian, v)
}
func writeBytes(w io.Writer, b []byte) {
	writeUint32(w, uint32(len(b)))
	_, _ = w.Write(b)
}
func writeString(w io.Writer, s string) { writeBytes(w, []byte(s)) }

func readByte(r io.Reader) (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	return b[0], err
}
func readUint32(r io.Reader) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}
func readInt32(r io.Reader) (int32, error) {
	var v int32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}
func readFloat32(r io.Reader) (float32, error) {
	var v float32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func readFloat64(r io.Reader) (float64, error) {
	var v float64
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}
func readBytes(r io.Reader) ([]byte, error) {
	size, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	if size > maxProtocolByteVector {
		return nil, fmt.Errorf("protocol byte vector length %d exceeds limit", size)
	}
	buf := make([]byte, size)
	_, err = io.ReadFull(r, buf)
	return buf, err
}
func readString(r io.Reader) (string, error) {
	bytes, err := readBytes(r)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
func readBool(r io.Reader) (bool, error) {
	val, err := readByte(r)
	if err != nil {
		return false, err
	}
	return val != 0, nil
}
