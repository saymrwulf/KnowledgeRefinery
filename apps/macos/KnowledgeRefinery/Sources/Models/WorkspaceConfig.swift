import Foundation

// MARK: - Workspace Model

struct Workspace: Codable, Identifiable, Hashable, Equatable {
    let id: String
    var name: String
    var port: Int
    var colorTag: String
    var dataLakePaths: [String]
    let createdAt: Date

    var dataDirURL: URL {
        FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".knowledge-refinery/workspaces/\(id)")
    }

    var dataDirPath: String { dataDirURL.path }

    static func == (lhs: Workspace, rhs: Workspace) -> Bool {
        lhs.id == rhs.id
    }

    func hash(into hasher: inout Hasher) {
        hasher.combine(id)
    }
}

// MARK: - Persistence Envelope

private struct WorkspacesFile: Codable {
    var workspaces: [Workspace]
}

// MARK: - Workspace Store

@MainActor
class WorkspaceStore: ObservableObject {
    @Published var workspaces: [Workspace] = []

    /// Daemon managers keyed by workspace ID. NOT @Published — individual managers
    /// are observed by views via @ObservedObject directly.
    private(set) var daemonManagers: [String: DaemonManager] = [:]

    private static let baseDir = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent(".knowledge-refinery")
    private static let filePath = baseDir.appendingPathComponent("workspaces.json")
    private static let workspacesDir = baseDir.appendingPathComponent("workspaces")
    private static let basePort = 8742

    init() {
        load()
    }

    // MARK: - Daemon Manager Access

    /// Returns the DaemonManager for a workspace. Always returns the same instance
    /// for the same workspace ID — safe to call from view body.
    func managerFor(_ workspace: Workspace) -> DaemonManager {
        if let existing = daemonManagers[workspace.id] {
            return existing
        }
        let mgr = DaemonManager(port: workspace.port, dataDir: workspace.dataDirPath)
        mgr.connect()  // Start health polling to detect already-running daemons
        daemonManagers[workspace.id] = mgr
        return mgr
    }

    func startAll() {
        for ws in workspaces {
            let mgr = managerFor(ws)
            if !mgr.isDaemonRunning {
                mgr.launchDaemon()
                // Auto-start ingestion after daemon connects
                Task {
                    // Wait for daemon to be ready
                    try? await Task.sleep(for: .seconds(5))
                    if mgr.isConnected {
                        mgr.startIngest()
                    }
                }
            } else if mgr.isConnected && !mgr.isIngesting {
                mgr.startIngest()
            }
        }
    }

    // MARK: - CRUD

    func createWorkspace(name: String, colorTag: String, dataLakePaths: [String]) -> Workspace {
        let ws = Workspace(
            id: "ws_\(UUID().uuidString.prefix(8).lowercased())",
            name: name,
            port: nextAvailablePort(),
            colorTag: colorTag,
            dataLakePaths: dataLakePaths,
            createdAt: Date()
        )
        workspaces.append(ws)
        // Pre-create daemon manager so it's ready when the card renders
        daemonManagers[ws.id] = DaemonManager(port: ws.port, dataDir: ws.dataDirPath)
        ensureDataDir(for: ws)
        save()
        return ws
    }

    func updateWorkspace(_ workspace: Workspace) {
        if let idx = workspaces.firstIndex(where: { $0.id == workspace.id }) {
            workspaces[idx] = workspace
            save()
        }
    }

    func deleteWorkspace(_ workspace: Workspace) {
        daemonManagers[workspace.id]?.stopDaemon()
        daemonManagers.removeValue(forKey: workspace.id)
        workspaces.removeAll { $0.id == workspace.id }
        save()
    }

    // MARK: - Port Assignment

    private func nextAvailablePort() -> Int {
        let usedPorts = Set(workspaces.map(\.port))
        var port = Self.basePort
        while usedPorts.contains(port) {
            port += 1
        }
        return port
    }

    // MARK: - Persistence

    func load() {
        let fm = FileManager.default
        guard fm.fileExists(atPath: Self.filePath.path) else { return }
        do {
            let data = try Data(contentsOf: Self.filePath)
            let decoder = JSONDecoder()
            decoder.dateDecodingStrategy = .iso8601
            let file = try decoder.decode(WorkspacesFile.self, from: data)
            workspaces = file.workspaces
            // Pre-create daemon managers for all loaded workspaces
            for ws in workspaces {
                daemonManagers[ws.id] = DaemonManager(port: ws.port, dataDir: ws.dataDirPath)
            }
        } catch {
            print("Failed to load workspaces.json: \(error)")
        }
    }

    func save() {
        let fm = FileManager.default
        do {
            try fm.createDirectory(at: Self.baseDir, withIntermediateDirectories: true)
            let encoder = JSONEncoder()
            encoder.dateEncodingStrategy = .iso8601
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
            let file = WorkspacesFile(workspaces: workspaces)
            let data = try encoder.encode(file)
            try data.write(to: Self.filePath, options: .atomic)
        } catch {
            print("Failed to save workspaces.json: \(error)")
        }
    }

    private func ensureDataDir(for workspace: Workspace) {
        let fm = FileManager.default
        let dir = workspace.dataDirURL
        try? fm.createDirectory(at: dir, withIntermediateDirectories: true)
        try? fm.createDirectory(at: dir.appendingPathComponent("vectors"), withIntermediateDirectories: true)
        try? fm.createDirectory(at: dir.appendingPathComponent("thumbnails"), withIntermediateDirectories: true)
        try? fm.createDirectory(at: dir.appendingPathComponent("tmp"), withIntermediateDirectories: true)
    }
}
