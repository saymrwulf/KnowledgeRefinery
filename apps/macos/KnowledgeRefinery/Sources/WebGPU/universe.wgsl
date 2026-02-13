// ============================================================
// universe.wgsl - WebGPU Shaders for Concept Universe Renderer
// ============================================================
// Contains vertex/fragment shaders for:
//   1. Node rendering (billboarded quads with circle mask + glow)
//   2. Edge rendering (lines with alpha blending)
// ============================================================

// ------------------------------------------------------------
// Shared uniform buffer: camera and scene parameters
// ------------------------------------------------------------
struct CameraUniforms {
    viewProjection: mat4x4<f32>,
    view:           mat4x4<f32>,
    projection:     mat4x4<f32>,
    cameraPosition: vec3<f32>,
    _pad0:          f32,
    focusPosition:  vec3<f32>,
    focusRadius:    f32,
    fogNear:        f32,
    fogFar:         f32,
    fogDensity:     f32,
    time:           f32,
    viewportWidth:  f32,
    viewportHeight: f32,
    lodLevel:       f32,       // 0=macro, 1=mid, 2=near
    _pad1:          f32,
};

@group(0) @binding(0) var<uniform> camera: CameraUniforms;

// ============================================================
// NODE SHADERS
// ============================================================

// Per-instance node data passed via vertex buffer
struct NodeInstance {
    @location(2) worldPos:  vec3<f32>,
    @location(3) size:      f32,
    @location(4) color:     vec4<f32>,
    @location(5) nodeLevel: f32,
    @location(6) focused:   f32,
    @location(7) hovered:   f32,
    @location(8) _extra:    f32,
};

// Quad vertex attributes
struct NodeVertex {
    @location(0) position: vec2<f32>,  // -1..1 quad corners
    @location(1) uv:       vec2<f32>,  // 0..1 UV coords
};

struct NodeVSOutput {
    @builtin(position) clipPos: vec4<f32>,
    @location(0)       uv:      vec2<f32>,
    @location(1)       color:   vec4<f32>,
    @location(2)       fogFactor: f32,
    @location(3)       focused:   f32,
    @location(4)       hovered:   f32,
    @location(5)       distToCamera: f32,
};

@vertex
fn nodeVertex(
    vert: NodeVertex,
    inst: NodeInstance,
) -> NodeVSOutput {
    var out: NodeVSOutput;

    // Distance from camera to node
    let toCamera = camera.cameraPosition - inst.worldPos;
    let dist = length(toCamera);
    out.distToCamera = dist;

    // Billboard: extract camera right and up from view matrix
    let right = vec3<f32>(camera.view[0][0], camera.view[1][0], camera.view[2][0]);
    let up    = vec3<f32>(camera.view[0][1], camera.view[1][1], camera.view[2][1]);

    // Scale: base size * instance size, with slight distance compensation
    let baseScale = inst.size * 0.5;
    // Minimum screen-size so far nodes don't vanish entirely
    let screenScale = max(baseScale, baseScale * 0.15 * dist / 10.0);
    let scale = baseScale;

    // Offset the quad corners in world space (billboarded)
    let worldPosition = inst.worldPos
        + right * (vert.position.x * scale)
        + up    * (vert.position.y * scale);

    out.clipPos = camera.viewProjection * vec4<f32>(worldPosition, 1.0);
    out.uv = vert.uv;
    out.color = inst.color;
    out.focused = inst.focused;
    out.hovered = inst.hovered;

    // Fog factor: linear fog from fogNear to fogFar
    let fogRange = camera.fogFar - camera.fogNear;
    let fogLinear = clamp((dist - camera.fogNear) / fogRange, 0.0, 1.0);
    out.fogFactor = fogLinear;

    return out;
}

@fragment
fn nodeFragment(in: NodeVSOutput) -> @location(0) vec4<f32> {
    // Circle SDF from UV center
    let center = vec2<f32>(0.5, 0.5);
    let diff = in.uv - center;
    let dist = length(diff);

    // Smooth circle mask (anti-aliased edge)
    let radius = 0.45;
    let edgeSoftness = 0.05;
    let circleMask = 1.0 - smoothstep(radius - edgeSoftness, radius + edgeSoftness, dist);

    if (circleMask < 0.01) {
        discard;
    }

    // Base color
    var color = in.color.rgb;

    // Inner highlight (slight 3D look)
    let highlightOffset = vec2<f32>(-0.12, -0.15);
    let highlightDist = length(diff - highlightOffset);
    let highlight = 1.0 - smoothstep(0.0, 0.35, highlightDist);
    color = mix(color, vec3<f32>(1.0, 1.0, 1.0), highlight * 0.3);

    // Glow effect for focused nodes
    let glowIntensity = in.focused;
    if (glowIntensity > 0.0) {
        let glowRadius = 0.5;
        let glowDist = length(diff);
        let glow = exp(-glowDist * glowDist * 8.0) * glowIntensity;
        // Pulsing glow
        let pulse = 0.8 + 0.2 * sin(camera.time * 3.0);
        color = color + in.color.rgb * glow * pulse * 0.6;
    }

    // Hover highlight: bright ring
    if (in.hovered > 0.0) {
        let ringDist = abs(dist - 0.38);
        let ring = 1.0 - smoothstep(0.0, 0.06, ringDist);
        color = mix(color, vec3<f32>(1.0, 1.0, 1.0), ring * 0.9 * in.hovered);
        // Also brighten the whole node slightly
        color = color * (1.0 + 0.2 * in.hovered);
    }

    // Focus+context: darken and desaturate distant nodes
    let focusDist = length(vec3<f32>(0.0) - vec3<f32>(0.0)); // placeholder
    // Use fog factor for focus+context
    let contextDim = mix(1.0, 0.35, in.fogFactor * 0.7);
    color = color * contextDim;

    // Apply fog (blend towards dark background)
    let fogColor = vec3<f32>(0.05, 0.05, 0.1);
    color = mix(color, fogColor, in.fogFactor * 0.8);

    // Final alpha: circle mask with slight edge glow for focused
    var alpha = circleMask * in.color.a;
    if (glowIntensity > 0.0) {
        let outerGlow = 1.0 - smoothstep(radius, 0.5, dist);
        alpha = max(alpha, outerGlow * 0.3 * glowIntensity);
    }

    return vec4<f32>(color, alpha);
}


// ============================================================
// EDGE SHADERS
// ============================================================

struct EdgeVertex {
    @location(0) position: vec3<f32>,
    @location(1) color:    vec4<f32>,
};

struct EdgeVSOutput {
    @builtin(position) clipPos: vec4<f32>,
    @location(0)       color:   vec4<f32>,
    @location(1)       fogFactor: f32,
};

@vertex
fn edgeVertex(vert: EdgeVertex) -> EdgeVSOutput {
    var out: EdgeVSOutput;

    out.clipPos = camera.viewProjection * vec4<f32>(vert.position, 1.0);
    out.color = vert.color;

    // Fog
    let dist = length(camera.cameraPosition - vert.position);
    let fogRange = camera.fogFar - camera.fogNear;
    let fogLinear = clamp((dist - camera.fogNear) / fogRange, 0.0, 1.0);
    out.fogFactor = fogLinear;

    return out;
}

@fragment
fn edgeFragment(in: EdgeVSOutput) -> @location(0) vec4<f32> {
    var color = in.color.rgb;

    // Apply fog
    let fogColor = vec3<f32>(0.05, 0.05, 0.1);
    color = mix(color, fogColor, in.fogFactor * 0.85);

    // Reduce alpha with fog as well
    let alpha = in.color.a * (1.0 - in.fogFactor * 0.7);

    return vec4<f32>(color, alpha);
}
