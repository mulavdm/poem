//go:build js && wasm

package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/mulavdm/poem/pkg/cartography"
	"github.com/mulavdm/poem/pkg/render/protocol"
	"golang.org/x/image/font/gofont/goregular"
)

type cameraInput struct {
	Latitude, Longitude, Zoom, Bearing, Pitch float64
	Width, Height                             uint32
}

func (camera cameraInput) value() cartography.Camera {
	return cartography.Camera{Latitude: camera.Latitude, Longitude: camera.Longitude, Zoom: camera.Zoom, Bearing: camera.Bearing, Pitch: camera.Pitch, Width: camera.Width, Height: camera.Height}
}

type tileInput struct {
	Z         uint8  `json:"z"`
	X, Y      uint32 `json:"x"`
	Bytes     string `json:"bytes"`
	Elevation string `json:"elevation"`
}

type featureInput struct {
	ID        string
	Geometry  uint8
	Role      uint8
	Positions []struct{ Latitude, Longitude float64 }
	Selected  bool
}

type buildInput struct {
	ViewportID       string                      `json:"viewport_id"`
	Generation       uint64                      `json:"generation"`
	Camera           cameraInput                 `json:"camera"`
	Tiles            []tileInput                 `json:"tiles"`
	Features         []featureInput              `json:"features"`
	Palette          cartography.SemanticPalette `json:"palette"`
	BuildingHeights  bool                        `json:"building_heights"`
	Locale           string                      `json:"locale"`
	POICategories    cartography.POICategories   `json:"poi_categories"`
	LabelDensity     float64                     `json:"label_density"`
	LabelMaximum     int                         `json:"label_maximum"`
	TerrainScale     float32                     `json:"terrain_scale"`
	TerrainDimension int                         `json:"terrain_dimension"`
	DynamicSun       bool                        `json:"dynamic_sun"`
	// UnixMillis is an injectable clock for deterministic solar lighting. When
	// zero and DynamicSun is set, the worker's own clock is used.
	UnixMillis int64 `json:"unix_millis"`
}

type resourceOutput struct {
	Type                  uint8  `json:"type"`
	Hash                  string `json:"hash"`
	Stride, Width, Height uint32 `json:"stride,omitempty"`
	Bytes                 string `json:"bytes"`
}

type drawOutput struct {
	VertexHash, IndexHash, TextureHash string  `json:"vertex_hash"`
	Primitive                          uint8   `json:"primitive"`
	First, Count                       uint32  `json:"first"`
	Layer                              int32   `json:"layer"`
	Opacity                            float32 `json:"opacity"`
	DepthTest                          bool    `json:"depth_test"`
}

type sceneOutput struct {
	Resources []resourceOutput `json:"resources"`
	Draws     []drawOutput     `json:"draws"`
	Picks     []pickOutput     `json:"picks,omitempty"`
}

type pickOutput struct {
	ID, Name, Description, Category, SourceLayer string
	Longitude, Latitude                          float64
}

var baseShaper, _ = cartography.NewTextShaper(goregular.TTF)

func main() {
	js.Global().Set("poemCartographyCover", js.FuncOf(cover))
	js.Global().Set("poemCartographyBuild", js.FuncOf(build))
	js.Global().Set("poemCartographyReady", true)
	select {}
}

func cover(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return errorJSON("invalid cover arguments")
	}
	var input struct {
		Camera   cameraInput `json:"camera"`
		Maximum  int         `json:"maximum"`
		TileZoom *int        `json:"tile_zoom"`
	}
	if err := json.Unmarshal([]byte(args[0].String()), &input); err != nil {
		return errorJSON(err.Error())
	}
	var tiles []cartography.TileID
	var err error
	if input.TileZoom != nil {
		tiles, err = cartography.CoverAtZoom(input.Camera.value(), *input.TileZoom, input.Maximum)
	} else {
		tiles, err = cartography.Cover(input.Camera.value(), input.Maximum)
	}
	if err != nil {
		return errorJSON(err.Error())
	}
	body, _ := json.Marshal(tiles)
	return string(body)
}

func build(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return errorJSON("invalid build arguments")
	}
	var input buildInput
	if err := json.Unmarshal([]byte(args[0].String()), &input); err != nil {
		return errorJSON(err.Error())
	}
	if input.Generation == 0 {
		return errorJSON("invalid generation")
	}
	tiles := make([]cartography.Tile, 0, len(input.Tiles))
	terrain := make([]cartography.TerrainTile, 0, len(input.Tiles))
	for _, encoded := range input.Tiles {
		body, err := base64.StdEncoding.DecodeString(encoded.Bytes)
		if err != nil {
			return errorJSON("invalid tile encoding")
		}
		layers, err := cartography.DecodeMVT(body)
		if err != nil {
			continue
		}
		tiles = append(tiles, cartography.Tile{ID: cartography.TileID{Z: encoded.Z, X: encoded.X, Y: encoded.Y}, Layers: layers})
		if encoded.Elevation != "" {
			elevationBytes, decodeErr := base64.StdEncoding.DecodeString(encoded.Elevation)
			if decodeErr == nil {
				if elevation, elevationErr := cartography.DecodeElevationTile(elevationBytes); elevationErr == nil {
					terrain = append(terrain, cartography.TerrainTile{ID: cartography.TileID{Z: encoded.Z, X: encoded.X, Y: encoded.Y}, Elevation: elevation})
				}
			}
		}
	}
	if len(tiles) == 0 {
		return errorJSON("no valid tiles")
	}
	camera := input.Camera.value()
	lighting := cartography.Lighting{DynamicSun: input.DynamicSun}
	if input.DynamicSun {
		if input.UnixMillis > 0 {
			lighting.Clock = time.UnixMilli(input.UnixMillis).UTC()
		} else {
			lighting.Clock = time.Now().UTC()
		}
	}
	scene, err := cartography.BuildSceneLit(input.ViewportID, input.Generation, camera, cartography.DefaultStyleWithOptions(input.Palette, cartography.StyleOptions{BuildingHeights: input.BuildingHeights, POICategories: input.POICategories}), tiles, lighting)
	if err == nil {
		scene, err = cartography.AddSceneFeatures(scene, wasmFeatures(input.Features, input.Palette))
	}
	if err == nil {
		candidates, candidateErr := cartography.BuildLabelCandidates(camera, cartography.DefaultLabelRulesFor(input.Locale, input.POICategories), tiles)
		if candidateErr == nil {
			candidates, candidateErr = cartography.LimitLabelCandidates(candidates, input.LabelDensity, input.LabelMaximum)
			if candidateErr == nil {
				placements, placementErr := cartography.PlaceLabels(context.Background(), camera, baseShaper, candidates)
				if placementErr == nil {
					scene, _ = cartography.AddLabelPlacements(scene, placements, baseShaper, input.Palette.Label, 900_000)
				}
			}
		}
	}
	if err == nil && len(terrain) > 0 {
		scene, err = cartography.DrapeSceneToTerrain(scene, terrain, input.TerrainScale)
	}
	if err == nil && len(terrain) > 0 {
		scene, err = cartography.AddTerrainMeshes(scene, terrain, cartography.TerrainMeshOptions{MaxDimension: input.TerrainDimension, VerticalScale: input.TerrainScale, SkirtDepth: 20, Color: input.Palette.Land})
	}
	if err != nil {
		return errorJSON(err.Error())
	}
	output := sceneOutput{Resources: make([]resourceOutput, 0, len(scene.Delta.Resources)), Draws: make([]drawOutput, 0, len(scene.Delta.Draws)), Picks: make([]pickOutput, 0, len(scene.Picks))}
	for _, resource := range scene.Delta.Resources {
		if resource.Operation != protocol.MapResourceUpload {
			continue
		}
		output.Resources = append(output.Resources, resourceOutput{Type: uint8(resource.Type), Hash: hex.EncodeToString(resource.Hash[:]), Stride: resource.Stride, Width: resource.Width, Height: resource.Height, Bytes: base64.StdEncoding.EncodeToString(resource.Bytes)})
	}
	for _, draw := range scene.Delta.Draws {
		output.Draws = append(output.Draws, drawOutput{VertexHash: hex.EncodeToString(draw.VertexHash[:]), IndexHash: hex.EncodeToString(draw.IndexHash[:]), TextureHash: nonzeroHash(draw.TextureHash), Primitive: uint8(draw.Primitive), First: draw.First, Count: draw.Count, Layer: draw.Layer, Opacity: draw.Opacity, DepthTest: draw.DepthTest})
	}
	for _, pick := range scene.Picks {
		if pick.SourceLayer != "poi" || pick.StableID == "" {
			continue
		}
		output.Picks = append(output.Picks, pickOutput{ID: pick.StableID, Name: pick.Name, Description: pick.Description, Category: pick.Category, SourceLayer: pick.SourceLayer, Longitude: pick.Longitude, Latitude: pick.Latitude})
	}
	body, marshalErr := json.Marshal(output)
	if marshalErr != nil {
		return errorJSON(marshalErr.Error())
	}
	return string(body)
}

func nonzeroHash(hash [32]byte) string {
	for _, value := range hash {
		if value != 0 {
			return hex.EncodeToString(hash[:])
		}
	}
	return ""
}

func errorJSON(message string) string {
	body, _ := json.Marshal(map[string]string{"error": message})
	return string(body)
}

func wasmFeatures(features []featureInput, palette cartography.SemanticPalette) []cartography.SceneFeature {
	result := make([]cartography.SceneFeature, 0, len(features))
	for _, feature := range features {
		if feature.ID == "" || len(feature.Positions) == 0 {
			continue
		}
		geometry := cartography.GeometryPoint
		if feature.Geometry == 1 {
			geometry = cartography.GeometryLine
		} else if feature.Geometry == 2 {
			geometry = cartography.GeometryPolygon
		}
		color, width, layer := palette.Route, float32(6), int32(1_000_100)
		if feature.Role == 2 || feature.Role == 3 {
			color, width, layer = palette.Marker, 12, 1_000_300
		}
		if feature.Role == 4 {
			color, width, layer = palette.Traffic, 8, 1_000_200
		}
		if feature.Selected {
			width *= 1.5
		}
		coordinates := make([][2]float64, len(feature.Positions))
		for index, position := range feature.Positions {
			coordinates[index] = [2]float64{position.Longitude, position.Latitude}
		}
		result = append(result, cartography.SceneFeature{ID: feature.ID, Geometry: geometry, Coordinates: coordinates, Color: color, Width: width, Layer: layer})
	}
	return result
}
