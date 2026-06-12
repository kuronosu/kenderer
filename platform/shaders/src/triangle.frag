#version 450

// F4.1 bring-up shader: constant color, no lighting.

layout(location = 0) out vec4 outColor;

void main() {
    outColor = vec4(0.9, 0.5, 0.2, 1.0);
}
