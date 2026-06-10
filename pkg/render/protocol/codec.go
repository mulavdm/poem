package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func EncodeInitEngine(msg InitEngine) ([]byte, error) {
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
	return wrapEnvelope(MessageInitEngine, body.Bytes()), nil
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

func EncodeEventBatch(msg EventBatch) ([]byte, error) {
	var body bytes.Buffer
	writeUint32(&body, uint32(len(msg.Events)))
	for _, ev := range msg.Events {
		writeByte(&body, byte(ev.Type))
		writeInt32(&body, ev.X)
		writeInt32(&body, ev.Y)
		writeInt32(&body, ev.Button)
		writeInt32(&body, ev.Delta)
		writeUint32(&body, ev.Keycode)
		writeUint32(&body, ev.Char)
		writeInt32(&body, ev.Width)
		writeInt32(&body, ev.Height)
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
	return NativeDebugRequest{
		CaptureFrame:          captureFrame,
		CapturePresentedFrame: capturePresentedFrame,
		CaptureDesktopFrame:   captureDesktopFrame,
		RestoreWindow:         restoreWindow,
		ClampToWorkArea:       clampToWorkArea,
		BringToForeground:     bringToForeground,
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
func readBytes(r io.Reader) ([]byte, error) {
	size, err := readUint32(r)
	if err != nil {
		return nil, err
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
