import SwiftUI

struct VolumeManagerView: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var volumes: [VolumeResponse] = []
    @State private var isLoading = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                Button("Add Folder...") {
                    addFolder()
                }
                .buttonStyle(.borderedProminent)

                Button("Refresh") {
                    loadVolumes()
                }
                .buttonStyle(.bordered)

                Spacer()

                if isLoading {
                    ProgressView()
                        .controlSize(.small)
                }
            }
            .padding(.horizontal)
            .padding(.top)

            if let error = errorMessage {
                Text(error)
                    .foregroundStyle(.red)
                    .font(.caption)
                    .padding(.horizontal)
            }

            if volumes.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "folder.badge.plus")
                        .font(.system(size: 48))
                        .foregroundStyle(.tertiary)
                    Text("No volumes added yet")
                        .font(.title2)
                        .foregroundStyle(.secondary)
                    Text("Add a folder to start ingesting documents")
                        .foregroundStyle(.tertiary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List(volumes) { vol in
                    HStack {
                        Image(systemName: "folder.fill")
                            .foregroundStyle(.blue)
                        VStack(alignment: .leading) {
                            Text(vol.label ?? vol.path)
                                .font(.headline)
                            Text(vol.path)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        Spacer()
                        if let scanTime = vol.last_scan_at {
                            Text("Scanned: \(scanTime)")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                        }
                    }
                    .padding(.vertical, 2)
                }
                .listStyle(.inset)
            }
        }
        .navigationTitle("Volumes")
        .onAppear { loadVolumes() }
    }

    private func addFolder() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.message = "Select a folder to add to the Knowledge Refinery"

        if panel.runModal() == .OK, let url = panel.url {
            Task {
                do {
                    _ = try await daemon.client.addVolume(path: url.path)
                    loadVolumes()
                } catch {
                    await MainActor.run {
                        errorMessage = error.localizedDescription
                    }
                }
            }
        }
    }

    private func loadVolumes() {
        guard daemon.isConnected else { return }
        isLoading = true

        Task {
            do {
                let vols = try await daemon.client.listVolumes()
                await MainActor.run {
                    volumes = vols
                    isLoading = false
                }
            } catch {
                await MainActor.run {
                    errorMessage = error.localizedDescription
                    isLoading = false
                }
            }
        }
    }
}
