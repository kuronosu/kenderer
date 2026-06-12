#version 450

// Line pass: unlit, constant per-segment color in linear RGB (the GPU analog
// of raster.DrawLine). The sRGB target applies the one output encode, so the
// saturated axis colors land as exact 255s, like shading.ToRGBA on the CPU.

layout(location = 0) in vec3 fragColor;

layout(location = 0) out vec4 outColor;

void main() {
    outColor = vec4(fragColor, 1.0);
}
