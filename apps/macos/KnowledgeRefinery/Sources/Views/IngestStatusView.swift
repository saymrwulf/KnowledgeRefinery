import SwiftUI

struct IngestStatusView: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var status: IngestStatusResponse?
    @State private var isRefreshing = false
    @State private var errorMessage: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                // Controls
                HStack {
                    Button("Start Ingestion") {
                        startIngest()
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(!daemon.isConnected)

                    Button("Refresh Status") {
                        refreshStatus()
                    }
                    .buttonStyle(.bordered)

                    Spacer()

                    if isRefreshing {
                        ProgressView()
                            .controlSize(.small)
                    }
                }

                if let error = errorMessage {
                    Text(error)
                        .foregroundStyle(.red)
                        .font(.caption)
                }

                if let status = status {
                    // Pipeline status
                    GroupBox("Pipeline") {
                        VStack(alignment: .leading, spacing: 8) {
                            HStack {
                                Circle()
                                    .fill(status.running ? Color.green : Color.gray)
                                    .frame(width: 10, height: 10)
                                Text(status.running ? "Running" : "Idle")
                                    .font(.headline)
                            }

                            LabeledContent("Total Assets", value: "\(status.total_assets)")
                            LabeledContent("Vectors", value: "\(status.vector_count ?? 0)")

                            if let job = status.latest_job {
                                Divider()
                                LabeledContent("Latest Job", value: job.job_id ?? "none")
                                LabeledContent("Job Status", value: job.status ?? "unknown")
                                if let progress = job.progress {
                                    LabeledContent("Stage", value: progress.stage ?? "unknown")
                                }
                            }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                    }

                    // Status breakdown
                    if let counts = status.status_counts, !counts.isEmpty {
                        GroupBox("Asset Status Breakdown") {
                            VStack(alignment: .leading, spacing: 4) {
                                ForEach(counts.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                                    HStack {
                                        Text(key.capitalized)
                                            .frame(width: 100, alignment: .leading)
                                        ProgressView(value: Double(value), total: Double(max(status.total_assets, 1)))
                                        Text("\(value)")
                                            .foregroundStyle(.secondary)
                                            .frame(width: 50, alignment: .trailing)
                                    }
                                }
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                        }
                    }
                } else {
                    Text("Click 'Refresh Status' to check pipeline status")
                        .foregroundStyle(.secondary)
                        .frame(maxWidth: .infinity, alignment: .center)
                        .padding(.top, 40)
                }
            }
            .padding()
        }
        .navigationTitle("Ingestion")
        .onAppear { refreshStatus() }
    }

    private func startIngest() {
        guard daemon.isConnected else { return }
        errorMessage = nil

        Task {
            do {
                _ = try await daemon.client.startIngest()
                await MainActor.run {
                    errorMessage = nil
                }
                // Wait a bit then refresh
                try? await Task.sleep(for: .seconds(1))
                refreshStatus()
            } catch {
                await MainActor.run {
                    errorMessage = error.localizedDescription
                }
            }
        }
    }

    private func refreshStatus() {
        guard daemon.isConnected else { return }
        isRefreshing = true

        Task {
            do {
                let s = try await daemon.client.ingestStatus()
                await MainActor.run {
                    status = s
                    isRefreshing = false
                }
            } catch {
                await MainActor.run {
                    errorMessage = error.localizedDescription
                    isRefreshing = false
                }
            }
        }
    }
}
