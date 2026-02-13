import SwiftUI

struct MasterDashboardView: View {
    @EnvironmentObject var workspaceStore: WorkspaceStore
    @EnvironmentObject var lmMonitor: LMStudioMonitor
    @State private var activeSheet: DashboardSheet?
    @Environment(\.openWindow) private var openWindow

    enum DashboardSheet: Identifiable {
        case newWorkspace
        case dataLakeMapping
        var id: Int {
            switch self {
            case .newWorkspace: return 0
            case .dataLakeMapping: return 1
            }
        }
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                // Top bar
                HStack {
                    Text("Knowledge Refinery")
                        .font(.largeTitle.bold())
                    Spacer()
                    Button {
                        activeSheet = .dataLakeMapping
                    } label: {
                        Label("Source Folders", systemImage: "map")
                    }
                    .controlSize(.large)
                    Button {
                        activeSheet = .newWorkspace
                    } label: {
                        Label("New Workspace", systemImage: "plus")
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                    Button {
                        workspaceStore.startAll()
                    } label: {
                        Label("Launch All", systemImage: "bolt.fill")
                    }
                    .buttonStyle(.bordered)
                    .controlSize(.large)
                    .disabled(workspaceStore.workspaces.isEmpty)
                    .help("Start all workspaces and begin processing documents")
                }
                .padding(.horizontal)

                // LM Studio Card
                LMStudioCard(monitor: lmMonitor)
                    .padding(.horizontal)

                // Workspace Grid
                if workspaceStore.workspaces.isEmpty {
                    emptyState
                } else {
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 320), spacing: 16)], spacing: 16) {
                        ForEach(workspaceStore.workspaces) { ws in
                            WorkspaceCard(
                                workspace: ws,
                                manager: workspaceStore.managerFor(ws),
                                onOpen: {
                                    openWindow(id: "workspace", value: ws.id)
                                },
                                onDelete: {
                                    workspaceStore.deleteWorkspace(ws)
                                }
                            )
                        }
                    }
                    .padding(.horizontal)
                }
            }
            .padding(.vertical)
        }
        .sheet(item: $activeSheet) { sheet in
            switch sheet {
            case .newWorkspace:
                WorkspaceSetupSheet(mode: .create)
                    .environmentObject(workspaceStore)
            case .dataLakeMapping:
                DataLakeMappingView()
                    .environmentObject(workspaceStore)
            }
        }
        .onAppear {
            lmMonitor.startPolling()
        }
        .onReceive(NotificationCenter.default.publisher(for: .krNewWorkspace)) { _ in
            activeSheet = .newWorkspace
        }
    }

    private var emptyState: some View {
        VStack(spacing: 16) {
            Image(systemName: "shippingbox")
                .font(.system(size: 56))
                .foregroundStyle(.secondary)
            Text("No Workspaces Yet")
                .font(.title)
            Text("Create a workspace to organize, search, and explore your documents with AI.")
                .font(.body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 400)
            Button {
                activeSheet = .newWorkspace
            } label: {
                Label("Create Your First Workspace", systemImage: "plus")
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.extraLarge)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 60)
    }
}

// MARK: - LM Studio Card

struct LMStudioCard: View {
    @ObservedObject var monitor: LMStudioMonitor
    @State private var showBrutalConfirm = false

    var body: some View {
        GroupBox {
            VStack(spacing: 8) {
                HStack(spacing: 16) {
                    VStack {
                        Circle()
                            .fill(monitor.isAlive ? Color.green : Color.red)
                            .frame(width: 16, height: 16)
                        Text(monitor.isAlive ? "Online" : "Offline")
                            .font(.callout)
                            .foregroundStyle(.secondary)
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("LM Studio")
                            .font(.title3.bold())
                        Text("Local AI Engine")
                            .font(.callout)
                            .foregroundStyle(.secondary)
                    }

                    Divider()
                        .frame(height: 40)

                    if monitor.isAlive {
                        VStack(alignment: .leading, spacing: 4) {
                            if let chat = monitor.chatModel {
                                Label(chat, systemImage: "bubble.left")
                                    .font(.callout)
                            }
                            if let embed = monitor.embeddingModel {
                                Label(embed, systemImage: "arrow.trianglehead.branch")
                                    .font(.callout)
                            }
                        }
                        Spacer()
                        VStack(alignment: .trailing, spacing: 4) {
                            Text("\(monitor.allModels.count) model\(monitor.allModels.count == 1 ? "" : "s") loaded")
                                .font(.callout)
                                .foregroundStyle(.secondary)
                            if let ctx = monitor.contextLength {
                                Text("Context: \(ctx / 1024)K tokens")
                                    .font(.callout)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    } else {
                        Text("Start LM Studio to enable AI features")
                            .font(.callout)
                            .foregroundStyle(.secondary)
                        Spacer()
                    }
                }

                // Repair controls
                if monitor.isRepairing {
                    HStack(spacing: 8) {
                        ProgressView()
                            .controlSize(.small)
                        Text(monitor.repairMessage ?? "Repairing...")
                            .font(.callout)
                            .foregroundStyle(.orange)
                        Spacer()
                    }
                } else {
                    HStack(spacing: 12) {
                        Spacer()
                        Button {
                            monitor.gracefulRelaunch()
                        } label: {
                            Label("Restart", systemImage: "arrow.clockwise")
                        }
                        .buttonStyle(.bordered)
                        .help("Quit and reopen LM Studio")

                        Button {
                            showBrutalConfirm = true
                        } label: {
                            Label("Reset & Restart", systemImage: "bolt.trianglebadge.exclamationmark")
                        }
                        .buttonStyle(.bordered)
                        .tint(.red)
                        .help("Force-kill, clear cache, and reopen. Use when LM Studio is stuck.")
                    }
                }
            }
            .padding(8)
        } label: {
            Label("AI Engine", systemImage: "cpu")
        }
        .confirmationDialog("Reset & Relaunch LM Studio?", isPresented: $showBrutalConfirm) {
            Button("Reset & Relaunch", role: .destructive) {
                monitor.brutalRelaunch()
            }
        } message: {
            Text("This will force-kill LM Studio, delete its cache files, and reopen it. Models will need to reload.")
        }
    }
}

// MARK: - Workspace Card

struct WorkspaceCard: View {
    let workspace: Workspace
    @ObservedObject var manager: DaemonManager
    let onOpen: () -> Void
    let onDelete: () -> Void
    @State private var showDeleteConfirm = false

    private var tagColor: Color {
        switch workspace.colorTag {
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

    private var humanStage: String {
        switch manager.ingestStatus?.currentStage {
        case "scanning": return "Discovering files..."
        case "extracting": return "Reading documents..."
        case "chunking": return "Splitting text..."
        case "embedding": return "Building search index..."
        case "annotating": return "Analyzing content..."
        case "conceptualizing": return "Finding themes..."
        case "completed": return "Processing complete"
        default: return ""
        }
    }

    var body: some View {
        GroupBox {
            VStack(alignment: .leading, spacing: 10) {
                // Header
                HStack(spacing: 10) {
                    Circle()
                        .fill(tagColor)
                        .frame(width: 12, height: 12)
                    Text(workspace.name)
                        .font(.title3.bold())
                    Spacer()
                    HStack(spacing: 6) {
                        Circle()
                            .fill(manager.isConnected ? Color.green : Color.red)
                            .frame(width: 10, height: 10)
                        Text(manager.isConnected ? "Ready" : "Offline")
                            .font(.callout)
                            .foregroundStyle(.secondary)
                    }
                }

                Divider()

                // Info
                HStack {
                    Label("\(workspace.dataLakePaths.count) source folder\(workspace.dataLakePaths.count == 1 ? "" : "s")", systemImage: "folder.fill")
                        .font(.callout)
                    Spacer()
                    Label("\(manager.vectorCount) indexed", systemImage: "magnifyingglass")
                        .font(.callout)
                }
                .foregroundStyle(.secondary)

                // Processing status
                if manager.isIngesting {
                    HStack(spacing: 8) {
                        ProgressView()
                            .controlSize(.small)
                        Text(humanStage)
                            .font(.callout.bold())
                            .foregroundStyle(.orange)
                        Spacer()
                        Text("\(manager.chunkCount) chunks")
                            .font(.callout.monospacedDigit())
                            .foregroundStyle(.secondary)
                            .contentTransition(.numericText())
                            .animation(.easeInOut(duration: 0.3), value: manager.chunkCount)
                    }
                } else if manager.isConnected {
                    Text("Ready to search and analyze")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                }

                Divider()

                // Actions — clean, clear, no jargon
                HStack(spacing: 12) {
                    if manager.isConnected {
                        Button {
                            manager.stopDaemon()
                        } label: {
                            Label("Shut Down", systemImage: "power")
                                .font(.callout)
                        }
                        .buttonStyle(.bordered)
                        .tint(.red)
                    } else {
                        Button {
                            manager.launchDaemon()
                        } label: {
                            Label("Start Up", systemImage: "power")
                                .font(.callout)
                        }
                        .buttonStyle(.bordered)
                        .tint(.green)
                    }

                    Spacer()

                    Button(role: .destructive) {
                        showDeleteConfirm = true
                    } label: {
                        Image(systemName: "trash")
                            .font(.callout)
                    }
                    .buttonStyle(.borderless)

                    Button {
                        onOpen()
                    } label: {
                        Label("Open Workspace", systemImage: "rectangle.expand.vertical")
                            .font(.callout)
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.regular)
                    .help("Open this workspace in a full window")
                }
            }
            .padding(8)
        }
        .confirmationDialog("Delete \"\(workspace.name)\"?", isPresented: $showDeleteConfirm) {
            Button("Delete", role: .destructive) { onDelete() }
        } message: {
            Text("The workspace configuration will be removed. Data files are preserved on disk.")
        }
    }
}
