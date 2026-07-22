package cartography

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/mulavdm/poem/pkg/render/protocol"
	"golang.org/x/image/vector"
)

const (
	labelVertexStride = 48
	maxGlyphAtlasSide = 2048
	glyphAtlasPadding = 2
)

type GlyphKey struct {
	ID       uint32
	Size64th uint16
	Weight   LabelWeight
}

type GlyphAtlasEntry struct {
	X, Y, Width, Height uint16
	MinX, MaxY          float32
}

type GlyphAtlas struct {
	Width, Height uint32
	Alpha         []byte
	Entries       map[GlyphKey]GlyphAtlasEntry
}

type rasterGlyph struct {
	key        GlyphKey
	alpha      *image.Alpha
	minX, maxY float32
}

// BuildGlyphAtlas rasterizes the exact shaped glyph IDs used by placements.
// Packing and pixels are deterministic for identical font and placement data.
func BuildGlyphAtlas(placements []LabelPlacement, shaper *TextShaper) (GlyphAtlas, error) {
	if shaper == nil || shaper.face == nil {
		return GlyphAtlas{}, errors.New("cartography: invalid glyph atlas shaper")
	}
	keys := make(map[GlyphKey]struct{})
	for _, placement := range placements {
		size64 := math.Round(placement.Candidate.Size * 64)
		if size64 < 384 || size64 > math.MaxUint16 {
			return GlyphAtlas{}, errors.New("cartography: invalid glyph atlas size")
		}
		for _, glyph := range placement.Shaped.Glyphs {
			keys[GlyphKey{ID: glyph.ID, Size64th: uint16(size64), Weight: placement.Candidate.Weight}] = struct{}{}
		}
	}
	if len(keys) == 0 || len(keys) > 8192 {
		return GlyphAtlas{}, errors.New("cartography: invalid glyph atlas glyph count")
	}
	ordered := make([]GlyphKey, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Size64th != ordered[j].Size64th {
			return ordered[i].Size64th < ordered[j].Size64th
		}
		if ordered[i].Weight != ordered[j].Weight {
			return ordered[i].Weight < ordered[j].Weight
		}
		return ordered[i].ID < ordered[j].ID
	})
	glyphs := make([]rasterGlyph, 0, len(ordered))
	for _, key := range ordered {
		glyph, err := rasterizeGlyph(shaper, key)
		if err != nil {
			return GlyphAtlas{}, err
		}
		glyphs = append(glyphs, glyph)
	}
	entries := make(map[GlyphKey]GlyphAtlasEntry, len(glyphs))
	x, y, rowHeight, usedWidth := 0, 0, 0, 0
	for _, glyph := range glyphs {
		width, height := glyph.alpha.Bounds().Dx(), glyph.alpha.Bounds().Dy()
		if width > maxGlyphAtlasSide || height > maxGlyphAtlasSide {
			return GlyphAtlas{}, errors.New("cartography: glyph exceeds atlas bounds")
		}
		if x+width > maxGlyphAtlasSide {
			x, y, rowHeight = 0, y+rowHeight, 0
		}
		if y+height > maxGlyphAtlasSide {
			return GlyphAtlas{}, errors.New("cartography: glyph atlas is full")
		}
		entries[glyph.key] = GlyphAtlasEntry{X: uint16(x), Y: uint16(y), Width: uint16(width), Height: uint16(height), MinX: glyph.minX, MaxY: glyph.maxY}
		x += width
		usedWidth = max(usedWidth, x)
		rowHeight = max(rowHeight, height)
	}
	usedHeight := y + rowHeight
	width, height := nextPowerOfTwo(max(1, usedWidth)), nextPowerOfTwo(max(1, usedHeight))
	atlas := image.NewAlpha(image.Rect(0, 0, width, height))
	for _, glyph := range glyphs {
		entry := entries[glyph.key]
		draw.Draw(atlas, image.Rect(int(entry.X), int(entry.Y), int(entry.X)+int(entry.Width), int(entry.Y)+int(entry.Height)), glyph.alpha, image.Point{}, draw.Src)
	}
	return GlyphAtlas{Width: uint32(width), Height: uint32(height), Alpha: append([]byte(nil), atlas.Pix...), Entries: entries}, nil
}

func rasterizeGlyph(shaper *TextShaper, key GlyphKey) (rasterGlyph, error) {
	shaper.mu.Lock()
	data := shaper.face.GlyphData(font.GID(key.ID))
	upem := shaper.face.Upem()
	shaper.mu.Unlock()
	var outline font.GlyphOutline
	switch value := data.(type) {
	case font.GlyphOutline:
		outline = value
	case font.GlyphSVG:
		outline = value.Outline
	default:
		return rasterGlyph{}, errors.New("cartography: glyph has no supported outline")
	}
	if upem == 0 || len(outline.Segments) > 1<<16 {
		return rasterGlyph{}, errors.New("cartography: invalid glyph outline")
	}
	if len(outline.Segments) == 0 {
		return rasterGlyph{key: key, alpha: image.NewAlpha(image.Rect(0, 0, 1, 1))}, nil
	}
	scale := (float32(key.Size64th) / 64) / float32(upem)
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -minX, -minY
	for _, segment := range outline.Segments {
		for _, point := range segment.ArgsSlice() {
			x, y := point.X*scale, point.Y*scale
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
	}
	if !finite(float64(minX)) || !finite(float64(minY)) || !finite(float64(maxX)) || !finite(float64(maxY)) || maxX < minX || maxY < minY {
		return rasterGlyph{}, errors.New("cartography: invalid glyph outline bounds")
	}
	width := int(math.Ceil(float64(maxX-minX))) + 2*glyphAtlasPadding
	height := int(math.Ceil(float64(maxY-minY))) + 2*glyphAtlasPadding
	if width <= 0 || height <= 0 || width > maxGlyphAtlasSide || height > maxGlyphAtlasSide {
		return rasterGlyph{}, errors.New("cartography: glyph raster exceeds bounds")
	}
	rasterizer := vector.NewRasterizer(width, height)
	transform := func(point font.SegmentPoint) (float32, float32) {
		return point.X*scale - minX + glyphAtlasPadding, maxY - point.Y*scale + glyphAtlasPadding
	}
	for _, segment := range outline.Segments {
		switch segment.Op {
		case opentype.SegmentOpMoveTo:
			x, y := transform(segment.Args[0])
			rasterizer.MoveTo(x, y)
		case opentype.SegmentOpLineTo:
			x, y := transform(segment.Args[0])
			rasterizer.LineTo(x, y)
		case opentype.SegmentOpQuadTo:
			x1, y1 := transform(segment.Args[0])
			x2, y2 := transform(segment.Args[1])
			rasterizer.QuadTo(x1, y1, x2, y2)
		case opentype.SegmentOpCubeTo:
			x1, y1 := transform(segment.Args[0])
			x2, y2 := transform(segment.Args[1])
			x3, y3 := transform(segment.Args[2])
			rasterizer.CubeTo(x1, y1, x2, y2, x3, y3)
		default:
			return rasterGlyph{}, errors.New("cartography: invalid glyph segment")
		}
	}
	alpha := image.NewAlpha(image.Rect(0, 0, width, height))
	rasterizer.Draw(alpha, alpha.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	if key.Weight != LabelWeightRegular {
		alpha = emboldenGlyph(alpha, key.Weight)
		minX--
		maxY++
	}
	return rasterGlyph{key: key, alpha: alpha, minX: minX, maxY: maxY}, nil
}

func emboldenGlyph(source *image.Alpha, weight LabelWeight) *image.Alpha {
	bounds := source.Bounds()
	result := image.NewAlpha(image.Rect(0, 0, bounds.Dx()+2, bounds.Dy()+2))
	for y := 0; y < result.Bounds().Dy(); y++ {
		for x := 0; x < result.Bounds().Dx(); x++ {
			var coverage uint8
			for dy := -1; dy <= 1; dy++ {
				if weight == LabelWeightMedium && dy != 0 {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					sx, sy := x-1+dx, y-1+dy
					if sx >= 0 && sx < bounds.Dx() && sy >= 0 && sy < bounds.Dy() {
						coverage = max(coverage, source.AlphaAt(sx, sy).A)
					}
				}
			}
			result.SetAlpha(x, y, color.Alpha{A: coverage})
		}
	}
	return result
}

// AddLabelPlacements appends one alpha atlas and textured glyph quads to a
// retained scene without changing the protocol's resource layout.
func AddLabelPlacements(scene Scene, placements []LabelPlacement, shaper *TextShaper, labelColor Color, layer int32) (Scene, error) {
	if len(placements) == 0 {
		return scene, nil
	}
	atlas, err := BuildGlyphAtlas(placements, shaper)
	if err != nil {
		return Scene{}, err
	}
	var vertices, indices []byte
	var vertexCount, indexCount uint32
	for _, placement := range placements {
		penX := -placement.Shaped.Advance / 2
		baselineY := (placement.Shaped.Ascent + placement.Shaped.Descent) / 2
		keySize := uint16(math.Round(placement.Candidate.Size * 64))
		for _, glyph := range placement.Shaped.Glyphs {
			entry, ok := atlas.Entries[GlyphKey{ID: glyph.ID, Size64th: keySize, Weight: placement.Candidate.Weight}]
			if !ok {
				return Scene{}, errors.New("cartography: shaped glyph missing from atlas")
			}
			x1 := penX + glyph.XOffset + entry.MinX - glyphAtlasPadding
			y1 := baselineY - glyph.YOffset - entry.MaxY - glyphAtlasPadding
			x2, y2 := x1+float32(entry.Width), y1+float32(entry.Height)
			u1, v1 := float32(entry.X)/float32(atlas.Width), float32(entry.Y)/float32(atlas.Height)
			u2, v2 := float32(uint32(entry.X)+uint32(entry.Width))/float32(atlas.Width), float32(uint32(entry.Y)+uint32(entry.Height))/float32(atlas.Height)
			for _, vertex := range [4][4]float32{{u1, v1, x1, y1}, {u2, v1, x2, y1}, {u1, v2, x1, y2}, {u2, v2, x2, y2}} {
				appendLabelVertex(&vertices, placement.Candidate.WorldX, placement.Candidate.WorldY, labelColor, vertex[0], vertex[1], vertex[2], vertex[3])
			}
			for _, index := range [...]uint32{0, 1, 2, 2, 1, 3} {
				appendIndex(&indices, vertexCount+index)
			}
			vertexCount += 4
			indexCount += 6
			penX += glyph.XAdvance
		}
	}
	if vertexCount > maxSceneVertices || indexCount > maxSceneIndices {
		return Scene{}, errors.New("cartography: label scene exceeds geometry bounds")
	}
	vertexHash, indexHash := sha256.Sum256(vertices), sha256.Sum256(indices)
	textureDigest := sha256.New()
	var textureHeader [8]byte
	binary.LittleEndian.PutUint32(textureHeader[0:4], atlas.Width)
	binary.LittleEndian.PutUint32(textureHeader[4:8], atlas.Height)
	textureDigest.Write(textureHeader[:])
	textureDigest.Write(atlas.Alpha)
	var textureHash [32]byte
	copy(textureHash[:], textureDigest.Sum(nil))
	scene.Delta.Resources = append(scene.Delta.Resources,
		protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceVertexBuffer, Hash: vertexHash, Stride: labelVertexStride, Bytes: vertices},
		protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceIndexBuffer, Hash: indexHash, Stride: 4, Bytes: indices},
		protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceTextureAlpha, Hash: textureHash, Width: atlas.Width, Height: atlas.Height, Stride: 1, Bytes: atlas.Alpha},
	)
	scene.Delta.Draws = append(scene.Delta.Draws, protocol.MapDrawBatch{VertexHash: vertexHash, IndexHash: indexHash, TextureHash: textureHash, Primitive: protocol.MapPrimitiveTriangles, Count: indexCount, Layer: layer, Opacity: 1})
	sort.SliceStable(scene.Delta.Draws, func(i, j int) bool { return scene.Delta.Draws[i].Layer < scene.Delta.Draws[j].Layer })
	return scene, nil
}

func appendLabelVertex(dst *[]byte, worldX, worldY float64, labelColor Color, u, v, offsetX, offsetY float32) {
	var data [labelVertexStride]byte
	binary.LittleEndian.PutUint32(data[0:4], math.Float32bits(float32(worldX)))
	binary.LittleEndian.PutUint32(data[4:8], math.Float32bits(float32(worldY)))
	data[12], data[13], data[14], data[15] = labelColor.R, labelColor.G, labelColor.B, labelColor.A
	binary.LittleEndian.PutUint32(data[28:32], math.Float32bits(u))
	binary.LittleEndian.PutUint32(data[32:36], math.Float32bits(v))
	binary.LittleEndian.PutUint32(data[36:40], math.Float32bits(offsetX))
	binary.LittleEndian.PutUint32(data[40:44], math.Float32bits(offsetY))
	*dst = append(*dst, data[:]...)
}

func nextPowerOfTwo(value int) int {
	result := 1
	for result < value {
		result <<= 1
	}
	return result
}
