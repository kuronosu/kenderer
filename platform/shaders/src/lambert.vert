#version 450

// Mirror of the CPU vertex stage (pipeline.processVertex): clip-space position
// plus world-space varyings. Matrices arrive column-major (transposed from
// kenderer's row-major Mat4 at upload); mvp uses math3d.PerspectiveZO, the
// z[0,1] projection SDL_GPU expects. SDL_GPU normalizes NDC across drivers
// (+Y up like OpenGL, SDL applies the Vulkan Y-flip itself), so no flip here.
//
// SDL_GPU SPIR-V binding convention: vertex uniform buffers live in set 1;
// binding N is PushVertexUniformData slot N.

layout(set = 1, binding = 0) uniform VertexUniforms {
    mat4 mvp;
    mat4 model;
    mat4 normalMat; // inverse-transpose of model's upper-left 3x3, in a mat4
};

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inNormal;
layout(location = 2) in vec2 inUV;
layout(location = 3) in vec3 inColor;

layout(location = 0) out vec3 fragWorldPos;
layout(location = 1) out vec3 fragNormal;
layout(location = 2) out vec2 fragUV;
layout(location = 3) out vec3 fragColor;

void main() {
    vec4 pos = vec4(inPosition, 1.0);
    gl_Position = mvp * pos;
    fragWorldPos = (model * pos).xyz;
    fragNormal = mat3(normalMat) * inNormal;
    fragUV = inUV;
    fragColor = inColor;
}
