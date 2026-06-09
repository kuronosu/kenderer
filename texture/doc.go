// Package texture provides CPU textures sampled in linear RGB. A Texture stores
// linear-RGB texels with the origin at the top-left (row 0 is v = 0); the asset
// loaders normalize incoming UVs to that convention (OBJ flips V, glTF passes it
// through). Sampling supports nearest and bilinear filtering with repeat or clamp
// wrapping.
//
// Color (albedo) images are authored in sRGB and decoded to linear on load, so
// sampling composes correctly with the renderer's linear shading; data textures
// (KindData, e.g. future normal maps) are kept linear. The package depends only
// on the standard library (image, image/png, image/jpeg) and math3d.
package texture
