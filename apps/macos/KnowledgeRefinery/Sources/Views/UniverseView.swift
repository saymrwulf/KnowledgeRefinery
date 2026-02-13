import SwiftUI
import WebKit

struct UniverseView: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var lod: String = "macro"
    @State private var snapshot: UniverseSnapshot?
    @State private var selectedNodeId: String?
    @State private var isLoading = false
    @State private var autoRefreshTimer: Timer?

    var body: some View {
        VStack(spacing: 0) {
            // Toolbar
            HStack {
                Text("Universe")
                    .font(.headline)

                Spacer()

                Picker("LOD", selection: $lod) {
                    Text("Macro").tag("macro")
                    Text("Mid").tag("mid")
                    Text("Near").tag("near")
                }
                .pickerStyle(.segmented)
                .frame(width: 200)

                Button("Refresh") { loadSnapshot() }
                    .buttonStyle(.bordered)
                    .controlSize(.small)

                if isLoading {
                    ProgressView()
                        .controlSize(.small)
                }
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(.bar)

            Divider()

            if let snapshot = snapshot, !snapshot.nodes.isEmpty {
                WebGPUUniverseView(
                    snapshot: snapshot,
                    onNodeSelected: { nodeId in
                        selectedNodeId = nodeId
                    },
                    useIncremental: daemon.isIngesting
                )
            } else {
                VStack(spacing: 12) {
                    Image(systemName: "globe")
                        .font(.system(size: 64))
                        .foregroundStyle(.tertiary)
                    Text("Universe Visualization")
                        .font(.title2)
                        .foregroundStyle(.secondary)
                    if snapshot?.nodes.isEmpty == true {
                        Text("No concepts yet. Run ingestion first.")
                            .foregroundStyle(.tertiary)
                    } else {
                        Text("Loading universe data...")
                            .foregroundStyle(.tertiary)
                    }
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }

            // Status bar
            if let s = snapshot {
                HStack {
                    Text("\(s.node_count) nodes")
                    Text("\(s.edge_count) edges")
                    Text("LOD: \(s.lod)")
                    Spacer()
                    if let nodeId = selectedNodeId {
                        Text("Selected: \(nodeId.prefix(8))...")
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.horizontal)
                .padding(.vertical, 4)
                .background(.bar)
            }
        }
        .onChange(of: lod) { _, _ in loadSnapshot() }
        .onChange(of: daemon.isIngesting) { _, ingesting in
            if ingesting {
                startAutoRefresh()
            } else {
                stopAutoRefresh()
                // Final full load when done
                loadSnapshot()
            }
        }
        .onAppear { loadSnapshot() }
        .onDisappear { stopAutoRefresh() }
    }

    private func loadSnapshot() {
        guard daemon.isConnected else {
            print("[Universe] Not connected, skipping snapshot load")
            return
        }
        isLoading = true
        Task {
            do {
                let s = try await daemon.client.universeSnapshot(lod: lod)
                await MainActor.run {
                    snapshot = s
                    isLoading = false
                    print("[Universe] Loaded snapshot: \(s.node_count) nodes, \(s.edge_count) edges")
                }
            } catch {
                print("[Universe] Failed to load snapshot: \(error)")
                await MainActor.run { isLoading = false }
            }
        }
    }

    private func startAutoRefresh() {
        stopAutoRefresh()
        autoRefreshTimer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { _ in
            Task { @MainActor [weak daemon] in
                guard let daemon = daemon, daemon.isConnected else { return }
                do {
                    let s = try await daemon.client.universeSnapshot(lod: "macro")
                    snapshot = s
                } catch {
                    // Silently continue
                }
            }
        }
    }

    private func stopAutoRefresh() {
        autoRefreshTimer?.invalidate()
        autoRefreshTimer = nil
    }
}

// MARK: - WKWebView wrapper for WebGPU rendering

struct WebGPUUniverseView: NSViewRepresentable {
    let snapshot: UniverseSnapshot
    let onNodeSelected: (String) -> Void
    var useIncremental: Bool = false

    func makeNSView(context: Context) -> WKWebView {
        let config = WKWebViewConfiguration()

        let webpagePrefs = WKWebpagePreferences()
        webpagePrefs.allowsContentJavaScript = true
        config.defaultWebpagePreferences = webpagePrefs

        let handler = context.coordinator
        config.userContentController.add(handler, name: "nodeSelected")
        config.userContentController.add(handler, name: "nodeHovered")
        config.userContentController.add(handler, name: "consoleLog")

        // Inject console.log capture script
        let consoleScript = WKUserScript(
            source: """
            (function() {
                var origLog = console.log;
                var origWarn = console.warn;
                var origError = console.error;
                console.log = function() {
                    origLog.apply(console, arguments);
                    window.webkit.messageHandlers.consoleLog.postMessage(
                        'LOG: ' + Array.from(arguments).join(' ')
                    );
                };
                console.warn = function() {
                    origWarn.apply(console, arguments);
                    window.webkit.messageHandlers.consoleLog.postMessage(
                        'WARN: ' + Array.from(arguments).join(' ')
                    );
                };
                console.error = function() {
                    origError.apply(console, arguments);
                    window.webkit.messageHandlers.consoleLog.postMessage(
                        'ERROR: ' + Array.from(arguments).join(' ')
                    );
                };
            })();
            """,
            injectionTime: .atDocumentStart,
            forMainFrameOnly: true
        )
        config.userContentController.addUserScript(consoleScript)

        let webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = context.coordinator

        // Load the universe HTML
        if let htmlURL = findUniverseHTML() {
            webView.loadFileURL(htmlURL, allowingReadAccessTo: htmlURL.deletingLastPathComponent())
        } else {
            // Fallback: load inline
            webView.loadHTMLString(fallbackHTML(), baseURL: nil)
        }

        return webView
    }

    func updateNSView(_ webView: WKWebView, context: Context) {
        context.coordinator.snapshot = snapshot
        context.coordinator.onNodeSelected = onNodeSelected
        injectData(into: webView, incremental: useIncremental)
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(snapshot: snapshot, onNodeSelected: onNodeSelected)
    }

    private func injectData(into webView: WKWebView, incremental: Bool = false) {
        guard let data = try? JSONEncoder().encode(snapshot),
              let json = String(data: data, encoding: .utf8) else { return }

        let fn = incremental ? "mergeUniverse" : "loadUniverse"
        let js = "if(window.\(fn)) window.\(fn)(\(json));"
        webView.evaluateJavaScript(js) { _, _ in }
    }

    private func findUniverseHTML() -> URL? {
        // SPM places resources in Bundle.module
        if let url = Bundle.module.url(forResource: "universe", withExtension: "html") {
            return url
        }

        // Fallback: main bundle
        if let url = Bundle.main.url(forResource: "universe", withExtension: "html") {
            return url
        }

        // Development fallback: source directory
        let srcDir = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("WebGPU/universe.html")
        if FileManager.default.fileExists(atPath: srcDir.path) {
            return srcDir
        }

        return nil
    }

    private func fallbackHTML() -> String {
        return """
        <!DOCTYPE html>
        <html>
        <head>
        <style>
        body { margin:0; background: #1a1a2e; color: #e0e0e0; font-family: system-ui;
               display: flex; align-items: center; justify-content: center; height: 100vh; }
        .container { text-align: center; }
        .nodes { display: flex; flex-wrap: wrap; gap: 12px; justify-content: center; padding: 20px; max-width: 800px; }
        .node { padding: 8px 16px; border-radius: 20px; cursor: pointer; transition: transform 0.2s; }
        .node:hover { transform: scale(1.1); }
        .concept { font-size: 16px; font-weight: 600; }
        .chunk { font-size: 12px; opacity: 0.7; }
        h2 { color: #7b68ee; }
        </style>
        </head>
        <body>
        <div class="container">
            <h2>Concept Universe</h2>
            <p>WebGPU renderer loading...</p>
            <div id="nodes" class="nodes"></div>
        </div>
        <script>
        window.loadUniverse = function(data) {
            const container = document.getElementById('nodes');
            container.innerHTML = '';
            if (!data || !data.nodes) return;
            data.nodes.forEach(node => {
                const el = document.createElement('div');
                el.className = 'node ' + node.type;
                el.style.background = node.color || 'hsl(200,50%,30%)';
                el.textContent = node.label;
                el.onclick = () => {
                    window.webkit?.messageHandlers?.nodeSelected?.postMessage(node.id);
                };
                container.appendChild(el);
            });
            document.querySelector('p').textContent =
                data.node_count + ' nodes, ' + data.edge_count + ' edges';
        };
        </script>
        </body>
        </html>
        """
    }

    class Coordinator: NSObject, WKNavigationDelegate, WKScriptMessageHandler {
        var snapshot: UniverseSnapshot
        var onNodeSelected: (String) -> Void

        init(snapshot: UniverseSnapshot, onNodeSelected: @escaping (String) -> Void) {
            self.snapshot = snapshot
            self.onNodeSelected = onNodeSelected
        }

        func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
            if message.name == "nodeSelected", let nodeId = message.body as? String {
                Task { @MainActor in
                    onNodeSelected(nodeId)
                }
            } else if message.name == "consoleLog", let msg = message.body as? String {
                let logLine = "[Universe JS] \(msg)\n"
                let logPath = "/tmp/kr-universe.log"
                if let handle = FileHandle(forWritingAtPath: logPath) {
                    handle.seekToEndOfFile()
                    handle.write(logLine.data(using: .utf8) ?? Data())
                    handle.closeFile()
                } else {
                    FileManager.default.createFile(atPath: logPath, contents: logLine.data(using: .utf8))
                }
            }
        }

        func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
            // First check WebGPU availability
            let diagJS = """
            (function() {
                var info = {
                    hasGPU: !!navigator.gpu,
                    hasLoadUniverse: !!window.loadUniverse,
                    fallbackVisible: document.getElementById('fallback')?.style.display,
                    canvasVisible: document.getElementById('gpu-canvas')?.style.display,
                    url: window.location.href
                };
                return JSON.stringify(info);
            })()
            """
            webView.evaluateJavaScript(diagJS) { result, error in
                let msg = "[Universe] Diagnostics: \(result ?? "nil") error: \(error?.localizedDescription ?? "none")"
                print(msg)
                Self.appendLog(msg)
            }

            // Inject data after JS init completes
            DispatchQueue.main.asyncAfter(deadline: .now() + 2.0) {
                guard let data = try? JSONEncoder().encode(self.snapshot),
                      let json = String(data: data, encoding: .utf8) else {
                    print("[Universe] Failed to encode snapshot")
                    return
                }
                let js = "if(window.loadUniverse) { window.loadUniverse(\(json)); 'ok'; } else { 'loadUniverse not found'; }"
                webView.evaluateJavaScript(js) { result, error in
                    let msg: String
                    if let error = error {
                        msg = "[Universe] JS inject error: \(error)"
                    } else {
                        msg = "[Universe] JS inject result: \(result ?? "nil")"
                    }
                    print(msg)
                    Self.appendLog(msg)
                }
            }
        }

        func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
            print("[Universe] Navigation failed: \(error)")
            Self.appendLog("[Universe] Navigation failed: \(error)")
        }

        static func appendLog(_ msg: String) {
            let logPath = "/tmp/kr-universe.log"
            let line = "\(msg)\n"
            if let handle = FileHandle(forWritingAtPath: logPath) {
                handle.seekToEndOfFile()
                handle.write(line.data(using: .utf8) ?? Data())
                handle.closeFile()
            } else {
                FileManager.default.createFile(atPath: logPath, contents: line.data(using: .utf8))
            }
        }
    }
}
