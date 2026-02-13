import SwiftUI

struct AssetsView: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var assets: [AssetInfo] = []
    @State private var isLoading = false

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("\(assets.count) assets")
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
                Text("No assets ingested yet")
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                Table(assets) {
                    TableColumn("Filename", value: \.filename)
                    TableColumn("Type") { asset in
                        Text(asset.mime_type ?? "unknown")
                            .foregroundStyle(.secondary)
                    }
                    .width(min: 100, ideal: 150)
                    TableColumn("Size") { asset in
                        Text(ByteCountFormatter.string(fromByteCount: Int64(asset.size_bytes), countStyle: .file))
                    }
                    .width(min: 60, ideal: 80)
                    TableColumn("Status") { asset in
                        Text(asset.status)
                            .font(.caption)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(statusColor(asset.status).opacity(0.15))
                            .foregroundStyle(statusColor(asset.status))
                            .clipShape(Capsule())
                    }
                    .width(min: 80, ideal: 100)
                }
            }
        }
        .navigationTitle("Assets")
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

    private func statusColor(_ status: String) -> Color {
        switch status {
        case "embedded": return .green
        case "chunked": return .blue
        case "extracted": return .cyan
        case "pending": return .orange
        case "error": return .red
        default: return .gray
        }
    }
}
