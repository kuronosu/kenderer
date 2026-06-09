package gltf

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"path/filepath"
	"testing"

	qgltf "github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"

	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/texture"
)

func vecClose(a, b math3d.Vec3, eps float64) bool {
	return math.Abs(a.X-b.X) < eps && math.Abs(a.Y-b.Y) < eps && math.Abs(a.Z-b.Z) < eps
}

// pngDataURI encodes a 1x1 image of c as a base64 PNG data URI.
func pngDataURI(t *testing.T, c color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestLoadGLTFRoundTrip(t *testing.T) {
	doc := qgltf.NewDocument()
	posIdx := modeler.WritePosition(doc, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})
	normIdx := modeler.WriteNormal(doc, [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}})
	uvIdx := modeler.WriteTextureCoord(doc, [][2]float32{{0, 0}, {1, 0}, {0, 1}})
	idxIdx := modeler.WriteIndices(doc, []uint32{0, 1, 2})

	doc.Images = append(doc.Images, &qgltf.Image{URI: pngDataURI(t, color.NRGBA{R: 255, A: 255})})
	doc.Textures = append(doc.Textures, &qgltf.Texture{Source: qgltf.Index(0)})
	doc.Materials = append(doc.Materials, &qgltf.Material{
		PBRMetallicRoughness: &qgltf.PBRMetallicRoughness{
			BaseColorFactor:  &[4]float64{0.5, 0.25, 0.75, 1},
			BaseColorTexture: &qgltf.TextureInfo{Index: 0},
		},
	})
	doc.Meshes = append(doc.Meshes, &qgltf.Mesh{Primitives: []*qgltf.Primitive{{
		Indices:  qgltf.Index(idxIdx),
		Material: qgltf.Index(0),
		Attributes: qgltf.PrimitiveAttributes{
			qgltf.POSITION:   posIdx,
			qgltf.NORMAL:     normIdx,
			qgltf.TEXCOORD_0: uvIdx,
		},
	}}})
	// A node with a pure translation, so the bake is observable in the positions.
	doc.Nodes = append(doc.Nodes, &qgltf.Node{Mesh: qgltf.Index(0), Translation: [3]float64{10, 0, 0}})
	doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, 0)

	path := filepath.Join(t.TempDir(), "model.glb")
	if err := qgltf.SaveBinary(doc, path); err != nil {
		t.Fatal(err)
	}

	models, err := LoadGLTF(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %d, want 1", len(models))
	}
	m := models[0]
	if got := len(m.Mesh.Vertices); got != 3 {
		t.Errorf("vertices = %d, want 3", got)
	}
	if got := len(m.Mesh.Indices); got != 3 {
		t.Errorf("indices = %d, want 3", got)
	}
	// Node translation (10,0,0) is baked into the positions: vertex 0 moves there.
	if got := m.Mesh.Vertices[0].Position; got != math3d.V3(10, 0, 0) {
		t.Errorf("baked position[0] = %v, want (10,0,0)", got)
	}
	// No COLOR_0 attribute → white default (so albedo*color is not zeroed).
	if got := m.Mesh.Vertices[0].Color; got != math3d.V3(1, 1, 1) {
		t.Errorf("vertex color = %v, want white (no COLOR_0)", got)
	}
	// baseColorFactor (linear) → BaseColor.
	if got := m.BaseColor; !vecClose(got, math3d.V3(0.5, 0.25, 0.75), 1e-6) {
		t.Errorf("BaseColor = %v, want (0.5,0.25,0.75)", got)
	}
	if m.AlbedoTex == nil {
		t.Fatal("AlbedoTex should be loaded from baseColorTexture")
	}
	// The embedded image is red sRGB → linear (1,0,0): proves decode + no R↔B swap.
	c := m.AlbedoTex.Sample(0.5, 0.5, texture.Nearest, texture.Clamp)
	if !vecClose(c, math3d.V3(1, 0, 0), 1e-9) {
		t.Errorf("sampled albedo = %v, want red (1,0,0)", c)
	}
}

func TestNodeMatrixColumnMajorTranspose(t *testing.T) {
	// glTF stores Node.Matrix column-major; a translation (1,2,3) occupies flat
	// indices 12..14. Loading it correctly puts the translation in the last column,
	// so the origin maps to (1,2,3). A transpose bug would scatter it into the last
	// row and leave the origin at (0,0,0).
	var cm [16]float64
	cm[0], cm[5], cm[10], cm[15] = 1, 1, 1, 1
	cm[12], cm[13], cm[14] = 1, 2, 3
	n := &qgltf.Node{Matrix: cm}

	got := nodeMatrix(n).MulVec4(math3d.V3(0, 0, 0).Vec4(1))
	if want := math3d.V4(1, 2, 3, 1); got != want {
		t.Errorf("nodeMatrix*origin = %v, want %v (translation in last column)", got, want)
	}
}
