package texture

import (
	"fmt"
	"image"
	_ "image/jpeg" // register the JPEG decoder for image.Decode
	_ "image/png"  // register the PNG decoder for image.Decode
	"io"
	"math"
	"os"

	"github.com/kuronosu/kenderer/math3d"
)

// Filter selects how Sample reconstructs a color between texel centers.
type Filter int

const (
	Nearest  Filter = iota // pick the nearest texel
	Bilinear               // blend the four nearest texels
)

// Wrap selects how Sample treats texture coordinates outside [0, 1].
type Wrap int

const (
	Repeat Wrap = iota // tile: coordinates wrap modulo the size
	Clamp              // clamp to the edge texel
)

// Kind records the color space the source image was authored in. Color (albedo)
// data is sRGB and is linearized on load; Data (e.g. future normal/roughness
// maps) is already linear and stored unchanged.
type Kind int

const (
	KindColor Kind = iota // sRGB-encoded color; linearized on load
	KindData              // linear data; stored as-is
)

// Texture is a 2D image stored as linear-RGB texels, row-major with the origin at
// the top-left (row 0 is v = 0), matching image.Image and the UV convention the
// loaders normalize to. Sampling always returns linear RGB.
type Texture struct {
	Width, Height int
	Kind          Kind
	pix           []math3d.Vec3 // len == Width*Height, linear RGB
}

// newTexture returns a zeroed Texture of the given size.
func newTexture(width, height int, kind Kind) *Texture {
	return &Texture{Width: width, Height: height, Kind: kind, pix: make([]math3d.Vec3, width*height)}
}

// set writes the linear-RGB texel at (x, y); x and y must be in range.
func (t *Texture) set(x, y int, c math3d.Vec3) { t.pix[y*t.Width+x] = c }

// texel returns the texel at already-wrapped indices; ix and iy must be in range.
func (t *Texture) texel(ix, iy int) math3d.Vec3 { return t.pix[iy*t.Width+ix] }

// at returns the linear-RGB texel at integer coordinates (x, y), applying wrap so
// the indices are always valid.
func (t *Texture) at(x, y int, wrap Wrap) math3d.Vec3 {
	return t.texel(wrapIndex(x, t.Width, wrap), wrapIndex(y, t.Height, wrap))
}

// Texel returns the stored linear-RGB texel at integer coordinates (x, y),
// which must be in [0, Width) x [0, Height). It exposes the raw image to
// consumers that re-upload it elsewhere (the GPU backend re-encodes texels to
// sRGB bytes); filtered, wrapped lookups remain Sample's job.
func (t *Texture) Texel(x, y int) math3d.Vec3 { return t.texel(x, y) }

// Sample returns the linear-RGB color at (u, v) using the given filter and wrap.
// The origin is top-left: v = 0 is row 0. An empty texture returns black.
func (t *Texture) Sample(u, v float64, filter Filter, wrap Wrap) math3d.Vec3 {
	if t.Width == 0 || t.Height == 0 {
		return math3d.Vec3{}
	}
	if filter == Nearest {
		x := int(math.Floor(u * float64(t.Width)))
		y := int(math.Floor(v * float64(t.Height)))
		return t.at(x, y, wrap)
	}
	// Bilinear: texel centers sit at half-integer coordinates, so shift by -0.5.
	fx := u*float64(t.Width) - 0.5
	fy := v*float64(t.Height) - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	// Wrap the two distinct indices per axis once, not once per corner.
	ix0, ix1 := wrapIndex(x0, t.Width, wrap), wrapIndex(x0+1, t.Width, wrap)
	iy0, iy1 := wrapIndex(y0, t.Height, wrap), wrapIndex(y0+1, t.Height, wrap)
	top := t.texel(ix0, iy0).Lerp(t.texel(ix1, iy0), tx)
	bot := t.texel(ix0, iy1).Lerp(t.texel(ix1, iy1), tx)
	return top.Lerp(bot, ty)
}

// LoadTexture decodes a PNG or JPEG image from r into a Texture. When kind is
// KindColor the 8-bit sRGB samples are converted to linear; KindData keeps the
// normalized [0, 1] values unchanged.
func LoadTexture(r io.Reader, kind Kind) (*Texture, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("texture: decode: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	t := newTexture(w, h, kind)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// RGBA returns 16-bit, alpha-premultiplied components; albedo textures
			// are opaque so the premultiplication is a no-op here.
			r16, g16, b16, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			c := math3d.V3(float64(r16)/0xffff, float64(g16)/0xffff, float64(b16)/0xffff)
			if kind == KindColor {
				c = math3d.V3(srgbToLinear(c.X), srgbToLinear(c.Y), srgbToLinear(c.Z))
			}
			t.set(x, y, c)
		}
	}
	return t, nil
}

// LoadTextureFile decodes the image at path. See LoadTexture for color handling.
func LoadTextureFile(path string, kind Kind) (*Texture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("texture: open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return LoadTexture(f, kind)
}

// srgbToLinear converts an sRGB component in [0, 1] to linear. It is the inverse
// of the encoding shading.ToRGBA applies at output; texture keeps its own copy to
// stay free of any shading dependency.
func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// wrapIndex maps an integer texel index into [0, n) per the wrap mode.
func wrapIndex(i, n int, wrap Wrap) int {
	switch wrap {
	case Clamp:
		if i < 0 {
			return 0
		}
		if i >= n {
			return n - 1
		}
		return i
	default: // Repeat
		i %= n
		if i < 0 {
			i += n
		}
		return i
	}
}
