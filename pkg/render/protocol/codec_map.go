package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const (
	maxMapSceneResources = 16_384
	maxMapSceneDraws     = 1_000_000
)

func EncodeMapSceneDelta(msg MapSceneDelta) ([]byte, error) {
	if len(msg.ViewportID) == 0 || len(msg.ViewportID) > 1024 || len(msg.Resources) > maxMapSceneResources || len(msg.Draws) > maxMapSceneDraws {
		return nil, fmt.Errorf("map scene exceeds protocol bounds")
	}
	var total int
	var body bytes.Buffer
	writeString(&body, msg.ViewportID)
	_ = binary.Write(&body, binary.LittleEndian, msg.Generation)
	writeUint32(&body, uint32(len(msg.Resources)))
	for _, resource := range msg.Resources {
		if resource.Operation > MapResourceRelease || resource.Type > MapResourceTextureAlpha {
			return nil, fmt.Errorf("invalid map resource type or operation")
		}
		if resource.Operation == MapResourceRelease && len(resource.Bytes) != 0 {
			return nil, fmt.Errorf("released map resource carries bytes")
		}
		if err := validateMapResource(resource); err != nil {
			return nil, err
		}
		total += len(resource.Bytes)
		if total > maxProtocolByteVector {
			return nil, fmt.Errorf("map resource bytes exceed limit")
		}
		writeByte(&body, byte(resource.Operation))
		writeByte(&body, byte(resource.Type))
		body.Write(resource.Hash[:])
		writeUint32(&body, resource.Stride)
		writeUint32(&body, resource.Width)
		writeUint32(&body, resource.Height)
		writeBytes(&body, resource.Bytes)
	}
	writeUint32(&body, uint32(len(msg.Draws)))
	for _, draw := range msg.Draws {
		if draw.Primitive > MapPrimitivePoints || math.IsNaN(float64(draw.Opacity)) || math.IsInf(float64(draw.Opacity), 0) || draw.Opacity < 0 || draw.Opacity > 1 {
			return nil, fmt.Errorf("invalid map primitive")
		}
		body.Write(draw.VertexHash[:])
		body.Write(draw.IndexHash[:])
		body.Write(draw.TextureHash[:])
		writeByte(&body, byte(draw.Primitive))
		writeUint32(&body, draw.First)
		writeUint32(&body, draw.Count)
		writeInt32(&body, draw.Layer)
		writeFloat32(&body, draw.Opacity)
		writeBool(&body, draw.DepthTest)
	}
	writeMapCamera(&body, msg.Camera)
	writeFloat32(&body, msg.SunAzimuth)
	writeFloat32(&body, msg.SunElevation)
	// Appended after the sun fields; existing values keep their positions.
	writeFloat32(&body, msg.FogDensity)
	writeFloat32(&body, msg.FogRed)
	writeFloat32(&body, msg.FogGreen)
	writeFloat32(&body, msg.FogBlue)
	writeByte(&body, msg.ShadowCascades)
	return wrapEnvelope(MessageMapSceneDelta, body.Bytes()), nil
}

func DecodeMapSceneDelta(payload []byte) (MapSceneDelta, error) {
	typeID, body, err := DecodeEnvelope(payload)
	if err != nil {
		return MapSceneDelta{}, err
	}
	if typeID != MessageMapSceneDelta {
		return MapSceneDelta{}, fmt.Errorf("unexpected message type %d", typeID)
	}
	r := bytes.NewReader(body)
	viewportID, err := readString(r)
	if err != nil || viewportID == "" || len(viewportID) > 1024 {
		return MapSceneDelta{}, fmt.Errorf("invalid map viewport id")
	}
	var generation uint64
	if err := binary.Read(r, binary.LittleEndian, &generation); err != nil {
		return MapSceneDelta{}, err
	}
	resourceCount, err := readUint32(r)
	if err != nil || resourceCount > maxMapSceneResources {
		return MapSceneDelta{}, fmt.Errorf("invalid map resource count")
	}
	resources := make([]MapSceneResource, 0, resourceCount)
	var total int
	for range resourceCount {
		operation, err := readByte(r)
		if err != nil {
			return MapSceneDelta{}, err
		}
		resourceType, err := readByte(r)
		if err != nil {
			return MapSceneDelta{}, err
		}
		resource := MapSceneResource{Operation: MapResourceOperation(operation), Type: MapResourceType(resourceType)}
		if resource.Operation > MapResourceRelease || resource.Type > MapResourceTextureAlpha {
			return MapSceneDelta{}, fmt.Errorf("invalid map resource type or operation")
		}
		if _, err := io.ReadFull(r, resource.Hash[:]); err != nil {
			return MapSceneDelta{}, err
		}
		if resource.Stride, err = readUint32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if resource.Width, err = readUint32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if resource.Height, err = readUint32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if resource.Bytes, err = readBytes(r); err != nil {
			return MapSceneDelta{}, err
		}
		total += len(resource.Bytes)
		if total > maxProtocolByteVector || resource.Operation == MapResourceRelease && len(resource.Bytes) != 0 {
			return MapSceneDelta{}, fmt.Errorf("invalid map resource bytes")
		}
		if err := validateMapResource(resource); err != nil {
			return MapSceneDelta{}, err
		}
		resources = append(resources, resource)
	}
	drawCount, err := readUint32(r)
	if err != nil || drawCount > maxMapSceneDraws {
		return MapSceneDelta{}, fmt.Errorf("invalid map draw count")
	}
	draws := make([]MapDrawBatch, 0, drawCount)
	for range drawCount {
		var draw MapDrawBatch
		for _, hash := range []*[32]byte{&draw.VertexHash, &draw.IndexHash, &draw.TextureHash} {
			if _, err := io.ReadFull(r, hash[:]); err != nil {
				return MapSceneDelta{}, err
			}
		}
		primitive, err := readByte(r)
		if err != nil || MapPrimitive(primitive) > MapPrimitivePoints {
			return MapSceneDelta{}, fmt.Errorf("invalid map primitive")
		}
		draw.Primitive = MapPrimitive(primitive)
		if draw.First, err = readUint32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if draw.Count, err = readUint32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if draw.Layer, err = readInt32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if draw.Opacity, err = readFloat32(r); err != nil {
			return MapSceneDelta{}, err
		}
		if math.IsNaN(float64(draw.Opacity)) || math.IsInf(float64(draw.Opacity), 0) || draw.Opacity < 0 || draw.Opacity > 1 {
			return MapSceneDelta{}, fmt.Errorf("invalid map opacity")
		}
		if draw.DepthTest, err = readBool(r); err != nil {
			return MapSceneDelta{}, err
		}
		draws = append(draws, draw)
	}
	camera, err := readMapCamera(r)
	if err != nil {
		return MapSceneDelta{}, err
	}
	sunAzimuth, err := readFloat32(r)
	if err != nil {
		return MapSceneDelta{}, err
	}
	sunElevation, err := readFloat32(r)
	if err != nil {
		return MapSceneDelta{}, err
	}
	fog := [4]float32{}
	for index := range fog {
		if fog[index], err = readFloat32(r); err != nil {
			return MapSceneDelta{}, err
		}
	}
	shadowCascades, err := r.ReadByte()
	if err != nil {
		return MapSceneDelta{}, err
	}
	if r.Len() != 0 {
		return MapSceneDelta{}, fmt.Errorf("map scene has trailing bytes")
	}
	return MapSceneDelta{ViewportID: viewportID, Generation: generation, Resources: resources, Draws: draws, Camera: camera,
		SunAzimuth: sunAzimuth, SunElevation: sunElevation,
		FogDensity: fog[0], FogRed: fog[1], FogGreen: fog[2], FogBlue: fog[3],
		ShadowCascades: shadowCascades}, nil
}

func validateMapResource(resource MapSceneResource) error {
	if resource.Operation == MapResourceRelease {
		return nil
	}
	switch resource.Type {
	case MapResourceVertexBuffer:
		if resource.Stride == 0 || resource.Stride > 1024 || len(resource.Bytes) == 0 || len(resource.Bytes)%int(resource.Stride) != 0 || resource.Width != 0 || resource.Height != 0 {
			return fmt.Errorf("invalid map vertex resource")
		}
	case MapResourceIndexBuffer:
		if resource.Stride != 4 || len(resource.Bytes) == 0 || len(resource.Bytes)%4 != 0 || resource.Width != 0 || resource.Height != 0 {
			return fmt.Errorf("invalid map index resource")
		}
	case MapResourceTextureRGBA, MapResourceTextureSDF, MapResourceTextureAlpha:
		bytesPerPixel := uint64(4)
		if resource.Type == MapResourceTextureSDF || resource.Type == MapResourceTextureAlpha {
			bytesPerPixel = 1
		}
		expected := uint64(resource.Width) * uint64(resource.Height) * bytesPerPixel
		if resource.Width == 0 || resource.Height == 0 || resource.Width > 4096 || resource.Height > 4096 || resource.Stride != uint32(bytesPerPixel) || expected != uint64(len(resource.Bytes)) {
			return fmt.Errorf("invalid map texture resource")
		}
	default:
		return fmt.Errorf("invalid map resource type")
	}
	return nil
}

func EncodeMapCamera(msg MapCamera) ([]byte, error) {
	var body bytes.Buffer
	writeMapCamera(&body, msg)
	return wrapEnvelope(MessageMapCamera, body.Bytes()), nil
}

func DecodeMapCamera(payload []byte) (MapCamera, error) {
	typeID, body, err := DecodeEnvelope(payload)
	if err != nil {
		return MapCamera{}, err
	}
	if typeID != MessageMapCamera {
		return MapCamera{}, fmt.Errorf("unexpected message type %d", typeID)
	}
	r := bytes.NewReader(body)
	camera, err := readMapCamera(r)
	if err != nil {
		return MapCamera{}, err
	}
	if r.Len() != 0 {
		return MapCamera{}, fmt.Errorf("map camera has trailing bytes")
	}
	return camera, nil
}

func writeMapCamera(body *bytes.Buffer, camera MapCamera) {
	writeFloat64(body, camera.Latitude)
	writeFloat64(body, camera.Longitude)
	writeFloat32(body, camera.Zoom)
	writeFloat32(body, camera.Bearing)
	writeFloat32(body, camera.Pitch)
	writeUint32(body, camera.ViewportWidth)
	writeUint32(body, camera.ViewportHeight)
}

func readMapCamera(r *bytes.Reader) (MapCamera, error) {
	var camera MapCamera
	var err error
	if camera.Latitude, err = readFloat64(r); err != nil {
		return MapCamera{}, err
	}
	if camera.Longitude, err = readFloat64(r); err != nil {
		return MapCamera{}, err
	}
	if camera.Zoom, err = readFloat32(r); err != nil {
		return MapCamera{}, err
	}
	if camera.Bearing, err = readFloat32(r); err != nil {
		return MapCamera{}, err
	}
	if camera.Pitch, err = readFloat32(r); err != nil {
		return MapCamera{}, err
	}
	if camera.ViewportWidth, err = readUint32(r); err != nil {
		return MapCamera{}, err
	}
	if camera.ViewportHeight, err = readUint32(r); err != nil {
		return MapCamera{}, err
	}
	return camera, nil
}
