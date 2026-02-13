import Foundation

/// Monitors LM Studio directly (independent of any daemon) by polling /v1/models.
@MainActor
class LMStudioMonitor: ObservableObject {
    @Published var isAlive = false
    @Published var chatModel: String?
    @Published var embeddingModel: String?
    @Published var allModels: [String] = []
    @Published var contextLength: Int?

    private var timer: Timer?
    private let session: URLSession
    private let baseURL = URL(string: "http://127.0.0.1:1234/v1")!
    private let nativeURL = URL(string: "http://127.0.0.1:1234/api/v0")!

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 5
        self.session = URLSession(configuration: config)
    }

    func startPolling() {
        poll()
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                self?.poll()
            }
        }
    }

    func stopPolling() {
        timer?.invalidate()
        timer = nil
    }

    private func poll() {
        Task {
            do {
                let url = baseURL.appendingPathComponent("models")
                let (data, response) = try await session.data(from: url)
                guard let http = response as? HTTPURLResponse,
                      (200...299).contains(http.statusCode) else {
                    markOffline()
                    return
                }
                let result = try JSONDecoder().decode(ModelsResponse.self, from: data)
                let ids = result.data.map(\.id)
                allModels = ids
                isAlive = true

                // Classify models
                chatModel = ids.first { id in
                    let low = id.lowercased()
                    return !["embed", "e5", "bge", "gte", "nomic", "whisper"].contains(where: { low.contains($0) })
                }
                embeddingModel = ids.first { id in
                    let low = id.lowercased()
                    return ["embed", "e5", "bge", "gte", "nomic"].contains(where: { low.contains($0) })
                }

                // Query native API for context window
                await fetchContextLength()
            } catch {
                markOffline()
            }
        }
    }

    private func fetchContextLength() async {
        do {
            let url = nativeURL.appendingPathComponent("models")
            let (data, response) = try await session.data(from: url)
            guard let http = response as? HTTPURLResponse,
                  (200...299).contains(http.statusCode) else { return }
            let result = try JSONDecoder().decode(NativeModelsResponse.self, from: data)
            // Find the LLM model's loaded context length
            for m in result.data {
                if m.type == "llm" {
                    contextLength = m.loaded_context_length ?? m.max_context_length
                    return
                }
            }
        } catch { }
    }

    private func markOffline() {
        isAlive = false
        chatModel = nil
        embeddingModel = nil
        allModels = []
        contextLength = nil
    }

    // MARK: - Repair Actions

    @Published var isRepairing = false
    @Published var repairMessage: String?

    /// Graceful relaunch: ask LM Studio to quit, wait, then reopen.
    func gracefulRelaunch() {
        isRepairing = true
        repairMessage = "Quitting LM Studio..."
        Task.detached {
            // Ask LM Studio to quit gracefully via AppleScript
            let quit = Process()
            quit.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
            quit.arguments = ["-e", "tell application \"LM Studio\" to quit"]
            try? quit.run()
            quit.waitUntilExit()

            // Wait for it to shut down
            try? await Task.sleep(for: .seconds(3))

            await MainActor.run { self.repairMessage = "Reopening LM Studio..." }

            // Reopen
            let open = Process()
            open.executableURL = URL(fileURLWithPath: "/usr/bin/open")
            open.arguments = ["-a", "LM Studio"]
            try? open.run()
            open.waitUntilExit()

            // Wait for it to start serving models
            try? await Task.sleep(for: .seconds(5))

            await MainActor.run {
                self.repairMessage = nil
                self.isRepairing = false
                self.poll()
            }
        }
    }

    /// Brutal relaunch: force-kill, clear cache dirs, then reopen.
    func brutalRelaunch() {
        isRepairing = true
        repairMessage = "Force-killing LM Studio..."
        Task.detached {
            // Force-kill all LM Studio processes
            let kill = Process()
            kill.executableURL = URL(fileURLWithPath: "/usr/bin/pkill")
            kill.arguments = ["-9", "-f", "LM Studio"]
            try? kill.run()
            kill.waitUntilExit()

            try? await Task.sleep(for: .seconds(2))

            await MainActor.run { self.repairMessage = "Clearing corrupted cache..." }

            // Clear known cache directories that get corrupted
            let fm = FileManager.default
            let home = fm.homeDirectoryForCurrentUser.path
            let cacheDirs = [
                "\(home)/.cache/lm-studio/tmp",
                "\(home)/Library/Application Support/LM Studio/Cache",
            ]
            for dir in cacheDirs {
                if fm.fileExists(atPath: dir) {
                    try? fm.removeItem(atPath: dir)
                }
            }

            try? await Task.sleep(for: .seconds(1))

            await MainActor.run { self.repairMessage = "Reopening LM Studio..." }

            // Reopen
            let open = Process()
            open.executableURL = URL(fileURLWithPath: "/usr/bin/open")
            open.arguments = ["-a", "LM Studio"]
            try? open.run()
            open.waitUntilExit()

            // Wait longer — models need to reload after cache clear
            try? await Task.sleep(for: .seconds(8))

            await MainActor.run {
                self.repairMessage = nil
                self.isRepairing = false
                self.poll()
            }
        }
    }
}

// MARK: - LM Studio API Responses

private struct ModelsResponse: Codable {
    let data: [ModelEntry]

    struct ModelEntry: Codable {
        let id: String
    }
}

private struct NativeModelsResponse: Codable {
    let data: [NativeModelEntry]

    struct NativeModelEntry: Codable {
        let id: String
        let type: String?
        let max_context_length: Int?
        let loaded_context_length: Int?
    }
}
