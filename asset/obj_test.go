package asset

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/texture"
)

const quadOBJ = `
# a unit quad with UVs and a shared normal
v 0 0 0
v 1 0 0
v 1 1 0
v 0 1 0
vt 0 0
vt 1 0
vt 1 1
vt 0 1
vn 0 0 1
f 1/1/1 2/2/1 3/3/1 4/4/1
`

func TestLoadOBJQuadCountsAndAttributes(t *testing.T) {
	m, err := LoadOBJ(strings.NewReader(quadOBJ), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(m.Mesh.Vertices); got != 4 {
		t.Errorf("vertices = %d, want 4 (quad de-duplicated)", got)
	}
	if got := len(m.Mesh.Indices); got != 6 {
		t.Errorf("indices = %d, want 6 (quad → 2 triangles)", got)
	}
	if got := m.Mesh.NumTriangles(); got != 2 {
		t.Errorf("triangles = %d, want 2", got)
	}

	v0 := m.Mesh.Vertices[0]
	if v0.Position != math3d.V3(0, 0, 0) {
		t.Errorf("v0 position = %v, want (0,0,0)", v0.Position)
	}
	// vt (0,0) flips to (0,1): OBJ bottom-left origin → sampler top-left.
	if v0.UV != math3d.V2(0, 1) {
		t.Errorf("v0 UV = %v, want (0,1) (V flipped)", v0.UV)
	}
	if v0.Normal != math3d.V3(0, 0, 1) {
		t.Errorf("v0 normal = %v, want (0,0,1)", v0.Normal)
	}
	if v0.Color != math3d.V3(1, 1, 1) {
		t.Errorf("v0 color = %v, want white (1,1,1)", v0.Color)
	}

	// Vertex 2 comes from 3/3/1: position (1,1,0), vt (1,1) → flipped (1,0).
	v2 := m.Mesh.Vertices[2]
	if v2.Position != math3d.V3(1, 1, 0) || v2.UV != math3d.V2(1, 0) {
		t.Errorf("v2 = pos %v uv %v, want pos (1,1,0) uv (1,0)", v2.Position, v2.UV)
	}

	if m.BaseColor != math3d.V3(1, 1, 1) {
		t.Errorf("BaseColor = %v, want white (no mtl)", m.BaseColor)
	}
	if m.AlbedoTex != nil {
		t.Error("AlbedoTex should be nil without a material")
	}
}

func TestLoadOBJSharesVerticesAndComputesNormals(t *testing.T) {
	// Two triangles sharing the diagonal (1,3); no vt/vn provided.
	const obj = `
v 0 0 0
v 1 0 0
v 1 1 0
v 0 1 0
f 1 2 3
f 1 3 4
`
	m, err := LoadOBJ(strings.NewReader(obj), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(m.Mesh.Vertices); got != 4 {
		t.Errorf("vertices = %d, want 4 (shared 1 and 3 de-duplicated)", got)
	}
	if got := len(m.Mesh.Indices); got != 6 {
		t.Errorf("indices = %d, want 6", got)
	}
	// Both faces lie in z=0 wound CCW, so every computed normal is +Z.
	for i, v := range m.Mesh.Vertices {
		if d := v.Normal.Sub(math3d.V3(0, 0, 1)); d.Length() > 1e-9 {
			t.Errorf("vertex %d computed normal = %v, want (0,0,1)", i, v.Normal)
		}
		if v.Color != math3d.V3(1, 1, 1) {
			t.Errorf("vertex %d color = %v, want white", i, v.Color)
		}
	}
}

func TestLoadOBJFaceWithoutUV(t *testing.T) {
	// v//vn form: explicit normal, no texcoord.
	const obj = `
v 0 0 0
v 1 0 0
v 0 1 0
vn 0 0 1
f 1//1 2//1 3//1
`
	m, err := LoadOBJ(strings.NewReader(obj), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(m.Mesh.Vertices); got != 3 {
		t.Errorf("vertices = %d, want 3", got)
	}
	for i, v := range m.Mesh.Vertices {
		if v.UV != (math3d.Vec2{}) {
			t.Errorf("vertex %d UV = %v, want (0,0) default", i, v.UV)
		}
		if v.Normal != math3d.V3(0, 0, 1) {
			t.Errorf("vertex %d normal = %v, want explicit (0,0,1)", i, v.Normal)
		}
	}
}

func TestLoadOBJMaterial(t *testing.T) {
	dir := t.TempDir()

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 255, G: 128, B: 0, A: 255})
	writePNG(t, filepath.Join(dir, "tex.png"), img)

	const mtl = "newmtl mat\nKd 0.2 0.4 0.6\nmap_Kd tex.png\n"
	if err := os.WriteFile(filepath.Join(dir, "m.mtl"), []byte(mtl), 0o644); err != nil {
		t.Fatal(err)
	}
	const obj = "mtllib m.mtl\nusemtl mat\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

	m, err := LoadOBJ(strings.NewReader(obj), dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.BaseColor != math3d.V3(0.2, 0.4, 0.6) {
		t.Errorf("BaseColor = %v, want (0.2,0.4,0.6) from Kd", m.BaseColor)
	}
	if m.AlbedoTex == nil {
		t.Fatal("AlbedoTex should be loaded from map_Kd")
	}
	// sRGB (255,128,0) → linear: R=1, B=0, proving no R↔B swap and correct decode.
	c := m.AlbedoTex.Sample(0.5, 0.5, texture.Nearest, texture.Clamp)
	if math.Abs(c.X-1) > 1e-9 || math.Abs(c.Z-0) > 1e-9 {
		t.Errorf("sampled albedo = %v, want R≈1 B≈0", c)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
