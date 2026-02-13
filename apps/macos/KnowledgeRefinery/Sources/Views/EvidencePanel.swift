import SwiftUI
import QuickLook

struct EvidencePanel: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var assets: [AssetInfo] = []
    @State private var selectedAsset: AssetInfo?
    @State private var previewURL: URL?
    @State private var isLoading = false

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Evidence Browser")
                    .font(.headline)
                Spacer()
                Button("Refresh") { loadAssets() }
                    .buttonStyle(.bordered)
                    .controlSize(.small)
            }
            .padding()
            .background(.bar)

            Divider()

            if assets.isEmpty && !isLoading {
                VStack(spacing: 12) {
                    Image(systemName: "doc.text.magnifyingglass")
                        .font(.system(size: 48))
                        .foregroundStyle(.tertiary)
                    Text("No assets ingested yet")
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List(assets, selection: $selectedAsset) { asset in
                    HStack {
                        Image(systemName: iconForMime(asset.mime_type))
                            .foregroundStyle(.blue)
                        VStack(alignment: .leading) {
                            Text(asset.filename)
                                .font(.body)
                            Text(formatBytes(asset.size_bytes))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        Spacer()
                        statusBadge(asset.status)
                    }
                    .tag(asset)
                    .onTapGesture(count: 2) {
                        openQuickLook(path: asset.path)
                    }
                }
                .listStyle(.inset)
                .onChange(of: selectedAsset) { _, newVal in
                    if let asset = newVal {
                        openQuickLook(path: asset.path)
                    }
                }
            }
        }
        .quickLookPreview($previewURL)
        .onAppear { loadAssets() }
    }

    private func loadAssets() {
        guard daemon.isConnected else { return }
        isLoading = true

        Task {
            do {
                let items = try await daemon.client.listAssets()
                await MainActor.run {
                    assets = items
                    isLoading = false
                }
            } catch {
                await MainActor.run { isLoading = false }
            }
        }
    }

    private func openQuickLook(path: String) {
        let url = URL(fileURLWithPath: path)
        if FileManager.default.fileExists(atPath: url.path) {
            previewURL = url
        }
    }

    private func statusBadge(_ status: String) -> some View {
        Text(status)
            .font(.caption2)
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .background(colorForStatus(status).opacity(0.2))
            .foregroundStyle(colorForStatus(status))
            .clipShape(Capsule())
    }

    private func colorForStatus(_ status: String) -> Color {
        switch status {
        case "embedded": return .green
        case "chunked": return .blue
        case "extracted": return .cyan
        case "pending": return .orange
        case "error": return .red
        default: return .gray
        }
    }

    private func iconForMime(_ mime: String?) -> String {
        guard let mime = mime else { return "doc" }
        if mime.contains("pdf") { return "doc.richtext" }
        if mime.contains("text") { return "doc.text" }
        if mime.contains("image") { return "photo" }
        if mime.contains("html") { return "globe" }
        return "doc"
    }

    private func formatBytes(_ bytes: Int) -> String {
        let formatter = ByteCountFormatter()
        formatter.allowedUnits = [.useAll]
        formatter.countStyle = .file
        return formatter.string(fromByteCount: Int64(bytes))
    }
}
