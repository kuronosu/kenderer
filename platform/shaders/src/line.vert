#version 450

// Line pass: world-space endpoints with a per-vertex color, transformed by
// viewProj only (endpoints arrive pre-transformed to world space, mirroring
// pipeline.drawSegment). Same z[0,1] projection notes as lambert.vert.

layout(set = 1, binding = 0) uniform LineUniforms {
    mat4 viewProj;
};

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inColor;

layout(location = 0) out vec3 fragColor;

void main() {
    gl_Position = viewProj * vec4(inPosition, 1.0);
    fragColor = inColor;
}
