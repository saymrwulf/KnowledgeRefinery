import SwiftUI

struct DataLakeMappingView: View {
    @EnvironmentObject var store: WorkspaceStore
    @Environment(\.dismiss) private var dismiss

    /// All unique data lake paths across all workspaces.
    private var allPaths: [String] {
        Array(Set(store.workspaces.flatMap(\.dataLakePaths))).sorted()
    }

    /// Map from data lake path to workspace IDs that reference it.
    private var pathToWorkspaces: [String: [Workspace]] {
        var map: [String: [Workspace]] = [:]
        for ws in store.workspaces {
            for path in ws.dataLakePaths {
                map[path, default: []].append(ws)
            }
        }
        return map
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Data Lake Mapping")
                    .font(.title2.bold())
                Spacer()
                Button("Done") { dismiss() }
                    .keyboardShortcut(.cancelAction)
            }
            .padding()
            Divider()

            if allPaths.isEmpty {
                VStack(spacing: 8) {
                    Image(systemName: "folder.badge.questionmark")
                        .font(.system(size: 36))
                        .foregroundStyle(.secondary)
                    Text("No data lakes configured")
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                GeometryReader { geo in
                    HStack(spacing: 0) {
                        // Left column: Data Lake paths
                        ScrollView {
                            VStack(alignment: .leading, spacing: 12) {
                                Text("Data Lakes")
                                    .font(.headline)
                                    .padding(.bottom, 4)
                                ForEach(allPaths, id: \.self) { path in
                                    dataLakeRow(path)
                                }
                            }
                            .padding()
                        }
                        .frame(width: geo.size.width * 0.45)

                        // Center: Connection lines
                        Canvas { context, size in
                            drawConnections(context: context, size: size)
                        }
                        .frame(width: geo.size.width * 0.1)

                        // Right column: Workspaces
                        ScrollView {
                            VStack(alignment: .leading, spacing: 12) {
                                Text("Workspaces")
                                    .font(.headline)
                                    .padding(.bottom, 4)
                                ForEach(store.workspaces) { ws in
                                    workspaceRow(ws)
                                }
                            }
                            .padding()
                        }
                        .frame(width: geo.size.width * 0.45)
                    }
                }
            }
        }
        .frame(width: 700, height: 500)
    }

    private func dataLakeRow(_ path: String) -> some View {
        HStack {
            Image(systemName: "folder.fill")
                .foregroundStyle(.blue)
            VStack(alignment: .leading) {
                Text(URL(fileURLWithPath: path).lastPathComponent)
                    .font(.caption.bold())
                Text(path)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            Spacer()
            if let wsList = pathToWorkspaces[path] {
                Text("\(wsList.count)")
                    .font(.caption2)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(.quaternary)
                    .clipShape(Capsule())
            }
        }
        .padding(8)
        .background(Color(nsColor: .controlBackgroundColor))
        .clipShape(RoundedRectangle(cornerRadius: 6))
    }

    private func workspaceRow(_ ws: Workspace) -> some View {
        HStack {
            Circle()
                .fill(tagColor(ws.colorTag))
                .frame(width: 10, height: 10)
            VStack(alignment: .leading) {
                Text(ws.name)
                    .font(.caption.bold())
                Text("\(ws.dataLakePaths.count) data lake\(ws.dataLakePaths.count == 1 ? "" : "s")")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            Text(":\(ws.port)")
                .font(.caption2.monospaced())
                .foregroundStyle(.secondary)
        }
        .padding(8)
        .background(Color(nsColor: .controlBackgroundColor))
        .clipShape(RoundedRectangle(cornerRadius: 6))
    }

    private func drawConnections(context: GraphicsContext, size: CGSize) {
        // Simple visual: draw lines from center-left to center-right
        // Color-coded by workspace color tag
        let pathCount = allPaths.count
        let wsCount = store.workspaces.count
        guard pathCount > 0, wsCount > 0 else { return }

        let leftSpacing = size.height / CGFloat(pathCount + 1)
        let rightSpacing = size.height / CGFloat(wsCount + 1)

        for (wsIndex, ws) in store.workspaces.enumerated() {
            let rightY = rightSpacing * CGFloat(wsIndex + 1)
            let color = tagColor(ws.colorTag)

            for pathStr in ws.dataLakePaths {
                if let pathIndex = allPaths.firstIndex(of: pathStr) {
                    let leftY = leftSpacing * CGFloat(pathIndex + 1)
                    var path = Path()
                    path.move(to: CGPoint(x: 0, y: leftY))
                    path.addCurve(
                        to: CGPoint(x: size.width, y: rightY),
                        control1: CGPoint(x: size.width * 0.4, y: leftY),
                        control2: CGPoint(x: size.width * 0.6, y: rightY)
                    )
                    context.stroke(path, with: .color(color.opacity(0.6)), lineWidth: 2)
                }
            }
        }
    }

    private func tagColor(_ tag: String) -> Color {
        switch tag {
        case "blue": return .blue
        case "green": return .green
        case "orange": return .orange
        case "purple": return .purple
        case "red": return .red
        case "yellow": return .yellow
        case "pink": return .pink
        default: return .gray
        }
    }
}
