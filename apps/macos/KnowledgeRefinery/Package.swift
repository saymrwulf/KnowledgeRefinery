// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "KnowledgeRefinery",
    platforms: [.macOS(.v15)],
    targets: [
        .executableTarget(
            name: "KnowledgeRefinery",
            path: "Sources",
            resources: [
                .copy("WebGPU/universe.html"),
                .copy("WebGPU/universe.js"),
                .copy("WebGPU/universe.wgsl"),
                .copy("Resources/AppIcon.icns"),
            ]
        ),
    ]
)
