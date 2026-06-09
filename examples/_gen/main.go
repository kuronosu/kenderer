// Command gen produces the sample assets under examples/ and an offscreen preview
// of each, loaded back through the real loaders. It is a dev-only tool: the
// leading-underscore directory keeps it out of `go build/test/vet ./...`, so the
// qmuntal/gltf writer used here is not part of the normal build.
//
// Run from the repo root:
//
//	go run examples/_gen/main.go
//
// It writes examples/torus.glb (texture embedded), examples/sphere.obj +
// sphere.mtl + sphere_uv.png, and preview_*.png renders for a quick eyeball.
package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	qgltf "github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"

	"github.com/kuronosu/kenderer/asset"
	assetgltf "github.com/kuronosu/kenderer/asset/gltf"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/pipeline"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
	"github.com/kuronosu/kenderer/texture"
)

const dir = "examples"

func main() {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}

	// --- Torus (.glb, texture embedded) -------------------------------------
	tp, tn, tuv, ti := torusMesh(1.0, 0.4, 64, 28)
	donutTex := uvChecker(512, 512, 16, color.NRGBA{R: 225, G: 150, B: 70}) // warm
	glbPath := filepath.Join(dir, "torus.glb")
	if err := writeGLB(glbPath, "donut", tp, tn, tuv, ti, donutTex, [3]float64{1, 1, 1}); err != nil {
		log.Fatal(err)
	}

	// --- Sphere (.obj + .mtl + .png) ----------------------------------------
	sp, sn, suv, si := sphereMesh(1.0, 48, 24)
	globeTex := uvChecker(512, 512, 12, color.NRGBA{R: 70, G: 180, B: 210}) // cool
	if err := os.WriteFile(filepath.Join(dir, "sphere_uv.png"), globeTex, 0o644); err != nil {
		log.Fatal(err)
	}
	if err := writeOBJ(filepath.Join(dir, "sphere.obj"), filepath.Join(dir, "sphere.mtl"),
		"sphere.mtl", "sphere_uv.png", sp, sn, suv, si, [3]float64{1, 1, 1}); err != nil {
		log.Fatal(err)
	}

	// --- Load both back through the real loaders and render previews --------
	bg := color.RGBA{R: 18, G: 18, B: 24, A: 255}

	torus, err := assetgltf.LoadGLTF(glbPath)
	if err != nil {
		log.Fatal(err)
	}
	report("torus.glb", torus)
	if err := renderPreview(torus, filepath.Join(dir, "preview_torus.png"), bg); err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(filepath.Join(dir, "sphere.obj"))
	if err != nil {
		log.Fatal(err)
	}
	sphere, err := asset.LoadOBJ(f, dir)
	_ = f.Close()
	if err != nil {
		log.Fatal(err)
	}
	report("sphere.obj", []*asset.Model{sphere})
	if err := renderPreview([]*asset.Model{sphere}, filepath.Join(dir, "preview_sphere.png"), bg); err != nil {
		log.Fatal(err)
	}

	fmt.Println("done: examples/torus.glb, examples/sphere.obj (+.mtl/.png), preview_*.png")
}

func report(name string, models []*asset.Model) {
	var verts, tris int
	for _, m := range models {
		verts += len(m.Mesh.Vertices)
		tris += m.Mesh.NumTriangles()
	}
	lo, hi := models[0].Mesh.Bounds()
	fmt.Printf("%-12s models=%d verts=%d tris=%d bounds=%v..%v tex=%v\n",
		name, len(models), verts, tris, lo, hi, models[0].AlbedoTex != nil)
}

// torusMesh builds a torus of major radius R and minor radius r, with su segments
// around the ring and sv around the tube. The hole axis is +Y.
func torusMesh(R, r float64, su, sv int) (pos, norm [][3]float32, uv [][2]float32, idx []uint32) {
	for i := 0; i <= su; i++ {
		u := float64(i) / float64(su)
		th := 2 * math.Pi * u
		ct, st := math.Cos(th), math.Sin(th)
		for j := 0; j <= sv; j++ {
			v := float64(j) / float64(sv)
			ph := 2 * math.Pi * v
			cp, sp := math.Cos(ph), math.Sin(ph)
			pos = append(pos, [3]float32{float32((R + r*cp) * ct), float32(r * sp), float32((R + r*cp) * st)})
			norm = append(norm, [3]float32{float32(cp * ct), float32(sp), float32(cp * st)})
			uv = append(uv, [2]float32{float32(u), float32(v)})
		}
	}
	return pos, norm, uv, grid(pos, norm, su, sv)
}

// sphereMesh builds a UV sphere of the given radius with seg longitude segments
// and ring latitude segments. v = 0 is the north pole (top, +Y).
func sphereMesh(rad float64, seg, ring int) (pos, norm [][3]float32, uv [][2]float32, idx []uint32) {
	for i := 0; i <= seg; i++ {
		u := float64(i) / float64(seg)
		th := 2 * math.Pi * u
		ct, st := math.Cos(th), math.Sin(th)
		for j := 0; j <= ring; j++ {
			v := float64(j) / float64(ring)
			ph := math.Pi * v
			sp, cp := math.Sin(ph), math.Cos(ph)
			pos = append(pos, [3]float32{float32(rad * sp * ct), float32(rad * cp), float32(rad * sp * st)})
			norm = append(norm, [3]float32{float32(sp * ct), float32(cp), float32(sp * st)})
			uv = append(uv, [2]float32{float32(u), float32(v)})
		}
	}
	return pos, norm, uv, grid(pos, norm, seg, ring)
}

// grid triangulates an (su+1)x(sv+1) vertex grid, orienting each triangle so its
// geometric normal agrees with the vertex normals (outward, CCW front).
func grid(pos, norm [][3]float32, su, sv int) []uint32 {
	stride := sv + 1
	var idx []uint32
	for i := 0; i < su; i++ {
		for j := 0; j < sv; j++ {
			a := uint32(i*stride + j)
			b := uint32((i+1)*stride + j)
			c := uint32((i+1)*stride + j + 1)
			d := uint32(i*stride + j + 1)
			idx = append(idx, orient(pos, norm, a, b, c)...)
			idx = append(idx, orient(pos, norm, a, c, d)...)
		}
	}
	return idx
}

func orient(pos, norm [][3]float32, a, b, c uint32) []uint32 {
	g := v3(pos[b]).Sub(v3(pos[a])).Cross(v3(pos[c]).Sub(v3(pos[a])))
	avg := v3(norm[a]).Add(v3(norm[b])).Add(v3(norm[c]))
	if g.Dot(avg) < 0 {
		return []uint32{a, c, b}
	}
	return []uint32{a, b, c}
}

func v3(p [3]float32) math3d.Vec3 { return math3d.V3(float64(p[0]), float64(p[1]), float64(p[2])) }

// uvChecker returns a PNG: a checkerboard whose brightness marks UV cells, tinted
// red with u and green with v so the texture orientation is unmistakable.
func uvChecker(w, h, cells int, tint color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		v := float64(y) / float64(h-1)
		for x := 0; x < w; x++ {
			u := float64(x) / float64(w-1)
			s := 1.0
			if (x*cells/w+y*cells/h)&1 == 1 {
				s = 0.45
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: clamp8(s * (35 + u*float64(tint.R))),
				G: clamp8(s * (35 + v*float64(tint.G))),
				B: clamp8(s * float64(tint.B)),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func clamp8(f float64) uint8 {
	switch {
	case f <= 0:
		return 0
	case f >= 255:
		return 255
	default:
		return uint8(f + 0.5)
	}
}

func writeGLB(path, matName string, pos, norm [][3]float32, uv [][2]float32, idx []uint32, pngBytes []byte, base [3]float64) error {
	doc := qgltf.NewDocument()
	p := modeler.WritePosition(doc, pos)
	n := modeler.WriteNormal(doc, norm)
	t := modeler.WriteTextureCoord(doc, uv)
	i := modeler.WriteIndices(doc, idx)

	doc.Images = append(doc.Images, &qgltf.Image{
		Name: matName,
		URI:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes),
	})
	doc.Textures = append(doc.Textures, &qgltf.Texture{Source: qgltf.Index(0)})
	doc.Materials = append(doc.Materials, &qgltf.Material{
		Name: matName,
		PBRMetallicRoughness: &qgltf.PBRMetallicRoughness{
			BaseColorFactor:  &[4]float64{base[0], base[1], base[2], 1},
			BaseColorTexture: &qgltf.TextureInfo{Index: 0},
		},
	})
	doc.Meshes = append(doc.Meshes, &qgltf.Mesh{Name: matName, Primitives: []*qgltf.Primitive{{
		Indices:  qgltf.Index(i),
		Material: qgltf.Index(0),
		Attributes: qgltf.PrimitiveAttributes{
			qgltf.POSITION:   p,
			qgltf.NORMAL:     n,
			qgltf.TEXCOORD_0: t,
		},
	}}})
	doc.Nodes = append(doc.Nodes, &qgltf.Node{Name: matName, Mesh: qgltf.Index(0)})
	doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, 0)
	return qgltf.SaveBinary(doc, path)
}

func writeOBJ(objPath, mtlPath, mtlName, texName string, pos, norm [][3]float32, uv [][2]float32, idx []uint32, base [3]float64) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# kenderer sample: UV sphere\nmtllib %s\nusemtl sphere\n", mtlName)
	for _, p := range pos {
		fmt.Fprintf(&b, "v %g %g %g\n", p[0], p[1], p[2])
	}
	for _, t := range uv {
		// Store V flipped: OBJ origin is bottom-left, so LoadOBJ's 1-v recovers the
		// top-left v this mesh was authored with.
		fmt.Fprintf(&b, "vt %g %g\n", t[0], 1-t[1])
	}
	for _, n := range norm {
		fmt.Fprintf(&b, "vn %g %g %g\n", n[0], n[1], n[2])
	}
	for k := 0; k+2 < len(idx); k += 3 {
		a, c, d := idx[k]+1, idx[k+1]+1, idx[k+2]+1
		fmt.Fprintf(&b, "f %d/%d/%d %d/%d/%d %d/%d/%d\n", a, a, a, c, c, c, d, d, d)
	}
	if err := os.WriteFile(objPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	mtl := fmt.Sprintf("newmtl sphere\nKd %g %g %g\nmap_Kd %s\n", base[0], base[1], base[2], texName)
	return os.WriteFile(mtlPath, []byte(mtl), 0o644)
}

// renderPreview renders the models from a fixed three-quarter view (smooth-shaded,
// textured) to a PNG, exercising the real load → scene → pipeline path.
func renderPreview(models []*asset.Model, outPath string, bg color.RGBA) error {
	objects := make([]scene.Object, 0, len(models))
	for _, m := range models {
		objects = append(objects, scene.Object{
			Mesh:      m.Mesh,
			Transform: scene.Transform{Rotation: math3d.QuatIdentity(), Scale: math3d.V3(1, 1, 1)},
			Material:  shading.Material{Albedo: m.BaseColor, AlbedoTex: m.AlbedoTex, Filter: texture.Bilinear, Wrap: texture.Repeat},
			Smooth:    true,
		})
	}

	const w, h = 720, 540
	fovy := 45.0 * math.Pi / 180
	lo, hi := scene.Bounds(objects)
	center := lo.Add(hi).Scale(0.5)
	radius := hi.Sub(lo).Length() * 0.5
	dist := radius / math.Sin(fovy/2) * 1.25
	eye := center.Add(math3d.V3(0.55, 0.45, 1).Normalize().Scale(dist))

	scn := scene.Scene{
		Camera:  scene.Camera{Eye: eye, Target: center, Up: math3d.V3(0, 1, 0), FOVY: fovy, Near: math.Max(1e-3, radius*0.01), Far: dist + radius*4},
		Objects: objects,
		Light:   shading.DirectionalLight{Direction: math3d.V3(-0.5, -0.8, -0.6).Normalize(), Color: math3d.V3(1, 1, 1), Intensity: 1},
		Ambient: 0.2,
	}
	img := pipeline.NewRenderer(pipeline.Options{Width: w, Height: h, Cull: pipeline.CullBack, Background: bg}).Render(scn).Image()

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
