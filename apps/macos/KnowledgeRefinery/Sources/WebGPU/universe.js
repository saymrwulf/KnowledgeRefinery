// ============================================================
// universe.js - WebGPU 3D Concept Universe Renderer
// ============================================================
// Self-contained renderer for a force-directed concept graph
// with LOD, orbit camera, and interaction support.
// ============================================================

"use strict";

// ------------------------------------------------------------
// Constants
// ------------------------------------------------------------
const LOD_MACRO = 0;
const LOD_MID   = 1;
const LOD_NEAR  = 2;

const REPULSION_STRENGTH  = 500.0;
const ATTRACTION_STRENGTH = 0.005;
const CENTERING_STRENGTH  = 0.002;
const DAMPING             = 0.92;
const MIN_DISTANCE        = 0.5;
const SIMULATION_STEPS    = 1; // steps per frame
const INITIAL_LAYOUT_STEPS = 200; // steps on data load

// Camera defaults
const DEFAULT_ORBIT_DISTANCE = 80.0;
const MIN_ORBIT_DISTANCE     = 5.0;
const MAX_ORBIT_DISTANCE     = 500.0;
const FOG_NEAR_RATIO         = 0.3;
const FOG_FAR_RATIO          = 3.0;

// LOD thresholds (camera distance)
const LOD_MACRO_THRESHOLD = 150.0;
const LOD_MID_THRESHOLD   = 60.0;

// ------------------------------------------------------------
// Globals
// ------------------------------------------------------------
let canvas, device, context, canvasFormat;
let uniformBuffer, uniformBindGroup, uniformBindGroupLayout;
let nodePipeline, edgePipeline;
let nodeQuadVB, nodeQuadIB;
let nodeInstanceBuffer, nodeInstanceData;
let edgeVertexBuffer, edgeVertexData;
let depthTexture, depthTextureView;
let shaderModule;

// Canvas2D fallback mode
let use2DFallback = false;
let ctx2d = null;

// Data
let universeData = null;
let nodes = [];
let edges = [];
let visibleNodes = [];
let visibleEdges = [];

// Simulation
let velocities = [];
let simulationRunning = false;
let simulationCooling = 1.0;

// Camera state
let camera = {
    orbitTheta:    0.5,    // horizontal angle
    orbitPhi:      0.6,    // vertical angle (0=top, pi=bottom)
    orbitDistance:  DEFAULT_ORBIT_DISTANCE,
    target:        [0, 0, 0],
    position:      [0, 0, 0],
    // Smooth transitions
    targetOrbitTheta:   0.5,
    targetOrbitPhi:     0.6,
    targetOrbitDistance: DEFAULT_ORBIT_DISTANCE,
    targetTarget:       [0, 0, 0],
    smoothSpeed:        0.08,
};

// Interaction state
let mouse = {
    x: 0, y: 0,
    lastX: 0, lastY: 0,
    leftDown: false,
    rightDown: false,
    hoveredNodeIndex: -1,
};

// Focus
let focusPosition = [0, 0, 0];
let focusRadius = 50.0;

// Performance
let fps = 0;
let frameCount = 0;
let lastFpsTime = performance.now();

// UI elements
let fpsDisplay, nodeCountDisplay, lodDisplay, tooltipEl;

// WGSL shader source (loaded from file or inlined)
let wgslSource = null;


// ============================================================
// Math Utilities
// ============================================================

function vec3Add(a, b) { return [a[0]+b[0], a[1]+b[1], a[2]+b[2]]; }
function vec3Sub(a, b) { return [a[0]-b[0], a[1]-b[1], a[2]-b[2]]; }
function vec3Scale(a, s) { return [a[0]*s, a[1]*s, a[2]*s]; }
function vec3Dot(a, b) { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]; }
function vec3Cross(a, b) {
    return [
        a[1]*b[2] - a[2]*b[1],
        a[2]*b[0] - a[0]*b[2],
        a[0]*b[1] - a[1]*b[0],
    ];
}
function vec3Length(a) { return Math.sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2]); }
function vec3Normalize(a) {
    const l = vec3Length(a);
    return l > 1e-8 ? [a[0]/l, a[1]/l, a[2]/l] : [0, 0, 0];
}
function vec3Lerp(a, b, t) {
    return [a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t, a[2]+(b[2]-a[2])*t];
}

function mat4Identity() {
    return new Float32Array([
        1,0,0,0, 0,1,0,0, 0,0,1,0, 0,0,0,1
    ]);
}

function mat4Multiply(a, b) {
    const out = new Float32Array(16);
    for (let i = 0; i < 4; i++) {
        for (let j = 0; j < 4; j++) {
            out[j*4+i] = 0;
            for (let k = 0; k < 4; k++) {
                out[j*4+i] += a[k*4+i] * b[j*4+k];
            }
        }
    }
    return out;
}

function mat4Perspective(fovY, aspect, near, far) {
    const f = 1.0 / Math.tan(fovY * 0.5);
    const rangeInv = 1.0 / (near - far);
    return new Float32Array([
        f/aspect, 0, 0, 0,
        0, f, 0, 0,
        0, 0, far*rangeInv, -1,
        0, 0, near*far*rangeInv, 0,
    ]);
}

function mat4LookAt(eye, target, up) {
    const zAxis = vec3Normalize(vec3Sub(eye, target));
    const xAxis = vec3Normalize(vec3Cross(up, zAxis));
    const yAxis = vec3Cross(zAxis, xAxis);
    return new Float32Array([
        xAxis[0], yAxis[0], zAxis[0], 0,
        xAxis[1], yAxis[1], zAxis[1], 0,
        xAxis[2], yAxis[2], zAxis[2], 0,
        -vec3Dot(xAxis, eye), -vec3Dot(yAxis, eye), -vec3Dot(zAxis, eye), 1,
    ]);
}


// ============================================================
// HSL to RGB
// ============================================================

function hslToRgb(h, s, l) {
    h = h % 360;
    if (h < 0) h += 360;
    s = Math.max(0, Math.min(1, s));
    l = Math.max(0, Math.min(1, l));
    const c = (1 - Math.abs(2*l - 1)) * s;
    const x = c * (1 - Math.abs((h/60) % 2 - 1));
    const m = l - c/2;
    let r, g, b;
    if (h < 60)       { r=c; g=x; b=0; }
    else if (h < 120) { r=x; g=c; b=0; }
    else if (h < 180) { r=0; g=c; b=x; }
    else if (h < 240) { r=0; g=x; b=c; }
    else if (h < 300) { r=x; g=0; b=c; }
    else              { r=c; g=0; b=x; }
    return [r+m, g+m, b+m];
}

function clusterColor(cluster, totalClusters) {
    const hue = (cluster / Math.max(totalClusters, 1)) * 360;
    return hslToRgb(hue, 0.75, 0.55);
}


// ============================================================
// Force-Directed Layout (Velocity Verlet)
// ============================================================

function initSimulation() {
    velocities = nodes.map(() => [0, 0, 0]);
    simulationCooling = 1.0;
}

function simulationStep(dt) {
    const n = nodes.length;
    if (n === 0) return;

    const forces = new Array(n);
    for (let i = 0; i < n; i++) {
        forces[i] = [0, 0, 0];
    }

    // Repulsion (all pairs, O(n^2) for <1000 nodes)
    const repStr = REPULSION_STRENGTH * simulationCooling;
    for (let i = 0; i < n; i++) {
        for (let j = i + 1; j < n; j++) {
            let dx = nodes[i].x - nodes[j].x;
            let dy = nodes[i].y - nodes[j].y;
            let dz = nodes[i].z - nodes[j].z;
            let dist2 = dx*dx + dy*dy + dz*dz;
            if (dist2 < MIN_DISTANCE * MIN_DISTANCE) {
                // Add small random jitter to prevent overlap
                dx += (Math.random() - 0.5) * 0.1;
                dy += (Math.random() - 0.5) * 0.1;
                dz += (Math.random() - 0.5) * 0.1;
                dist2 = dx*dx + dy*dy + dz*dz;
            }
            const dist = Math.sqrt(dist2);
            const force = repStr / dist2;
            const fx = (dx / dist) * force;
            const fy = (dy / dist) * force;
            const fz = (dz / dist) * force;
            forces[i][0] += fx;
            forces[i][1] += fy;
            forces[i][2] += fz;
            forces[j][0] -= fx;
            forces[j][1] -= fy;
            forces[j][2] -= fz;
        }
    }

    // Attraction along edges
    const attStr = ATTRACTION_STRENGTH * simulationCooling;
    for (const edge of edges) {
        const si = edge.sourceIdx;
        const ti = edge.targetIdx;
        if (si < 0 || ti < 0) continue;
        const dx = nodes[ti].x - nodes[si].x;
        const dy = nodes[ti].y - nodes[si].y;
        const dz = nodes[ti].z - nodes[si].z;
        const dist = Math.sqrt(dx*dx + dy*dy + dz*dz);
        if (dist < 1e-6) continue;
        const w = edge.weight || 1.0;
        const force = dist * attStr * w;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;
        const fz = (dz / dist) * force;
        forces[si][0] += fx;
        forces[si][1] += fy;
        forces[si][2] += fz;
        forces[ti][0] -= fx;
        forces[ti][1] -= fy;
        forces[ti][2] -= fz;
    }

    // Centering force
    const cenStr = CENTERING_STRENGTH;
    for (let i = 0; i < n; i++) {
        forces[i][0] -= nodes[i].x * cenStr;
        forces[i][1] -= nodes[i].y * cenStr;
        forces[i][2] -= nodes[i].z * cenStr;
    }

    // Velocity Verlet integration
    for (let i = 0; i < n; i++) {
        velocities[i][0] = (velocities[i][0] + forces[i][0] * dt) * DAMPING;
        velocities[i][1] = (velocities[i][1] + forces[i][1] * dt) * DAMPING;
        velocities[i][2] = (velocities[i][2] + forces[i][2] * dt) * DAMPING;

        // Clamp velocity
        const vLen = vec3Length(velocities[i]);
        if (vLen > 5.0) {
            const scale = 5.0 / vLen;
            velocities[i][0] *= scale;
            velocities[i][1] *= scale;
            velocities[i][2] *= scale;
        }

        nodes[i].x += velocities[i][0] * dt;
        nodes[i].y += velocities[i][1] * dt;
        nodes[i].z += velocities[i][2] * dt;
    }

    // Cool down
    simulationCooling *= 0.998;
    if (simulationCooling < 0.01) {
        simulationCooling = 0.01;
    }
}


// ============================================================
// LOD System
// ============================================================

function getCurrentLOD() {
    const dist = camera.orbitDistance;
    if (dist > LOD_MACRO_THRESHOLD) return LOD_MACRO;
    if (dist > LOD_MID_THRESHOLD)   return LOD_MID;
    return LOD_NEAR;
}

function getLODLabel(lod) {
    switch (lod) {
        case LOD_MACRO: return "MACRO";
        case LOD_MID:   return "MID";
        case LOD_NEAR:  return "NEAR";
        default: return "?";
    }
}

function filterByLOD() {
    const lod = getCurrentLOD();

    switch (lod) {
        case LOD_MACRO:
            visibleNodes = nodes.filter(n => n.level === 0);
            visibleEdges = [];
            break;
        case LOD_MID:
            visibleNodes = nodes.filter(n => n.level <= 1);
            // Show edges between visible nodes with weight > 0.3
            {
                const visibleIds = new Set(visibleNodes.map(n => n.id));
                visibleEdges = edges.filter(e =>
                    visibleIds.has(e.source) && visibleIds.has(e.target) &&
                    (e.weight || 0) > 0.3
                );
            }
            break;
        case LOD_NEAR:
            visibleNodes = [...nodes];
            visibleEdges = [...edges];
            break;
    }
}


// ============================================================
// WebGPU Initialization
// ============================================================

async function initWebGPU() {
    canvas = document.getElementById("gpu-canvas");
    if (!navigator.gpu) {
        showFallback("WebGPU is not supported in this browser.");
        return false;
    }

    const adapter = await navigator.gpu.requestAdapter();
    if (!adapter) {
        showFallback("Could not obtain WebGPU adapter.");
        return false;
    }

    device = await adapter.requestDevice();
    device.lost.then((info) => {
        console.error("WebGPU device lost:", info.message);
    });

    canvasFormat = navigator.gpu.getPreferredCanvasFormat();
    context = canvas.getContext("webgpu");
    context.configure({
        device: device,
        format: canvasFormat,
        alphaMode: "premultiplied",
    });

    return true;
}

function showFallback(message) {
    console.log("WebGPU unavailable, activating Canvas2D fallback: " + message);
    use2DFallback = true;

    // Hide the fallback message div and the GPU canvas
    document.getElementById("fallback").style.display = "none";
    var gpuCanvas = document.getElementById("gpu-canvas");
    gpuCanvas.style.display = "none";

    // Create a fresh canvas for 2D (the GPU canvas may already be locked to webgpu context)
    canvas = document.createElement("canvas");
    canvas.id = "canvas-2d";
    canvas.style.position = "absolute";
    canvas.style.inset = "0";
    canvas.style.display = "block";
    canvas.style.cursor = "grab";
    document.body.appendChild(canvas);

    // Get 2D context
    ctx2d = canvas.getContext("2d");

    // Size canvas to window
    function resizeCanvas2D() {
        canvas.width = window.innerWidth * (window.devicePixelRatio || 1);
        canvas.height = window.innerHeight * (window.devicePixelRatio || 1);
        canvas.style.width = window.innerWidth + "px";
        canvas.style.height = window.innerHeight + "px";
        ctx2d.setTransform(window.devicePixelRatio || 1, 0, 0, window.devicePixelRatio || 1, 0, 0);
    }
    resizeCanvas2D();
    window.addEventListener("resize", resizeCanvas2D);

    // Setup 2D input (pan, zoom, click)
    setup2DInput();

    // Start 2D render loop
    requestAnimationFrame(frame2D);
}


// ============================================================
// Shader and Pipeline Creation
// ============================================================

async function loadShaders() {
    // Try loading from file first, fall back to fetch
    try {
        const response = await fetch("universe.wgsl");
        if (response.ok) {
            wgslSource = await response.text();
        }
    } catch (e) {
        console.warn("Could not load universe.wgsl from file, using embedded shaders.");
    }

    if (!wgslSource) {
        // Inline fallback: minimal shaders
        console.error("WGSL source not available. Rendering will not work.");
        return false;
    }

    shaderModule = device.createShaderModule({
        label: "Universe Shader Module",
        code: wgslSource,
    });

    // Check for compilation errors
    const info = await shaderModule.getCompilationInfo();
    for (const msg of info.messages) {
        if (msg.type === "error") {
            console.error("Shader compilation error:", msg.message, "line:", msg.lineNum);
            return false;
        }
        if (msg.type === "warning") {
            console.warn("Shader warning:", msg.message);
        }
    }

    return true;
}

function createPipelines() {
    // Uniform bind group layout
    uniformBindGroupLayout = device.createBindGroupLayout({
        label: "Camera Uniforms BGL",
        entries: [{
            binding: 0,
            visibility: GPUShaderStage.VERTEX | GPUShaderStage.FRAGMENT,
            buffer: { type: "uniform" },
        }],
    });

    const pipelineLayout = device.createPipelineLayout({
        label: "Universe Pipeline Layout",
        bindGroupLayouts: [uniformBindGroupLayout],
    });

    // Uniform buffer (CameraUniforms struct: 18 floats padded to 80 bytes = 20 floats)
    uniformBuffer = device.createBuffer({
        label: "Camera Uniform Buffer",
        size: 256, // generous allocation, aligned to 256
        usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST,
    });

    uniformBindGroup = device.createBindGroup({
        label: "Camera Uniforms BG",
        layout: uniformBindGroupLayout,
        entries: [{
            binding: 0,
            resource: { buffer: uniformBuffer },
        }],
    });

    // --- Node pipeline ---
    nodePipeline = device.createRenderPipeline({
        label: "Node Pipeline",
        layout: pipelineLayout,
        vertex: {
            module: shaderModule,
            entryPoint: "nodeVertex",
            buffers: [
                // Quad vertex buffer (position + uv)
                {
                    arrayStride: 16,  // 2 floats pos + 2 floats uv = 16 bytes
                    stepMode: "vertex",
                    attributes: [
                        { shaderLocation: 0, offset: 0, format: "float32x2" },  // position
                        { shaderLocation: 1, offset: 8, format: "float32x2" },  // uv
                    ],
                },
                // Instance buffer
                {
                    arrayStride: 48,  // 12 floats = 48 bytes per instance
                    stepMode: "instance",
                    attributes: [
                        { shaderLocation: 2, offset: 0,  format: "float32x3" }, // worldPos
                        { shaderLocation: 3, offset: 12, format: "float32" },   // size
                        { shaderLocation: 4, offset: 16, format: "float32x4" }, // color
                        { shaderLocation: 5, offset: 32, format: "float32" },   // nodeLevel
                        { shaderLocation: 6, offset: 36, format: "float32" },   // focused
                        { shaderLocation: 7, offset: 40, format: "float32" },   // hovered
                        { shaderLocation: 8, offset: 44, format: "float32" },   // extra
                    ],
                },
            ],
        },
        fragment: {
            module: shaderModule,
            entryPoint: "nodeFragment",
            targets: [{
                format: canvasFormat,
                blend: {
                    color: {
                        srcFactor: "src-alpha",
                        dstFactor: "one-minus-src-alpha",
                        operation: "add",
                    },
                    alpha: {
                        srcFactor: "one",
                        dstFactor: "one-minus-src-alpha",
                        operation: "add",
                    },
                },
            }],
        },
        primitive: {
            topology: "triangle-list",
            cullMode: "none",
        },
        depthStencil: {
            format: "depth24plus",
            depthWriteEnabled: true,
            depthCompare: "less",
        },
    });

    // --- Edge pipeline ---
    edgePipeline = device.createRenderPipeline({
        label: "Edge Pipeline",
        layout: pipelineLayout,
        vertex: {
            module: shaderModule,
            entryPoint: "edgeVertex",
            buffers: [{
                arrayStride: 28,  // 3 floats pos + 4 floats color = 28 bytes
                stepMode: "vertex",
                attributes: [
                    { shaderLocation: 0, offset: 0,  format: "float32x3" }, // position
                    { shaderLocation: 1, offset: 12, format: "float32x4" }, // color
                ],
            }],
        },
        fragment: {
            module: shaderModule,
            entryPoint: "edgeFragment",
            targets: [{
                format: canvasFormat,
                blend: {
                    color: {
                        srcFactor: "src-alpha",
                        dstFactor: "one-minus-src-alpha",
                        operation: "add",
                    },
                    alpha: {
                        srcFactor: "one",
                        dstFactor: "one-minus-src-alpha",
                        operation: "add",
                    },
                },
            }],
        },
        primitive: {
            topology: "line-list",
        },
        depthStencil: {
            format: "depth24plus",
            depthWriteEnabled: false,
            depthCompare: "less",
        },
    });

    // Quad vertices for node billboards
    // Two triangles: (-1,-1), (1,-1), (1,1), (-1,1)
    const quadVerts = new Float32Array([
        // pos.x, pos.y, uv.x, uv.y
        -1, -1,  0, 1,
         1, -1,  1, 1,
         1,  1,  1, 0,
        -1,  1,  0, 0,
    ]);
    nodeQuadVB = device.createBuffer({
        label: "Node Quad VB",
        size: quadVerts.byteLength,
        usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST,
    });
    device.queue.writeBuffer(nodeQuadVB, 0, quadVerts);

    const quadIndices = new Uint16Array([0, 1, 2, 0, 2, 3]);
    nodeQuadIB = device.createBuffer({
        label: "Node Quad IB",
        size: quadIndices.byteLength,
        usage: GPUBufferUsage.INDEX | GPUBufferUsage.COPY_DST,
    });
    device.queue.writeBuffer(nodeQuadIB, 0, quadIndices);
}

function ensureDepthTexture() {
    const w = canvas.width;
    const h = canvas.height;
    if (depthTexture && depthTexture.width === w && depthTexture.height === h) return;
    if (depthTexture) depthTexture.destroy();
    depthTexture = device.createTexture({
        label: "Depth Texture",
        size: [w, h],
        format: "depth24plus",
        usage: GPUTextureUsage.RENDER_ATTACHMENT,
    });
    depthTextureView = depthTexture.createView();
}


// ============================================================
// Buffer Updates
// ============================================================

function updateNodeInstanceBuffer() {
    const count = visibleNodes.length;
    if (count === 0) return;

    const floatsPerNode = 12;
    const data = new Float32Array(count * floatsPerNode);

    for (let i = 0; i < count; i++) {
        const n = visibleNodes[i];
        const off = i * floatsPerNode;
        data[off + 0] = n.x;
        data[off + 1] = n.y;
        data[off + 2] = n.z;
        // Size based on level and importance
        let size = n.size || 1.0;
        if (n.level === 0) size *= 2.5;
        else if (n.level === 1) size *= 1.5;
        data[off + 3] = size;
        // Color
        const col = n._rgb || [0.5, 0.5, 0.5];
        data[off + 4] = col[0];
        data[off + 5] = col[1];
        data[off + 6] = col[2];
        data[off + 7] = 1.0; // alpha
        // Level
        data[off + 8] = n.level;
        // Focused (distance to focus)
        const dx = n.x - focusPosition[0];
        const dy = n.y - focusPosition[1];
        const dz = n.z - focusPosition[2];
        const distToFocus = Math.sqrt(dx*dx + dy*dy + dz*dz);
        data[off + 9] = distToFocus < focusRadius ? (1.0 - distToFocus / focusRadius) : 0.0;
        // Hovered
        data[off + 10] = (mouse.hoveredNodeIndex >= 0 && visibleNodes[mouse.hoveredNodeIndex] === n) ? 1.0 : 0.0;
        // Extra
        data[off + 11] = 0.0;
    }

    const requiredSize = data.byteLength;
    if (!nodeInstanceBuffer || nodeInstanceBuffer.size < requiredSize) {
        if (nodeInstanceBuffer) nodeInstanceBuffer.destroy();
        nodeInstanceBuffer = device.createBuffer({
            label: "Node Instance Buffer",
            size: Math.max(requiredSize, 256),
            usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST,
        });
    }
    device.queue.writeBuffer(nodeInstanceBuffer, 0, data);
    nodeInstanceData = data;
}

function updateEdgeVertexBuffer() {
    const count = visibleEdges.length;
    if (count === 0) return;

    // Each edge = 2 vertices, each vertex = 3 floats pos + 4 floats color = 7 floats
    const floatsPerVertex = 7;
    const data = new Float32Array(count * 2 * floatsPerVertex);

    // Build node lookup by ID
    const nodeMap = new Map();
    for (const n of nodes) {
        nodeMap.set(n.id, n);
    }

    let validCount = 0;
    for (let i = 0; i < count; i++) {
        const e = visibleEdges[i];
        const sn = nodeMap.get(e.source);
        const tn = nodeMap.get(e.target);
        if (!sn || !tn) continue;

        const off = validCount * 2 * floatsPerVertex;
        const alpha = Math.min(1.0, (e.weight || 0.5) * 0.6);

        // Determine edge color: blend of source and target
        const sc = sn._rgb || [0.5, 0.5, 0.5];
        const tc = tn._rgb || [0.5, 0.5, 0.5];

        // Source vertex
        data[off + 0] = sn.x;
        data[off + 1] = sn.y;
        data[off + 2] = sn.z;
        data[off + 3] = sc[0] * 0.7;
        data[off + 4] = sc[1] * 0.7;
        data[off + 5] = sc[2] * 0.7;
        data[off + 6] = alpha;

        // Target vertex
        data[off + 7]  = tn.x;
        data[off + 8]  = tn.y;
        data[off + 9]  = tn.z;
        data[off + 10] = tc[0] * 0.7;
        data[off + 11] = tc[1] * 0.7;
        data[off + 12] = tc[2] * 0.7;
        data[off + 13] = alpha;

        validCount++;
    }

    if (validCount === 0) {
        edgeVertexData = null;
        return;
    }

    const requiredSize = validCount * 2 * floatsPerVertex * 4;
    if (!edgeVertexBuffer || edgeVertexBuffer.size < requiredSize) {
        if (edgeVertexBuffer) edgeVertexBuffer.destroy();
        edgeVertexBuffer = device.createBuffer({
            label: "Edge Vertex Buffer",
            size: Math.max(requiredSize, 256),
            usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST,
        });
    }
    device.queue.writeBuffer(edgeVertexBuffer, 0, data, 0, validCount * 2 * floatsPerVertex);
    edgeVertexData = { count: validCount * 2 };
}


// ============================================================
// Camera
// ============================================================

function updateCamera() {
    // Smooth interpolation towards target values
    const s = camera.smoothSpeed;
    camera.orbitTheta    += (camera.targetOrbitTheta - camera.orbitTheta) * s;
    camera.orbitPhi      += (camera.targetOrbitPhi - camera.orbitPhi) * s;
    camera.orbitDistance  += (camera.targetOrbitDistance - camera.orbitDistance) * s;
    camera.target = vec3Lerp(camera.target, camera.targetTarget, s);

    // Clamp phi
    camera.orbitPhi = Math.max(0.1, Math.min(Math.PI - 0.1, camera.orbitPhi));

    // Compute camera position from spherical coordinates
    const sinPhi = Math.sin(camera.orbitPhi);
    const cosPhi = Math.cos(camera.orbitPhi);
    const sinTheta = Math.sin(camera.orbitTheta);
    const cosTheta = Math.cos(camera.orbitTheta);

    camera.position = [
        camera.target[0] + camera.orbitDistance * sinPhi * cosTheta,
        camera.target[1] + camera.orbitDistance * cosPhi,
        camera.target[2] + camera.orbitDistance * sinPhi * sinTheta,
    ];
}

function writeUniforms() {
    const aspect = canvas.width / canvas.height;
    const fov = Math.PI / 3.0;

    const viewMatrix = mat4LookAt(camera.position, camera.target, [0, 1, 0]);
    const projMatrix = mat4Perspective(fov, aspect, 0.1, 2000.0);
    const vpMatrix = mat4Multiply(projMatrix, viewMatrix);

    const fogNear = camera.orbitDistance * FOG_NEAR_RATIO;
    const fogFar  = camera.orbitDistance * FOG_FAR_RATIO;
    const lod = getCurrentLOD();
    const time = performance.now() / 1000.0;

    // CameraUniforms layout (must match WGSL struct exactly):
    // mat4x4 viewProjection  : 16 floats (offset 0)
    // mat4x4 view            : 16 floats (offset 64)
    // mat4x4 projection      : 16 floats (offset 128)
    // vec3 cameraPosition    : 3 floats  (offset 192)
    // f32 _pad0              : 1 float   (offset 204)
    // vec3 focusPosition     : 3 floats  (offset 208)
    // f32 focusRadius        : 1 float   (offset 220)
    // f32 fogNear            : 1 float   (offset 224)
    // f32 fogFar             : 1 float   (offset 228)
    // f32 fogDensity         : 1 float   (offset 232)
    // f32 time               : 1 float   (offset 236)
    // f32 viewportWidth      : 1 float   (offset 240)
    // f32 viewportHeight     : 1 float   (offset 244)
    // f32 lodLevel           : 1 float   (offset 248)
    // f32 _pad1              : 1 float   (offset 252)
    // Total: 256 bytes = 64 floats

    const data = new Float32Array(64);
    data.set(vpMatrix, 0);
    data.set(viewMatrix, 16);
    data.set(projMatrix, 32);
    data[48] = camera.position[0];
    data[49] = camera.position[1];
    data[50] = camera.position[2];
    data[51] = 0; // pad
    data[52] = focusPosition[0];
    data[53] = focusPosition[1];
    data[54] = focusPosition[2];
    data[55] = focusRadius;
    data[56] = fogNear;
    data[57] = fogFar;
    data[58] = 0.5; // fogDensity
    data[59] = time;
    data[60] = canvas.width;
    data[61] = canvas.height;
    data[62] = lod;
    data[63] = 0; // pad

    device.queue.writeBuffer(uniformBuffer, 0, data);
}


// ============================================================
// Rendering
// ============================================================

function render() {
    ensureDepthTexture();
    writeUniforms();

    const encoder = device.createCommandEncoder({ label: "Frame Encoder" });
    const textureView = context.getCurrentTexture().createView();

    const renderPass = encoder.beginRenderPass({
        colorAttachments: [{
            view: textureView,
            clearValue: { r: 0.04, g: 0.04, b: 0.08, a: 1.0 },
            loadOp: "clear",
            storeOp: "store",
        }],
        depthStencilAttachment: {
            view: depthTextureView,
            depthClearValue: 1.0,
            depthLoadOp: "clear",
            depthStoreOp: "store",
        },
    });

    // Draw edges first (behind nodes)
    if (edgeVertexData && edgeVertexData.count > 0 && edgeVertexBuffer) {
        renderPass.setPipeline(edgePipeline);
        renderPass.setBindGroup(0, uniformBindGroup);
        renderPass.setVertexBuffer(0, edgeVertexBuffer);
        renderPass.draw(edgeVertexData.count);
    }

    // Draw nodes
    if (visibleNodes.length > 0 && nodeInstanceBuffer) {
        renderPass.setPipeline(nodePipeline);
        renderPass.setBindGroup(0, uniformBindGroup);
        renderPass.setVertexBuffer(0, nodeQuadVB);
        renderPass.setVertexBuffer(1, nodeInstanceBuffer);
        renderPass.setIndexBuffer(nodeQuadIB, "uint16");
        renderPass.drawIndexed(6, visibleNodes.length);
    }

    renderPass.end();
    device.queue.submit([encoder.finish()]);
}


// ============================================================
// Hit Testing (ray-sphere intersection for node picking)
// ============================================================

function screenToRay(mx, my) {
    const rect = canvas.getBoundingClientRect();
    const x = ((mx - rect.left) / rect.width) * 2 - 1;
    const y = 1 - ((my - rect.top) / rect.height) * 2;

    const aspect = canvas.width / canvas.height;
    const fov = Math.PI / 3.0;
    const tanHalfFov = Math.tan(fov * 0.5);

    // Camera basis vectors
    const forward = vec3Normalize(vec3Sub(camera.target, camera.position));
    const right = vec3Normalize(vec3Cross(forward, [0, 1, 0]));
    const up = vec3Cross(right, forward);

    const dirX = right[0] * x * tanHalfFov * aspect + up[0] * y * tanHalfFov + forward[0];
    const dirY = right[1] * x * tanHalfFov * aspect + up[1] * y * tanHalfFov + forward[1];
    const dirZ = right[2] * x * tanHalfFov * aspect + up[2] * y * tanHalfFov + forward[2];

    return {
        origin: [...camera.position],
        direction: vec3Normalize([dirX, dirY, dirZ]),
    };
}

function hitTestNodes(mx, my) {
    const ray = screenToRay(mx, my);
    let closestDist = Infinity;
    let closestIdx = -1;

    for (let i = 0; i < visibleNodes.length; i++) {
        const n = visibleNodes[i];
        let hitRadius = (n.size || 1.0);
        if (n.level === 0) hitRadius *= 2.5;
        else if (n.level === 1) hitRadius *= 1.5;
        hitRadius *= 0.5;

        // Ray-sphere intersection
        const oc = vec3Sub(ray.origin, [n.x, n.y, n.z]);
        const b = vec3Dot(oc, ray.direction);
        const c = vec3Dot(oc, oc) - hitRadius * hitRadius;
        const discriminant = b * b - c;

        if (discriminant >= 0) {
            const t = -b - Math.sqrt(discriminant);
            if (t > 0 && t < closestDist) {
                closestDist = t;
                closestIdx = i;
            }
        }
    }
    return closestIdx;
}


// ============================================================
// Input Handling
// ============================================================

function setupInput() {
    canvas.addEventListener("mousedown", (e) => {
        e.preventDefault();
        if (e.button === 0) mouse.leftDown = true;
        if (e.button === 2) mouse.rightDown = true;
        mouse.lastX = e.clientX;
        mouse.lastY = e.clientY;
    });

    window.addEventListener("mouseup", (e) => {
        if (e.button === 0) {
            // Click detection (if barely moved)
            const dx = e.clientX - mouse.lastX;
            const dy = e.clientY - mouse.lastY;
            if (Math.abs(dx) < 3 && Math.abs(dy) < 3) {
                handleClick(e.clientX, e.clientY);
            }
            mouse.leftDown = false;
        }
        if (e.button === 2) mouse.rightDown = false;
    });

    canvas.addEventListener("mousemove", (e) => {
        const dx = e.clientX - mouse.x;
        const dy = e.clientY - mouse.y;
        mouse.x = e.clientX;
        mouse.y = e.clientY;

        if (mouse.leftDown) {
            // Orbit
            camera.targetOrbitTheta -= dx * 0.005;
            camera.targetOrbitPhi   += dy * 0.005;
            camera.targetOrbitPhi = Math.max(0.1, Math.min(Math.PI - 0.1, camera.targetOrbitPhi));
        }
        if (mouse.rightDown) {
            // Pan
            const panSpeed = camera.orbitDistance * 0.002;
            const forward = vec3Normalize(vec3Sub(camera.target, camera.position));
            const right = vec3Normalize(vec3Cross(forward, [0, 1, 0]));
            const up = vec3Cross(right, forward);
            camera.targetTarget = vec3Add(
                camera.targetTarget,
                vec3Add(
                    vec3Scale(right, -dx * panSpeed),
                    vec3Scale(up, dy * panSpeed)
                )
            );
        }

        // Hover test
        if (!mouse.leftDown && !mouse.rightDown) {
            mouse.hoveredNodeIndex = hitTestNodes(e.clientX, e.clientY);
            updateTooltip(e.clientX, e.clientY);
        }
    });

    canvas.addEventListener("wheel", (e) => {
        e.preventDefault();
        const zoomSpeed = camera.targetOrbitDistance * 0.001;
        camera.targetOrbitDistance += e.deltaY * zoomSpeed;
        camera.targetOrbitDistance = Math.max(MIN_ORBIT_DISTANCE, Math.min(MAX_ORBIT_DISTANCE, camera.targetOrbitDistance));
    }, { passive: false });

    canvas.addEventListener("contextmenu", (e) => e.preventDefault());

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
        const dpr = window.devicePixelRatio || 1;
        canvas.width  = Math.floor(canvas.clientWidth * dpr);
        canvas.height = Math.floor(canvas.clientHeight * dpr);
    });
    resizeObserver.observe(canvas);

    // Initial size
    const dpr = window.devicePixelRatio || 1;
    canvas.width  = Math.floor(canvas.clientWidth * dpr);
    canvas.height = Math.floor(canvas.clientHeight * dpr);
}

function handleClick(mx, my) {
    const idx = hitTestNodes(mx, my);
    if (idx >= 0) {
        const n = visibleNodes[idx];
        // Focus camera on this node
        focusOnNode(n);
        // Send message to Swift
        sendToSwift("nodeClicked", {
            id: n.id,
            label: n.label,
            level: n.level,
            cluster: n.cluster,
        });
    }
}

function focusOnNode(node) {
    camera.targetTarget = [node.x, node.y, node.z];
    focusPosition = [node.x, node.y, node.z];

    // Zoom in based on level
    let targetDist = camera.orbitDistance;
    if (node.level === 0) targetDist = Math.min(camera.orbitDistance, 60);
    else if (node.level === 1) targetDist = Math.min(camera.orbitDistance, 35);
    else targetDist = Math.min(camera.orbitDistance, 15);
    camera.targetOrbitDistance = targetDist;
    camera.smoothSpeed = 0.05; // slower for smooth transition
    // Restore normal speed after a bit
    setTimeout(() => { camera.smoothSpeed = 0.08; }, 2000);
}

function updateTooltip(mx, my) {
    if (mouse.hoveredNodeIndex >= 0) {
        const n = visibleNodes[mouse.hoveredNodeIndex];
        tooltipEl.textContent = n.label || n.id;
        tooltipEl.style.display = "block";
        tooltipEl.style.left = (mx + 15) + "px";
        tooltipEl.style.top  = (my - 10) + "px";
    } else {
        tooltipEl.style.display = "none";
    }
}


// ============================================================
// Swift Integration
// ============================================================

function sendToSwift(action, data) {
    try {
        if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.universe) {
            window.webkit.messageHandlers.universe.postMessage({
                action: action,
                ...data,
            });
        }
    } catch (e) {
        console.log("Swift message handler not available:", e);
    }
}


// ============================================================
// Data Loading
// ============================================================

window.loadUniverse = function(data) {
    universeData = data;

    // Determine total clusters for color assignment
    const clusterSet = new Set();
    for (const n of data.nodes) {
        if (n.cluster !== undefined) clusterSet.add(n.cluster);
    }
    const totalClusters = clusterSet.size || 1;

    // Process nodes
    nodes = data.nodes.map((n, i) => {
        const color = n.color
            ? (typeof n.color === "string" ? parseColor(n.color) : n.color)
            : clusterColor(n.cluster || 0, totalClusters);

        return {
            id:      n.id,
            label:   n.label || n.id,
            level:   n.level || 0,
            x:       n.x !== undefined ? n.x : (Math.random() - 0.5) * 50,
            y:       n.y !== undefined ? n.y : (Math.random() - 0.5) * 50,
            z:       n.z !== undefined ? n.z : (Math.random() - 0.5) * 50,
            size:    n.size || 1.0,
            cluster: n.cluster || 0,
            _rgb:    color,
        };
    });

    // Build node index lookup for edges
    const nodeIdxMap = new Map();
    nodes.forEach((n, i) => nodeIdxMap.set(n.id, i));

    // Process edges
    edges = data.edges.map(e => ({
        source:    e.source,
        target:    e.target,
        weight:    e.weight || 0.5,
        type:      e.type || "related",
        sourceIdx: nodeIdxMap.has(e.source) ? nodeIdxMap.get(e.source) : -1,
        targetIdx: nodeIdxMap.has(e.target) ? nodeIdxMap.get(e.target) : -1,
    }));

    // Initialize simulation
    initSimulation();

    // Run initial layout steps
    for (let i = 0; i < INITIAL_LAYOUT_STEPS; i++) {
        simulationStep(0.5);
    }
    simulationCooling = 0.3; // continue gentle simulation
    simulationRunning = true;

    // Reset camera
    camera.targetTarget = [0, 0, 0];
    camera.targetOrbitDistance = DEFAULT_ORBIT_DISTANCE;

    // Initial LOD filter
    filterByLOD();
    if (!use2DFallback) {
        updateNodeInstanceBuffer();
        updateEdgeVertexBuffer();
    }

    console.log(`Universe loaded: ${nodes.length} nodes, ${edges.length} edges`);
};

/**
 * Incrementally merge new nodes/edges into the existing universe
 * without resetting positions of already-placed nodes.
 * Used during live ingestion to show growth in real-time.
 */
window.mergeUniverse = function(data) {
    if (!data || !data.nodes) return;

    // Determine total clusters for color assignment
    const clusterSet = new Set();
    for (const n of nodes) clusterSet.add(n.cluster);
    for (const n of data.nodes) {
        if (n.cluster !== undefined) clusterSet.add(n.cluster);
    }
    const totalClusters = clusterSet.size || 1;

    // Index existing nodes by ID
    const existingIds = new Set(nodes.map(n => n.id));

    // Add only new nodes
    let addedCount = 0;
    for (const n of data.nodes) {
        if (existingIds.has(n.id)) continue;

        const color = n.color
            ? (typeof n.color === "string" ? parseColor(n.color) : n.color)
            : clusterColor(n.cluster || 0, totalClusters);

        nodes.push({
            id:      n.id,
            label:   n.label || n.id,
            level:   n.level || 0,
            x:       n.x !== undefined ? n.x : (Math.random() - 0.5) * 50,
            y:       n.y !== undefined ? n.y : (Math.random() - 0.5) * 50,
            z:       n.z !== undefined ? n.z : (Math.random() - 0.5) * 50,
            size:    n.size || 1.0,
            cluster: n.cluster || 0,
            _rgb:    color,
        });
        velocities.push([0, 0, 0]);
        addedCount++;
    }

    // Index existing edges
    const existingEdgeKeys = new Set(edges.map(e => e.source + "|" + e.target));

    // Rebuild node index lookup
    const nodeIdxMap = new Map();
    nodes.forEach((n, i) => nodeIdxMap.set(n.id, i));

    // Add only new edges
    let addedEdges = 0;
    for (const e of data.edges) {
        const key = e.source + "|" + e.target;
        if (existingEdgeKeys.has(key)) continue;

        edges.push({
            source:    e.source,
            target:    e.target,
            weight:    e.weight || 0.5,
            type:      e.type || "related",
            sourceIdx: nodeIdxMap.has(e.source) ? nodeIdxMap.get(e.source) : -1,
            targetIdx: nodeIdxMap.has(e.target) ? nodeIdxMap.get(e.target) : -1,
        });
        addedEdges++;
    }

    if (addedCount > 0 || addedEdges > 0) {
        // Gently re-heat simulation so new nodes settle
        simulationCooling = Math.max(simulationCooling, 0.3);
        simulationRunning = true;

        // Quick layout pass for new nodes
        for (let i = 0; i < 30; i++) {
            simulationStep(0.3);
        }

        filterByLOD();
        if (!use2DFallback) {
            updateNodeInstanceBuffer();
            updateEdgeVertexBuffer();
        }

        console.log(`Universe merged: +${addedCount} nodes, +${addedEdges} edges (total: ${nodes.length} nodes, ${edges.length} edges)`);
    }
};

function parseColor(str) {
    // Simple hex color parser
    if (str.startsWith("#")) {
        const hex = str.slice(1);
        const r = parseInt(hex.slice(0, 2), 16) / 255;
        const g = parseInt(hex.slice(2, 4), 16) / 255;
        const b = parseInt(hex.slice(4, 6), 16) / 255;
        return [r, g, b];
    }
    return [0.5, 0.5, 0.5];
}


// ============================================================
// Demo Data (used when no data is loaded from Swift)
// ============================================================

function loadDemoData() {
    const clusters = 5;
    const nodesPerCluster = 8;
    const subNodesPerConcept = 4;
    const chunksPerSub = 3;

    const demoNodes = [];
    const demoEdges = [];
    let id = 0;

    // Generate cluster centers
    for (let c = 0; c < clusters; c++) {
        const angle = (c / clusters) * Math.PI * 2;
        const cx = Math.cos(angle) * 30;
        const cz = Math.sin(angle) * 30;

        // Level 0 concepts
        for (let i = 0; i < nodesPerCluster; i++) {
            const conceptId = `concept_${id}`;
            const spread = 12;
            demoNodes.push({
                id: conceptId,
                label: `Concept ${c}.${i}`,
                level: 0,
                x: cx + (Math.random() - 0.5) * spread,
                y: (Math.random() - 0.5) * spread,
                z: cz + (Math.random() - 0.5) * spread,
                size: 1.5 + Math.random(),
                cluster: c,
            });

            // Edges between concepts in same cluster
            if (i > 0) {
                demoEdges.push({
                    source: `concept_${id}`,
                    target: `concept_${id - 1}`,
                    weight: 0.5 + Math.random() * 0.5,
                    type: "intra-cluster",
                });
            }

            // Level 1 sub-concepts
            for (let j = 0; j < subNodesPerConcept; j++) {
                const subId = `sub_${id}_${j}`;
                demoNodes.push({
                    id: subId,
                    label: `Sub ${c}.${i}.${j}`,
                    level: 1,
                    x: cx + (Math.random() - 0.5) * spread * 1.5,
                    y: (Math.random() - 0.5) * spread * 1.5,
                    z: cz + (Math.random() - 0.5) * spread * 1.5,
                    size: 0.8 + Math.random() * 0.5,
                    cluster: c,
                });
                demoEdges.push({
                    source: conceptId,
                    target: subId,
                    weight: 0.6 + Math.random() * 0.4,
                    type: "parent-child",
                });

                // Level 2 chunks
                for (let k = 0; k < chunksPerSub; k++) {
                    const chunkId = `chunk_${id}_${j}_${k}`;
                    demoNodes.push({
                        id: chunkId,
                        label: `Chunk ${c}.${i}.${j}.${k}`,
                        level: 2,
                        x: cx + (Math.random() - 0.5) * spread * 2,
                        y: (Math.random() - 0.5) * spread * 2,
                        z: cz + (Math.random() - 0.5) * spread * 2,
                        size: 0.3 + Math.random() * 0.3,
                        cluster: c,
                    });
                    demoEdges.push({
                        source: subId,
                        target: chunkId,
                        weight: 0.3 + Math.random() * 0.4,
                        type: "contains",
                    });
                }
            }

            id++;
        }

        // Inter-cluster edges
        if (c > 0) {
            demoEdges.push({
                source: `concept_${id - nodesPerCluster}`,
                target: `concept_${id - nodesPerCluster - nodesPerCluster + Math.floor(Math.random() * nodesPerCluster)}`,
                weight: 0.2 + Math.random() * 0.3,
                type: "inter-cluster",
            });
        }
    }

    window.loadUniverse({ nodes: demoNodes, edges: demoEdges });
}


// ============================================================
// Main Loop
// ============================================================

let lastFrameTime = 0;

function frame(timestamp) {
    requestAnimationFrame(frame);

    // FPS counter
    frameCount++;
    const now = performance.now();
    if (now - lastFpsTime >= 1000) {
        fps = frameCount;
        frameCount = 0;
        lastFpsTime = now;
        if (fpsDisplay) fpsDisplay.textContent = `${fps} FPS`;
    }

    // Simulation step
    if (simulationRunning && nodes.length > 0) {
        const dt = Math.min((timestamp - lastFrameTime) / 1000, 0.05);
        for (let s = 0; s < SIMULATION_STEPS; s++) {
            simulationStep(dt);
        }
    }
    lastFrameTime = timestamp;

    // Update camera
    updateCamera();

    // LOD filtering
    filterByLOD();

    // Update GPU buffers
    updateNodeInstanceBuffer();
    updateEdgeVertexBuffer();

    // Update UI
    if (nodeCountDisplay) {
        nodeCountDisplay.textContent = `${visibleNodes.length} / ${nodes.length} nodes`;
    }
    if (lodDisplay) {
        lodDisplay.textContent = `LOD: ${getLODLabel(getCurrentLOD())}`;
    }

    // Render
    render();
}


// ============================================================
// Canvas2D Fallback Renderer
// ============================================================

// 2D camera state
let cam2d = {
    x: 0,       // pan offset in world coords
    y: 0,
    zoom: 4.0,  // pixels per world unit
    dragging: false,
    lastX: 0,
    lastY: 0,
    startX: 0,  // mousedown position for click detection
    startY: 0,
    hoveredNode: -1,
};

function setup2DInput() {
    canvas.addEventListener("mousedown", function(e) {
        cam2d.dragging = true;
        cam2d.lastX = e.clientX;
        cam2d.lastY = e.clientY;
        cam2d.startX = e.clientX;
        cam2d.startY = e.clientY;
    });

    canvas.addEventListener("mousemove", function(e) {
        if (cam2d.dragging) {
            const dx = e.clientX - cam2d.lastX;
            const dy = e.clientY - cam2d.lastY;
            cam2d.x -= dx / cam2d.zoom;
            cam2d.y -= dy / cam2d.zoom;
            cam2d.lastX = e.clientX;
            cam2d.lastY = e.clientY;
        } else {
            // Hit test for hover
            const mx = e.clientX;
            const my = e.clientY;
            cam2d.hoveredNode = -1;
            const w = window.innerWidth;
            const h = window.innerHeight;
            for (let i = nodes.length - 1; i >= 0; i--) {
                const n = nodes[i];
                const sx = (n.x - cam2d.x) * cam2d.zoom + w / 2;
                const sy = (n.y - cam2d.y) * cam2d.zoom + h / 2;
                const r = Math.max(6, n.size * 4 * cam2d.zoom);
                const dx = mx - sx;
                const dy = my - sy;
                if (dx * dx + dy * dy < r * r) {
                    cam2d.hoveredNode = i;
                    break;
                }
            }
            canvas.style.cursor = cam2d.hoveredNode >= 0 ? "pointer" : "grab";

            // Tooltip
            if (cam2d.hoveredNode >= 0 && tooltipEl) {
                const n = nodes[cam2d.hoveredNode];
                tooltipEl.textContent = n.label;
                tooltipEl.style.display = "block";
                tooltipEl.style.left = (mx + 12) + "px";
                tooltipEl.style.top = (my - 8) + "px";
            } else if (tooltipEl) {
                tooltipEl.style.display = "none";
            }
        }
    });

    canvas.addEventListener("mouseup", function(e) {
        if (cam2d.dragging) {
            const dx = Math.abs(e.clientX - cam2d.startX);
            const dy = Math.abs(e.clientY - cam2d.startY);
            // If barely moved from mousedown, treat as click
            if (dx < 5 && dy < 5 && cam2d.hoveredNode >= 0) {
                const n = nodes[cam2d.hoveredNode];
                if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nodeSelected) {
                    window.webkit.messageHandlers.nodeSelected.postMessage(n.id);
                }
            }
        }
        cam2d.dragging = false;
    });

    canvas.addEventListener("wheel", function(e) {
        e.preventDefault();
        const factor = e.deltaY > 0 ? 0.9 : 1.1;
        cam2d.zoom *= factor;
        cam2d.zoom = Math.max(0.5, Math.min(30, cam2d.zoom));
    }, { passive: false });
}

function render2D() {
    if (!ctx2d) return;
    const w = window.innerWidth;
    const h = window.innerHeight;

    // Clear
    ctx2d.fillStyle = "#0a0a14";
    ctx2d.fillRect(0, 0, w, h);

    // Draw edges
    ctx2d.lineWidth = 1;
    for (const e of edges) {
        const si = e.sourceIdx;
        const ti = e.targetIdx;
        if (si < 0 || ti < 0) continue;
        const ns = nodes[si];
        const nt = nodes[ti];
        const sx = (ns.x - cam2d.x) * cam2d.zoom + w / 2;
        const sy = (ns.y - cam2d.y) * cam2d.zoom + h / 2;
        const tx = (nt.x - cam2d.x) * cam2d.zoom + w / 2;
        const ty = (nt.y - cam2d.y) * cam2d.zoom + h / 2;
        const alpha = Math.max(0.08, Math.min(0.5, (e.weight || 0.3) * 0.6));
        ctx2d.strokeStyle = `rgba(100, 140, 220, ${alpha})`;
        ctx2d.beginPath();
        ctx2d.moveTo(sx, sy);
        ctx2d.lineTo(tx, ty);
        ctx2d.stroke();
    }

    // Draw nodes
    for (let i = 0; i < nodes.length; i++) {
        const n = nodes[i];
        const sx = (n.x - cam2d.x) * cam2d.zoom + w / 2;
        const sy = (n.y - cam2d.y) * cam2d.zoom + h / 2;

        // Skip offscreen nodes
        if (sx < -100 || sx > w + 100 || sy < -100 || sy > h + 100) continue;

        const r = Math.max(3, n.size * 3 * Math.min(cam2d.zoom, 5));
        const rgb = n._rgb || [0.5, 0.5, 0.7];
        const cr = Math.round(rgb[0] * 255);
        const cg = Math.round(rgb[1] * 255);
        const cb = Math.round(rgb[2] * 255);

        // Glow for hovered node
        if (i === cam2d.hoveredNode) {
            ctx2d.shadowColor = `rgb(${cr}, ${cg}, ${cb})`;
            ctx2d.shadowBlur = 20;
        }

        ctx2d.fillStyle = `rgb(${cr}, ${cg}, ${cb})`;
        ctx2d.beginPath();
        ctx2d.arc(sx, sy, r, 0, Math.PI * 2);
        ctx2d.fill();

        // Reset shadow
        if (i === cam2d.hoveredNode) {
            ctx2d.shadowBlur = 0;
        }

        // Draw label if zoomed in enough or node is big
        if (cam2d.zoom > 2.0 || n.size > 1.5 || i === cam2d.hoveredNode) {
            ctx2d.font = `${Math.max(10, Math.min(14, 10 * cam2d.zoom / 3))}px -apple-system, sans-serif`;
            ctx2d.fillStyle = i === cam2d.hoveredNode ? "#ffffff" : "rgba(224, 224, 232, 0.8)";
            ctx2d.textAlign = "center";
            ctx2d.fillText(n.label, sx, sy - r - 4);
        }
    }
}

let lastFrame2DTime = 0;

function frame2D(timestamp) {
    requestAnimationFrame(frame2D);

    // FPS counter
    frameCount++;
    const now = performance.now();
    if (now - lastFpsTime >= 1000) {
        fps = frameCount;
        frameCount = 0;
        lastFpsTime = now;
        if (fpsDisplay) fpsDisplay.textContent = `${fps} FPS`;
    }

    // Simulation step
    if (simulationRunning && nodes.length > 0) {
        const dt = Math.min((timestamp - lastFrame2DTime) / 1000, 0.05);
        for (let s = 0; s < SIMULATION_STEPS; s++) {
            simulationStep(dt);
        }
    }
    lastFrame2DTime = timestamp;

    // Update UI
    if (nodeCountDisplay) {
        nodeCountDisplay.textContent = `${nodes.length} nodes`;
    }
    if (lodDisplay) {
        lodDisplay.textContent = "2D";
    }

    render2D();
}


// ============================================================
// Initialization
// ============================================================

async function init() {
    // Get UI elements
    fpsDisplay      = document.getElementById("fps");
    nodeCountDisplay = document.getElementById("node-count");
    lodDisplay       = document.getElementById("lod-level");
    tooltipEl        = document.getElementById("tooltip");

    const gpuReady = await initWebGPU();
    if (!gpuReady) {
        // showFallback already activated Canvas2D mode and started frame2D loop
        console.log("Using Canvas2D fallback renderer.");
        return;
    }

    const shadersReady = await loadShaders();
    if (!shadersReady) {
        showFallback("Failed to compile WebGPU shaders.");
        return;
    }

    createPipelines();
    setupInput();

    // Hide fallback, show canvas
    document.getElementById("fallback").style.display = "none";
    canvas.style.display = "block";

    // Load demo data if no external data provided after a short delay
    setTimeout(() => {
        if (nodes.length === 0) {
            console.log("No data loaded from Swift, loading demo data...");
            loadDemoData();
        }
    }, 500);

    // Start render loop
    requestAnimationFrame(frame);

    console.log("Concept Universe Renderer initialized (WebGPU).");
}

// Start when DOM is ready
if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
} else {
    init();
}
