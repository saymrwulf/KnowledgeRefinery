import Foundation
import SwiftUI

@MainActor
class DaemonManager: ObservableObject {
    @Published var isConnected = false
    @Published var isLMStudioAvailable = false
    @Published var vectorCount = 0
    @Published var statusMessage = "Starting..."
    @Published var chatModel: String?
    @Published var embeddingModel: String?
    @Published var contextLength: Int?
    @Published var watchedVolumes: [String] = []
    @Published var logLines: [String] = []
    @Published var isDaemonRunning = false

    // Ingest polling state
    @Published var ingestStatus: IngestStatusResponse?
    @Published var isIngesting = false
    @Published var activityLog: [ActivityLogEntry] = []
    @Published var chunkCount = 0
    @Published var annotationCount = 0
    @Published var conceptCount = 0
    @Published var edgeCount = 0

    let client: DaemonClient
    let port: Int
    let dataDir: String?

    private var healthTimer: Timer?
    private var ingestTimer: Timer?
    private var daemonProcess: Process?
    private var healthFailCount = 0
    private var autoRestartCount = 0
    private let maxAutoRestarts = 3

    /// Default init — uses port 8742 and global data dir.
    init() {
        self.port = 8742
        self.dataDir = nil
        self.client = DaemonClient()
    }

    /// Workspace-aware init — custom port and data directory.
    init(port: Int, dataDir: String) {
        self.port = port
        self.dataDir = dataDir
        self.client = DaemonClient(
            baseURL: URL(string: "http://127.0.0.1:\(port)")!
        )
    }

    // MARK: - Connection

    func connect() {
        Task {
            await checkHealth()
            startHealthPolling()
        }
    }

    func disconnect() {
        healthTimer?.invalidate()
        healthTimer = nil
        isConnected = false
        statusMessage = "Disconnected"
    }

    private func startHealthPolling() {
        healthTimer?.invalidate()
        healthTimer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                await self?.checkHealth()
            }
        }
    }

    func checkHealth() async {
        do {
            let health = try await client.healthCheck()
            isConnected = true
            isDaemonRunning = true
            healthFailCount = 0
            isLMStudioAvailable = health.lm_studio == "connected"
            vectorCount = health.vector_count
            chatModel = health.chat_model
            embeddingModel = health.embedding_model
            contextLength = health.context_length
            watchedVolumes = health.watched_volumes ?? []
            statusMessage = "Connected | LM Studio: \(health.lm_studio) | Vectors: \(health.vector_count)"
        } catch {
            isConnected = false
            healthFailCount += 1

            if healthFailCount >= 3 && daemonProcess != nil && autoRestartCount < maxAutoRestarts {
                // We launched this daemon and it crashed — auto-restart
                autoRestartCount += 1
                statusMessage = "Daemon crashed — restarting (\(autoRestartCount)/\(maxAutoRestarts))..."
                restartDaemon()
            } else if healthFailCount >= 3 && autoRestartCount >= maxAutoRestarts {
                isDaemonRunning = false
                statusMessage = "Daemon failed — restart limit reached"
            } else if healthFailCount >= 3 {
                isDaemonRunning = false
                statusMessage = "Daemon not running"
            } else {
                statusMessage = "Connecting..."
            }
        }
    }

    // MARK: - PID Detection

    /// Check if a daemon is already running for this workspace by reading its PID file.
    func detectRunningDaemon() -> Bool {
        guard let dataDir = dataDir else { return false }
        let pidPath = URL(fileURLWithPath: dataDir).appendingPathComponent("daemon.pid").path

        guard FileManager.default.fileExists(atPath: pidPath),
              let pidString = try? String(contentsOfFile: pidPath, encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines),
              let pid = Int32(pidString) else {
            return false
        }

        // signal 0 = check if process exists, no actual signal sent
        if kill(pid, 0) == 0 {
            isDaemonRunning = true
            return true
        } else {
            // Stale PID file — daemon is dead, clean up
            try? FileManager.default.removeItem(atPath: pidPath)
            return false
        }
    }

    // MARK: - Daemon Process Management

    func launchDaemon() {
        // Already have a running process handle
        guard daemonProcess == nil else {
            connect()
            return
        }

        // Check if daemon is already running (from previous app session or external launch)
        if detectRunningDaemon() {
            statusMessage = "Found running daemon on port \(port)"
            connect()
            return
        }

        let goBinary = findGoBinary()
        guard let binaryURL = goBinary else {
            statusMessage = "Cannot find daemon binary"
            return
        }

        let process = Process()
        process.executableURL = binaryURL
        process.arguments = []
        process.currentDirectoryURL = binaryURL.deletingLastPathComponent()

        var env = ProcessInfo.processInfo.environment
        if let dataDir = dataDir {
            env["KR_DATA_DIR"] = dataDir
        }
        env["KR_PORT"] = String(port)
        process.environment = env

        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = pipe

        pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty,
                  let line = String(data: data, encoding: .utf8) else { return }
            Task { @MainActor [weak self] in
                let trimmed = line.trimmingCharacters(in: .newlines)
                if !trimmed.isEmpty {
                    self?.logLines.append(trimmed)
                    if (self?.logLines.count ?? 0) > 500 {
                        self?.logLines.removeFirst((self?.logLines.count ?? 500) - 500)
                    }
                }
            }
        }

        do {
            try process.run()
            daemonProcess = process
            isDaemonRunning = true
            statusMessage = "Daemon starting on port \(port)..."

            Task {
                try? await Task.sleep(for: .seconds(3))
                connect()
            }
        } catch {
            statusMessage = "Failed to launch daemon: \(error.localizedDescription)"
        }
    }

    func stopDaemon() {
        daemonProcess?.terminate()
        daemonProcess = nil
        isDaemonRunning = false
        autoRestartCount = 0
        disconnect()
    }

    func restartDaemon() {
        daemonProcess?.terminate()
        daemonProcess = nil
        Task {
            try? await Task.sleep(for: .seconds(1))
            launchDaemon()
        }
    }

    // MARK: - Ingest

    func startIngest() {
        Task {
            do {
                let response = try await client.startIngest()
                statusMessage = "Ingest started: \(response.job_id)"
                startIngestPolling()
            } catch {
                statusMessage = "Ingest failed: \(error.localizedDescription)"
            }
        }
    }

    func startIngestPolling() {
        isIngesting = true
        ingestTimer?.invalidate()
        Task { await pollIngestStatus() }
        ingestTimer = Timer.scheduledTimer(withTimeInterval: 1.5, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                await self?.pollIngestStatus()
            }
        }
    }

    func stopIngestPolling() {
        ingestTimer?.invalidate()
        ingestTimer = nil
        isIngesting = false
    }

    private func pollIngestStatus() async {
        do {
            let status = try await client.ingestStatus()
            ingestStatus = status
            vectorCount = status.vector_count ?? vectorCount
            chunkCount = status.chunk_count ?? chunkCount
            annotationCount = status.annotation_count ?? annotationCount
            conceptCount = status.concept_count ?? conceptCount
            edgeCount = status.edge_count ?? edgeCount

            if let log = status.activity_log {
                activityLog = log
            }

            if !status.running {
                let stage = status.currentStage
                if stage == "completed" || stage == nil {
                    stopIngestPolling()
                    statusMessage = "Pipeline completed"
                }
            }
        } catch {
            // Silently continue — daemon may be busy
        }
    }

    private func findGoBinary() -> URL? {
        let binaryName = "knowledge-refinery-daemon"
        var candidates: [URL] = []

        if let envDir = ProcessInfo.processInfo.environment["KR_DAEMON_DIR"] {
            candidates.append(URL(fileURLWithPath: envDir).appendingPathComponent(binaryName))
        }

        if let resourceURL = Bundle.main.resourceURL {
            candidates.append(resourceURL.appendingPathComponent(binaryName))
        }

        // Development: daemon-go directory relative to project root
        let projectRoot = Bundle.main.bundleURL
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        candidates.append(projectRoot.appendingPathComponent("daemon-go/\(binaryName)"))

        // Hardcoded development fallback
        candidates.append(
            FileManager.default.homeDirectoryForCurrentUser
                .appendingPathComponent("GitClone/ClaudeCodeProjects/LongLocalTimeHorizonInfoRetrieval/daemon-go/\(binaryName)")
        )

        for url in candidates {
            if FileManager.default.fileExists(atPath: url.path) {
                return url
            }
        }
        return nil
    }
}
