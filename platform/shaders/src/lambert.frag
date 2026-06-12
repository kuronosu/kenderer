#version 450

// Mirror of the CPU fragment stage (shading.Lambert.Shade), all in linear RGB:
//
//   base = albedoTex(uv) * albedoFactor * vertexColor
//   out  = base * ambient + base * lightColor * intensity * max(0, dot(N, L))
//
// The texture is sRGB-format, so the sampler decodes to linear (the CPU path
// decodes on load) and bilinear filtering happens in linear space, like
// texture.Sample. The render target / swapchain is sRGB too, so the hardware
// applies the one output encode (the CPU path's shading.ToRGBA). Untextured
// objects bind a 1x1 white texture and carry the material albedo in
// albedoFactor; textured ones bind their map with a white factor — exactly the
// CPU semantics, where Material.Albedo is ignored when a map is present.
//
// SDL_GPU SPIR-V binding convention: fragment samplers live in set 2 and
// fragment uniform buffers in set 3; binding N is the bind/push slot N.

layout(set = 2, binding = 0) uniform sampler2D albedoTex;

layout(set = 3, binding = 0) uniform FragmentUniforms {
    vec4 lightDir;     // xyz: direction the light travels, world space
    vec4 lightColor;   // rgb: light color, a: intensity
    vec4 albedoFactor; // rgb: constant albedo (white when textured)
    vec4 params;       // x: ambient, y: smooth flag (>0.5 keeps vertex normals)
};

layout(location = 0) in vec3 fragWorldPos;
layout(location = 1) in vec3 fragNormal;
layout(location = 2) in vec2 fragUV;
layout(location = 3) in vec3 fragColor;

layout(location = 0) out vec4 outColor;

void main() {
    vec3 n;
    if (params.y > 0.5) {
        // Smooth: interpolation denormalizes the vertex normals, so normalize
        // per fragment (= CPU Phong path).
        n = normalize(fragNormal);
    } else {
        // Flat: geometric face normal from the world-position screen
        // derivatives, the GPU equivalent of the CPU's per-face normal. With
        // SDL_GPU's top-left viewport (+y down), dFdy x dFdx points toward the
        // viewer for front (CCW) faces; the CPU parity test pins this sign.
        n = normalize(cross(dFdy(fragWorldPos), dFdx(fragWorldPos)));
    }
    vec3 toLight = -normalize(lightDir.xyz);
    float lambert = max(dot(n, toLight), 0.0);
    vec3 base = texture(albedoTex, fragUV).rgb * albedoFactor.rgb * fragColor;
    vec3 lit = base * params.x + base * lightColor.rgb * (lightColor.a * lambert);
    outColor = vec4(lit, 1.0);
}
