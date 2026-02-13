import SwiftUI

/// Wrapper that looks up the workspace and daemon manager from the store,
/// then renders the actual content with @ObservedObject for reactive updates.
struct WorkspaceDetailView: View {
    let workspaceId: String
    @EnvironmentObject var workspaceStore: WorkspaceStore

    var body: some View {
        if let ws = workspaceStore.workspaces.first(where: { $0.id == workspaceId }) {
            WorkspaceContentView(
                workspace: ws,
                daemon: workspaceStore.managerFor(ws)
            )
            .environmentObject(workspaceStore)
        } else {
            VStack(spacing: 12) {
                Image(systemName: "exclamationmark.triangle")
                    .font(.system(size: 36))
                    .foregroundStyle(.secondary)
                Text("Workspace not found")
                    .font(.title3)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

/// The actual workspace content — takes @ObservedObject daemon so all
/// @Published properties trigger reactive updates.
struct WorkspaceContentView: View {
    let workspace: Workspace
    @ObservedObject var daemon: DaemonManager
    @EnvironmentObject var workspaceStore: WorkspaceStore
    @State private var activeSheet: DetailSheet?

    enum DetailSheet: Identifiable {
        case editWorkspace
        case logs
        var id: Int {
            switch self {
            case .editWorkspace: return 0
            case .logs: return 1
            }
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            // Header bar
            workspaceHeader
            Divider()

            // Data lakes + pipeline controls
            HStack(spacing: 0) {
                dataLakeSidebar
                    .frame(width: 340)
                Divider()
                // Embedded ContentView — the existing search/universe/concepts UI
                ContentView()
                    .environmentObject(daemon)
            }
        }
        .sheet(item: $activeSheet) { sheet in
            switch sheet {
            case .editWorkspace:
                WorkspaceSetupSheet(mode: .edit(workspace))
                    .environmentObject(workspaceStore)
            case .logs:
                daemonLogSheet
            }
        }
        .onAppear {
            // Start health polling (don't auto-launch daemon — user controls that)
            daemon.connect()
        }
    }

    // MARK: - Header

    /// Human-readable status for the header
    private var humanStatus: String {
        if daemon.isIngesting {
            switch daemon.ingestStatus?.currentStage {
            case "scanning": return "Discovering files..."
            case "extracting": return "Reading documents..."
            case "chunking": return "Splitting text..."
            case "embedding": return "Building search index..."
            case "annotating": return "Analyzing content..."
            case "conceptualizing": return "Finding themes..."
            case "completed": return "Processing complete"
            default: return "Processing..."
            }
        }
        if daemon.isConnected { return "Ready" }
        if daemon.isDaemonRunning { return "Starting up..." }
        return "Offline"
    }

    private var workspaceHeader: some View {
        HStack(spacing: 12) {
            Circle()
                .fill(tagColor(workspace.colorTag))
                .frame(width: 14, height: 14)
            Text(workspace.name)
                .font(.title3.bold())

            HStack(spacing: 6) {
                Circle()
                    .fill(daemon.isConnected ? Color.green : Color.red)
                    .frame(width: 10, height: 10)
                Text(humanStatus)
                    .font(.callout)
                    .foregroundStyle(.secondary)
            }

            Spacer()

            // Workspace controls — clear labels, proper sizing
            if daemon.isConnected {
                Button {
                    daemon.restartDaemon()
                } label: {
                    Label("Restart", systemImage: "arrow.clockwise")
                }
                .help("Restart the processing engine")

                Button {
                    daemon.stopDaemon()
                } label: {
                    Label("Shut Down", systemImage: "power")
                }
                .tint(.red)
                .help("Stop the processing engine")
            } else {
                Button {
                    daemon.launchDaemon()
                } label: {
                    Label("Start Up", systemImage: "power")
                }
                .buttonStyle(.borderedProminent)
                .help("Start the processing engine")
            }

            Button {
                activeSheet = .logs
            } label: {
                Label("Logs", systemImage: "doc.text")
            }
            .help("View processing engine logs")

            Button {
                activeSheet = .editWorkspace
            } label: {
                Label("Settings", systemImage: "gear")
            }
            .help("Edit workspace settings")
        }
        .padding(.horizontal)
        .padding(.vertical, 10)
        .background(.bar)
    }

    // MARK: - Data Lake Sidebar

    private var dataLakeSidebar: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("Source Folders")
                .font(.title3.bold())
                .padding(.horizontal)
                .padding(.top, 12)
                .padding(.bottom, 8)

            if workspace.dataLakePaths.isEmpty {
                VStack(spacing: 10) {
                    Text("No source folders configured.")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                    Button("Add Source Folders") {
                        activeSheet = .editWorkspace
                    }
                    .controlSize(.regular)
                }
                .padding(.horizontal)
                .frame(maxHeight: .infinity)
            } else {
                List {
                    ForEach(workspace.dataLakePaths, id: \.self) { path in
                        HStack {
                            Image(systemName: "folder.fill")
                                .foregroundStyle(.blue)
                                .font(.body)
                            VStack(alignment: .leading, spacing: 2) {
                                Text(URL(fileURLWithPath: path).lastPathComponent)
                                    .font(.callout.bold())
                                Text(path)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                                    .truncationMode(.middle)
                            }
                        }
                        .padding(.vertical, 2)
                    }
                }
                .listStyle(.plain)
            }

            Divider()

            // Pipeline progress panel
            ScrollView {
                PipelineProgressPanel(daemon: daemon)
            }
        }
        .background(Color(nsColor: .controlBackgroundColor))
    }

    // MARK: - Log Sheet

    private var daemonLogSheet: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text("Processing Engine Logs")
                    .font(.title3.bold())
                Spacer()
                Button("Clear") {
                    daemon.logLines.removeAll()
                }
                Button("Close") {
                    activeSheet = nil
                }
            }
            .padding()
            Divider()

            if daemon.logLines.isEmpty {
                VStack(spacing: 8) {
                    Text("No log output yet.")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                    if !daemon.isDaemonRunning {
                        Text("Start the processing engine to see logs.")
                            .font(.callout)
                            .foregroundStyle(.tertiary)
                    }
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        ForEach(Array(daemon.logLines.enumerated()), id: \.offset) { _, line in
                            Text(line)
                                .font(.system(.caption, design: .monospaced))
                                .textSelection(.enabled)
                        }
                    }
                    .padding()
                }
            }
        }
        .frame(width: 700, height: 400)
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
