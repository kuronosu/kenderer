package asset

import (
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/texture"
)

// Model is a loaded mesh together with the base material data the renderer needs.
// asset returns a Model instead of a shading.Material so it does not depend on the
// shading package; a caller builds shading.Material{Albedo: BaseColor, AlbedoTex:
// AlbedoTex}. BaseColor is linear RGB (default white); AlbedoTex is nil when the
// asset carries no albedo map.
type Model struct {
	Mesh      *geometry.Mesh
	BaseColor math3d.Vec3
	AlbedoTex *texture.Texture
}
