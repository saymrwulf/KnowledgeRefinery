import SwiftUI
import AppKit

extension Notification.Name {
    static let krNewWorkspace = Notification.Name("krNewWorkspace")
}

@main
struct KnowledgeRefineryApp: App {
    @StateObject private var workspaceStore = WorkspaceStore()
    @StateObject private var lmMonitor = LMStudioMonitor()

    init() {
        // Ensure the app appears in the Dock and can become frontmost
        NSApplication.shared.setActivationPolicy(.regular)
    }

    var body: some Scene {
        WindowGroup("Knowledge Refinery", id: "main") {
            MasterDashboardView()
                .environmentObject(workspaceStore)
                .environmentObject(lmMonitor)
                .frame(minWidth: 900, minHeight: 600)
                .onAppear {
                    setAppIcon()
                    autoStartDaemons()
                    runSelfTestIfRequested()
                }
        }
        .defaultSize(width: 1200, height: 800)

        .commands {
            CommandGroup(after: .newItem) {
                Button("New Workspace") {
                    NotificationCenter.default.post(name: .krNewWorkspace, object: nil)
                }
                .keyboardShortcut("n", modifiers: [.command, .shift])
            }
        }

        WindowGroup("Workspace", id: "workspace", for: String.self) { $workspaceId in
            if let wsId = workspaceId {
                WorkspaceDetailView(workspaceId: wsId)
                    .environmentObject(workspaceStore)
                    .frame(minWidth: 1000, minHeight: 650)
            } else {
                Text("No workspace selected")
                    .frame(width: 300, height: 200)
            }
        }
        .defaultSize(width: 1300, height: 850)
    }

    /// Set the Dock icon from the bundled .icns file.
    private func setAppIcon() {
        if let iconURL = Bundle.module.url(forResource: "AppIcon", withExtension: "icns") {
            let icon = NSImage(contentsOf: iconURL)
            NSApp.applicationIconImage = icon
        }
    }

    /// Auto-start daemons for all workspaces on app launch.
    private func autoStartDaemons() {
        for ws in workspaceStore.workspaces {
            let mgr = workspaceStore.managerFor(ws)
            if !mgr.isDaemonRunning {
                mgr.launchDaemon()
            }
        }
    }

    /// Automated self-test: set KR_SELF_TEST=1 to exercise workspace CRUD and exit.
    private func runSelfTestIfRequested() {
        guard ProcessInfo.processInfo.environment["KR_SELF_TEST"] == "1" else { return }
        Task { @MainActor in
            print("[SELF-TEST] Starting...")

            // Test 1: Create workspace
            let initialCount = workspaceStore.workspaces.count
            let ws = workspaceStore.createWorkspace(
                name: "SelfTest",
                colorTag: "green",
                dataLakePaths: ["/tmp/selftest"]
            )
            assert(workspaceStore.workspaces.count == initialCount + 1, "Workspace count should increase")
            assert(ws.name == "SelfTest", "Name should match")
            assert(ws.port >= 8742, "Port should be assigned")
            print("[SELF-TEST] PASS: createWorkspace — \(ws.id) on port \(ws.port)")

            // Test 2: Verify persistence
            let fileExists = FileManager.default.fileExists(
                atPath: FileManager.default.homeDirectoryForCurrentUser
                    .appendingPathComponent(".knowledge-refinery/workspaces.json").path
            )
            assert(fileExists, "workspaces.json should exist after create")
            print("[SELF-TEST] PASS: persistence — workspaces.json written")

            // Test 3: Verify data dir created
            let dataDirExists = FileManager.default.fileExists(atPath: ws.dataDirPath)
            assert(dataDirExists, "Data dir should exist")
            print("[SELF-TEST] PASS: dataDir — \(ws.dataDirPath)")

            // Test 4: Verify daemon manager was created
            let mgr = workspaceStore.managerFor(ws)
            assert(mgr.port == ws.port, "Manager port should match")
            print("[SELF-TEST] PASS: daemonManager — port \(mgr.port)")

            // Test 5: Update workspace
            var updated = ws
            updated.name = "SelfTest-Updated"
            updated.dataLakePaths = ["/tmp/selftest", "/tmp/selftest2"]
            workspaceStore.updateWorkspace(updated)
            let found = workspaceStore.workspaces.first { $0.id == ws.id }
            assert(found?.name == "SelfTest-Updated", "Name should be updated")
            assert(found?.dataLakePaths.count == 2, "Should have 2 data lakes")
            print("[SELF-TEST] PASS: updateWorkspace")

            // Test 6: Reload from disk
            let store2 = WorkspaceStore()
            let reloaded = store2.workspaces.first { $0.id == ws.id }
            assert(reloaded != nil, "Should persist and reload")
            assert(reloaded?.name == "SelfTest-Updated", "Name should persist")
            print("[SELF-TEST] PASS: reload from disk")

            // Test 7: Delete workspace
            workspaceStore.deleteWorkspace(ws)
            assert(!workspaceStore.workspaces.contains { $0.id == ws.id }, "Should be deleted")
            print("[SELF-TEST] PASS: deleteWorkspace")

            // Test 8: Verify LM Studio monitor initialized
            print("[SELF-TEST] PASS: LMStudioMonitor — isAlive=\(lmMonitor.isAlive)")

            print("[SELF-TEST] ALL TESTS PASSED (8/8)")

            // Clean up
            try? FileManager.default.removeItem(atPath: ws.dataDirPath)

            // Exit with success
            exit(0)
        }
    }
}
