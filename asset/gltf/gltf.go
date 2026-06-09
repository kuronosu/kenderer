package gltf

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	qgltf "github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"

	"github.com/kuronosu/kenderer/asset"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/texture"
)

// LoadGLTF loads a glTF or GLB file at path and returns one asset.Model per mesh
// primitive. It traverses the default scene, accumulating each node's world
// transform and baking it into the primitive's positions and normals (a flat
// world-space mesh; see the package doc for the deferred scope).
func LoadGLTF(path string) ([]*asset.Model, error) {
	doc, err := qgltf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gltf: open %q: %w", path, err)
	}
	dir := filepath.Dir(path)
	texCache := map[int]*texture.Texture{} // by image index, decoded once

	var models []*asset.Model
	for _, ni := range sceneRoots(doc) {
		if err := walkNode(doc, ni, math3d.Identity(), dir, texCache, &models); err != nil {
			return nil, err
		}
	}
	return models, nil
}

// sceneRoots returns the root node indices of the document's default scene, or
// every node when the document defines no scenes.
func sceneRoots(doc *qgltf.Document) []int {
	si := 0
	if doc.Scene != nil {
		si = *doc.Scene
	}
	if si >= 0 && si < len(doc.Scenes) {
		return doc.Scenes[si].Nodes
	}
	all := make([]int, len(doc.Nodes))
	for i := range all {
		all[i] = i
	}
	return all
}

// walkNode recurses the node hierarchy, accumulating the world transform and
// emitting a Model for every primitive of any mesh it encounters.
func walkNode(doc *qgltf.Document, ni int, parent math3d.Mat4, dir string, texCache map[int]*texture.Texture, out *[]*asset.Model) error {
	if ni < 0 || ni >= len(doc.Nodes) {
		return fmt.Errorf("gltf: node index %d out of range", ni)
	}
	n := doc.Nodes[ni]
	world := parent.Mul(nodeMatrix(n))

	if n.Mesh != nil {
		mi := *n.Mesh
		if mi < 0 || mi >= len(doc.Meshes) {
			return fmt.Errorf("gltf: mesh index %d out of range", mi)
		}
		normalMat := math3d.NormalMatrix(world)
		for _, prim := range doc.Meshes[mi].Primitives {
			model, err := loadPrimitive(doc, prim, world, normalMat, dir, texCache)
			if err != nil {
				return err
			}
			*out = append(*out, model)
		}
	}

	for _, child := range n.Children {
		if err := walkNode(doc, child, world, dir, texCache, out); err != nil {
			return err
		}
	}
	return nil
}

// emptyMatrix is the all-zero value Node.Matrix can hold for an in-memory node;
// qmuntal's UnmarshalJSON instead fills an absent matrix with gltf.DefaultMatrix
// (identity). Either means "no explicit matrix", so nodeMatrix falls back to TRS.
var emptyMatrix [16]float64

// nodeMatrix returns the node's local transform. glTF nodes carry either a matrix
// or a translation/rotation/scale triple (never both); the matrix is used only
// when it is an explicit, non-identity transform.
func nodeMatrix(n *qgltf.Node) math3d.Mat4 {
	if n.Matrix != emptyMatrix && n.Matrix != qgltf.DefaultMatrix {
		return fromColumnMajor(n.Matrix)
	}
	t := n.TranslationOrDefault()
	r := n.RotationOrDefault() // (x, y, z, w)
	s := n.ScaleOrDefault()
	translate := math3d.Translate(math3d.V3(t[0], t[1], t[2]))
	rotate := math3d.Quat{X: r[0], Y: r[1], Z: r[2], W: r[3]}.Mat4()
	scale := math3d.Scale(math3d.V3(s[0], s[1], s[2]))
	return translate.Mul(rotate).Mul(scale)
}

// fromColumnMajor converts a glTF column-major 4x4 (stored as a flat [16]) into a
// row-major math3d.Mat4: element (row r, col c) lives at flat index c*4 + r, so
// the load transposes the natural row-major copy.
func fromColumnMajor(m [16]float64) math3d.Mat4 {
	var out math3d.Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			out[r][c] = m[c*4+r]
		}
	}
	return out
}

// vec3f widens a glTF/modeler float32 triple (a POSITION or NORMAL element) to a
// math3d.Vec3.
func vec3f(p [3]float32) math3d.Vec3 { return math3d.V3(float64(p[0]), float64(p[1]), float64(p[2])) }

// loadPrimitive reads one mesh primitive into a Model, baking world into the
// positions and normals and resolving the base material.
func loadPrimitive(doc *qgltf.Document, prim *qgltf.Primitive, world math3d.Mat4, normalMat math3d.Mat3, dir string, texCache map[int]*texture.Texture) (*asset.Model, error) {
	posIdx, ok := prim.Attributes[qgltf.POSITION]
	if !ok {
		return nil, fmt.Errorf("gltf: primitive without POSITION attribute")
	}
	positions, err := modeler.ReadPosition(doc, doc.Accessors[posIdx], nil)
	if err != nil {
		return nil, fmt.Errorf("gltf: read positions: %w", err)
	}

	var normals [][3]float32
	if ni, ok := prim.Attributes[qgltf.NORMAL]; ok {
		if normals, err = modeler.ReadNormal(doc, doc.Accessors[ni], nil); err != nil {
			return nil, fmt.Errorf("gltf: read normals: %w", err)
		}
	}
	var uvs [][2]float32
	if ti, ok := prim.Attributes[qgltf.TEXCOORD_0]; ok {
		if uvs, err = modeler.ReadTextureCoord(doc, doc.Accessors[ti], nil); err != nil {
			return nil, fmt.Errorf("gltf: read texcoords: %w", err)
		}
	}
	var colors [][4]uint8
	if ci, ok := prim.Attributes[qgltf.COLOR_0]; ok {
		if colors, err = modeler.ReadColor(doc, doc.Accessors[ci], nil); err != nil {
			return nil, fmt.Errorf("gltf: read colors: %w", err)
		}
	}

	mesh := &geometry.Mesh{Vertices: make([]geometry.Vertex, len(positions))}
	for i, p := range positions {
		// Bake the world transform: positions through the matrix, normals through
		// its inverse-transpose. The mesh is therefore already in world space.
		wp := world.MulVec4(vec3f(p).Vec4(1)).XYZ()
		v := geometry.Vertex{Position: wp, Color: math3d.V3(1, 1, 1)}
		if i < len(normals) {
			v.Normal = normalMat.MulVec3(vec3f(normals[i]))
		}
		if i < len(uvs) {
			v.UV = math3d.V2(float64(uvs[i][0]), float64(uvs[i][1])) // glTF UV origin is already top-left
		}
		if i < len(colors) {
			// COLOR_0 is linear vertex color; normalized bytes /255, no sRGB decode.
			v.Color = math3d.V3(float64(colors[i][0])/255, float64(colors[i][1])/255, float64(colors[i][2])/255)
		}
		mesh.Vertices[i] = v
	}

	if prim.Indices != nil {
		idx, err := modeler.ReadIndices(doc, doc.Accessors[*prim.Indices], nil)
		if err != nil {
			return nil, fmt.Errorf("gltf: read indices: %w", err)
		}
		mesh.Indices = idx
	} else {
		mesh.Indices = make([]uint32, len(positions))
		for i := range mesh.Indices {
			mesh.Indices[i] = uint32(i)
		}
	}

	model := &asset.Model{Mesh: mesh, BaseColor: math3d.V3(1, 1, 1)}
	if prim.Material != nil {
		mi := *prim.Material
		if mi < 0 || mi >= len(doc.Materials) {
			return nil, fmt.Errorf("gltf: material index %d out of range", mi)
		}
		if err := applyMaterial(doc, doc.Materials[mi], dir, texCache, model); err != nil {
			return nil, err
		}
	}
	return model, nil
}

// applyMaterial copies the base color factor and texture into the Model.
func applyMaterial(doc *qgltf.Document, mat *qgltf.Material, dir string, texCache map[int]*texture.Texture, model *asset.Model) error {
	pbr := mat.PBRMetallicRoughness
	if pbr == nil {
		return nil
	}
	f := pbr.BaseColorFactorOrDefault() // linear, default [1,1,1,1]
	model.BaseColor = math3d.V3(f[0], f[1], f[2])
	if pbr.BaseColorTexture != nil {
		tex, err := loadTexture(doc, pbr.BaseColorTexture.Index, dir, texCache)
		if err != nil {
			return err
		}
		model.AlbedoTex = tex
	}
	return nil
}

// loadTexture resolves a glTF texture index to a decoded, linearized Texture,
// caching by underlying image index so a shared image is decoded only once.
func loadTexture(doc *qgltf.Document, texIdx int, dir string, cache map[int]*texture.Texture) (*texture.Texture, error) {
	if texIdx < 0 || texIdx >= len(doc.Textures) {
		return nil, fmt.Errorf("gltf: texture index %d out of range", texIdx)
	}
	src := doc.Textures[texIdx].Source
	if src == nil {
		return nil, fmt.Errorf("gltf: texture %d has no image source", texIdx)
	}
	imgIdx := *src
	if tex, ok := cache[imgIdx]; ok {
		return tex, nil
	}
	if imgIdx < 0 || imgIdx >= len(doc.Images) {
		return nil, fmt.Errorf("gltf: image index %d out of range", imgIdx)
	}
	data, err := imageBytes(doc, doc.Images[imgIdx], dir)
	if err != nil {
		return nil, err
	}
	tex, err := texture.LoadTexture(bytes.NewReader(data), texture.KindColor)
	if err != nil {
		return nil, fmt.Errorf("gltf: decode image %d: %w", imgIdx, err)
	}
	cache[imgIdx] = tex
	return tex, nil
}

// imageBytes returns the encoded image bytes whether the image lives in a buffer
// view (typical of GLB), is embedded as a data URI, or is an external file
// relative to dir.
func imageBytes(doc *qgltf.Document, img *qgltf.Image, dir string) ([]byte, error) {
	switch {
	case img.BufferView != nil:
		data, err := modeler.ReadBufferView(doc, doc.BufferViews[*img.BufferView])
		if err != nil {
			return nil, fmt.Errorf("gltf: read image buffer view: %w", err)
		}
		return data, nil
	case img.IsEmbeddedResource():
		data, err := img.MarshalData()
		if err != nil {
			return nil, fmt.Errorf("gltf: decode embedded image: %w", err)
		}
		return data, nil
	case img.URI != "":
		name := img.URI
		if dec, err := url.PathUnescape(name); err == nil {
			name = dec
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("gltf: read external image %q: %w", img.URI, err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("gltf: image has no data source")
	}
}
