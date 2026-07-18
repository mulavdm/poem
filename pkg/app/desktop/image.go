package desktop

import (
	"bytes"
	"hash/fnv"
	"image"
	"image/draw"

	// Decoders for the formats ImageNode promises (image.Decode dispatch).
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// decodedImage is a decode-once cache entry for ImageNode bytes: the engine
// rebuilds the component tree every repaint, and decoding a PNG at 60fps
// would dwarf the frame budget. Keyed by FNV-1a of the encoded bytes, so a
// View returning the same slice (or equal content) every frame decodes once.
type decodedImage struct {
	width, height int
	pixels        []byte
}

var imageCache = map[uint64]decodedImage{}

func decodeImageCached(encoded []byte) (decodedImage, bool) {
	if len(encoded) == 0 {
		return decodedImage{}, false
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write(encoded)
	key := hasher.Sum64()
	if cached, ok := imageCache[key]; ok {
		return cached, true
	}
	source, _, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return decodedImage{}, false
	}
	boundsRect := source.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, boundsRect.Dx(), boundsRect.Dy()))
	draw.Draw(rgba, rgba.Bounds(), source, boundsRect.Min, draw.Src)
	entry := decodedImage{width: rgba.Bounds().Dx(), height: rgba.Bounds().Dy(), pixels: rgba.Pix}
	if len(imageCache) >= 16 {
		clear(imageCache) // tiny working set; drop-all beats bookkeeping
	}
	imageCache[key] = entry
	return entry, true
}
