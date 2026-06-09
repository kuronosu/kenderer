// Package gltf loads glTF 2.0 and GLB assets into the renderer's geometry,
// texture and asset.Model types. It is the single place the third-party
// qmuntal/gltf dependency is used, keeping the rest of the module dependency-free.
//
// Scope (kenderer F3): mesh geometry plus base color (PBR baseColorFactor and
// baseColorTexture). Each mesh primitive becomes one asset.Model, and the node's
// accumulated world transform is baked into the vertex positions/normals, so the
// result is a flat world-space mesh. A live node hierarchy, animation and the
// full metallic-roughness model are deferred. glTF's conventions already match
// kenderer's (right-handed, +Y up, -Z forward; UV origin top-left; CCW front
// faces), so positions and UVs need no axis or V flip. Base color textures are
// sRGB and are linearized on load; the baseColorFactor is linear.
package gltf
