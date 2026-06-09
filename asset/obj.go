package asset

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/texture"
)

// LoadOBJ parses a Wavefront OBJ mesh from r. dir is the directory used to resolve
// files the OBJ references relatively: the .mtl named by mtllib and, in turn, any
// map_Kd texture.
//
// Polygons are triangulated as a fan; (v/vt/vn) triplets are de-duplicated into
// combined vertices; missing UVs default to (0,0); missing normals are computed
// from the surrounding faces (area-weighted, then normalized). UVs are flipped
// vertically (v' = 1 - v) because OBJ's texture origin is bottom-left while the
// sampler's is top-left. Vertex colors default to white so the shaded base
// (albedo * vertexColor) is not zeroed. Material support is single-material:
// BaseColor comes from Kd and AlbedoTex from map_Kd.
func LoadOBJ(r io.Reader, dir string) (*Model, error) {
	var (
		positions []math3d.Vec3
		texcoords []math3d.Vec2
		normals   []math3d.Vec3
	)
	mesh := &geometry.Mesh{}
	combined := map[[3]int]uint32{}
	var needNormal []bool // parallel to mesh.Vertices: true when vn was absent

	materials := map[string]*mtlMaterial{}
	usedMtl := ""

	// add returns the combined-vertex index for a (v,vt,vn) triplet, creating it on
	// first sight and reusing it afterwards.
	add := func(ref [3]int) (uint32, error) {
		if idx, ok := combined[ref]; ok {
			return idx, nil
		}
		vi, ti, ni := ref[0], ref[1], ref[2]
		if vi < 0 || vi >= len(positions) {
			return 0, fmt.Errorf("obj: vertex position index out of range")
		}
		v := geometry.Vertex{Position: positions[vi], Color: math3d.V3(1, 1, 1)}
		if ti >= 0 {
			if ti >= len(texcoords) {
				return 0, fmt.Errorf("obj: texcoord index out of range")
			}
			v.UV = texcoords[ti]
		}
		if ni >= 0 {
			if ni >= len(normals) {
				return 0, fmt.Errorf("obj: normal index out of range")
			}
			v.Normal = normals[ni]
		}
		idx := uint32(len(mesh.Vertices))
		mesh.Vertices = append(mesh.Vertices, v)
		needNormal = append(needNormal, ni < 0)
		combined[ref] = idx
		return idx, nil
	}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "v":
			p, err := parseVec3(fields[1:])
			if err != nil {
				return nil, err
			}
			positions = append(positions, p)
		case "vt":
			uv, err := parseUV(fields[1:])
			if err != nil {
				return nil, err
			}
			texcoords = append(texcoords, uv)
		case "vn":
			n, err := parseVec3(fields[1:])
			if err != nil {
				return nil, err
			}
			normals = append(normals, n)
		case "f":
			refs, err := parseFace(fields[1:], len(positions), len(texcoords), len(normals))
			if err != nil {
				return nil, err
			}
			// Fan triangulation: (refs[0], refs[i], refs[i+1]).
			for i := 1; i+1 < len(refs); i++ {
				ia, err := add(refs[0])
				if err != nil {
					return nil, err
				}
				ib, err := add(refs[i])
				if err != nil {
					return nil, err
				}
				ic, err := add(refs[i+1])
				if err != nil {
					return nil, err
				}
				mesh.Indices = append(mesh.Indices, ia, ib, ic)
				accumulateFaceNormal(mesh, needNormal, ia, ib, ic)
			}
		case "mtllib":
			for _, name := range fields[1:] {
				loaded, err := loadMTL(filepath.Join(dir, name), dir)
				if err != nil {
					return nil, err
				}
				for k, v := range loaded {
					materials[k] = v
				}
			}
		case "usemtl":
			if len(fields) > 1 && usedMtl == "" {
				usedMtl = fields[1]
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("obj: read: %w", err)
	}

	// Finalize computed normals: normalize the accumulated face normals.
	for i, need := range needNormal {
		if need {
			mesh.Vertices[i].Normal = mesh.Vertices[i].Normal.Normalize()
		}
	}

	model := &Model{Mesh: mesh, BaseColor: math3d.V3(1, 1, 1)}
	if m := pickMaterial(materials, usedMtl); m != nil {
		if m.hasColor {
			model.BaseColor = m.baseColor
		}
		model.AlbedoTex = m.albedo
	}
	return model, nil
}

// accumulateFaceNormal adds the (unnormalized, area-weighted) face normal to each
// of the triangle's vertices that still needs a computed normal.
func accumulateFaceNormal(mesh *geometry.Mesh, needNormal []bool, ia, ib, ic uint32) {
	if !needNormal[ia] && !needNormal[ib] && !needNormal[ic] {
		return
	}
	a := mesh.Vertices[ia].Position
	b := mesh.Vertices[ib].Position
	c := mesh.Vertices[ic].Position
	n := b.Sub(a).Cross(c.Sub(a))
	for _, idx := range [3]uint32{ia, ib, ic} {
		if needNormal[idx] {
			mesh.Vertices[idx].Normal = mesh.Vertices[idx].Normal.Add(n)
		}
	}
}

func parseVec3(f []string) (math3d.Vec3, error) {
	if len(f) < 3 {
		return math3d.Vec3{}, fmt.Errorf("obj: expected 3 floats, got %d", len(f))
	}
	x, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return math3d.Vec3{}, err
	}
	y, err := strconv.ParseFloat(f[1], 64)
	if err != nil {
		return math3d.Vec3{}, err
	}
	z, err := strconv.ParseFloat(f[2], 64)
	if err != nil {
		return math3d.Vec3{}, err
	}
	return math3d.V3(x, y, z), nil
}

// parseUV reads a vt line, taking the first two components and flipping V.
func parseUV(f []string) (math3d.Vec2, error) {
	if len(f) < 1 {
		return math3d.Vec2{}, fmt.Errorf("obj: vt needs at least 1 value")
	}
	u, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return math3d.Vec2{}, err
	}
	v := 0.0
	if len(f) >= 2 {
		v, err = strconv.ParseFloat(f[1], 64)
		if err != nil {
			return math3d.Vec2{}, err
		}
	}
	return math3d.V2(u, 1-v), nil // OBJ bottom-left origin → sampler top-left
}

// parseFace resolves each face token "v[/vt[/vn]]" to a 0-based (v,vt,vn) triplet,
// with -1 marking an absent attribute. nv/nt/nn are the current counts, used to
// resolve negative (relative) indices.
func parseFace(tokens []string, nv, nt, nn int) ([][3]int, error) {
	if len(tokens) < 3 {
		return nil, fmt.Errorf("obj: face needs at least 3 vertices, got %d", len(tokens))
	}
	refs := make([][3]int, len(tokens))
	for i, tok := range tokens {
		parts := strings.Split(tok, "/")
		ref := [3]int{-1, -1, -1}
		vi, err := resolveIndex(parts[0], nv)
		if err != nil {
			return nil, err
		}
		ref[0] = vi
		if len(parts) >= 2 && parts[1] != "" {
			ti, err := resolveIndex(parts[1], nt)
			if err != nil {
				return nil, err
			}
			ref[1] = ti
		}
		if len(parts) >= 3 && parts[2] != "" {
			ni, err := resolveIndex(parts[2], nn)
			if err != nil {
				return nil, err
			}
			ref[2] = ni
		}
		refs[i] = ref
	}
	return refs, nil
}

// resolveIndex converts a 1-based OBJ index (positive) or relative index
// (negative, -1 = last) to a 0-based index against a list of length count.
func resolveIndex(s string, count int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("obj: bad index %q: %w", s, err)
	}
	switch {
	case n > 0:
		return n - 1, nil
	case n < 0:
		return count + n, nil
	default:
		return 0, fmt.Errorf("obj: index 0 is invalid (indices are 1-based)")
	}
}

// mtlMaterial holds the subset of a .mtl material that the renderer uses.
type mtlMaterial struct {
	baseColor math3d.Vec3
	hasColor  bool
	albedo    *texture.Texture
}

// loadMTL parses a .mtl file, loading map_Kd textures relative to dir.
func loadMTL(path, dir string) (map[string]*mtlMaterial, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("obj: open mtl %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	materials := map[string]*mtlMaterial{}
	var cur *mtlMaterial
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "newmtl":
			if len(fields) < 2 {
				return nil, fmt.Errorf("obj: newmtl missing name")
			}
			cur = &mtlMaterial{baseColor: math3d.V3(1, 1, 1)}
			materials[fields[1]] = cur
		case "Kd":
			if cur == nil {
				continue
			}
			c, err := parseVec3(fields[1:])
			if err != nil {
				return nil, err
			}
			cur.baseColor = c
			cur.hasColor = true
		case "map_Kd":
			if cur == nil {
				continue
			}
			// The filename is the last token; any sampler options precede it.
			name := fields[len(fields)-1]
			tex, err := texture.LoadTextureFile(filepath.Join(dir, name), texture.KindColor)
			if err != nil {
				return nil, fmt.Errorf("obj: load map_Kd: %w", err)
			}
			cur.albedo = tex
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("obj: read mtl: %w", err)
	}
	return materials, nil
}

// pickMaterial chooses the model's single material: the one named by the first
// usemtl, or the sole material when the file defines exactly one.
func pickMaterial(materials map[string]*mtlMaterial, used string) *mtlMaterial {
	if used != "" {
		if m, ok := materials[used]; ok {
			return m
		}
	}
	if len(materials) == 1 {
		for _, m := range materials {
			return m
		}
	}
	return nil
}
