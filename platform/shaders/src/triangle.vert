#version 450

// F4.1 bring-up shader: passes clip-space positions straight through. SDL_GPU
// normalizes NDC across drivers (+Y up, z in [0,1]), so no flip happens here.

layout(location = 0) in vec3 inPosition;

void main() {
    gl_Position = vec4(inPosition, 1.0);
}
