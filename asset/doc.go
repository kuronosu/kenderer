// Package asset loads 3D models into the renderer's geometry and texture types.
// A loader returns a Model (mesh + base color + optional albedo map) rather than
// a shading.Material, so asset stays independent of the shading package; the
// caller assembles the material.
//
// LoadOBJ here parses Wavefront OBJ/MTL using only the standard library and the
// texture package (zero third-party dependencies). The glTF loader lives in the
// asset/gltf subpackage, which is where the sole third-party dependency
// (qmuntal/gltf) is confined.
package asset
