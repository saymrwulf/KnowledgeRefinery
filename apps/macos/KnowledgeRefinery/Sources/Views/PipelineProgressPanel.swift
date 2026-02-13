import SwiftUI

/// Live pipeline progress panel showing stage tracker, counters, interactions, and activity log.
struct PipelineProgressPanel: View {
    @ObservedObject var daemon: DaemonManager

    private var currentStage: String {
        daemon.ingestStatus?.currentStage ?? "idle"
    }

    private let stages = ["scanning", "extracting", "chunking", "embedding", "annotating", "conceptualizing"]

    /// Human-readable stage names for display
    private func humanStageName(_ stage: String) -> String {
        switch stage {
        case "scanning": return "Discovering Files"
        case "extracting": return "Reading Content"
        case "chunking": return "Splitting Text"
        case "embedding": return "Building Index"
        case "annotating": return "Analyzing Content"
        case "conceptualizing": return "Finding Themes"
        default: return stage.capitalized
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Process Documents button + status
            HStack {
                Button {
                    daemon.startIngest()
                } label: {
                    Label("Process Documents", systemImage: "play.fill")
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.regular)
                .disabled(daemon.isIngesting || !daemon.isConnected)
                .help("Read all source folders, analyze documents, and build a searchable knowledge base")

                if daemon.isIngesting {
                    ProgressView()
                        .controlSize(.small)
                    Text(humanStageName(currentStage))
                        .font(.callout.bold())
                        .foregroundStyle(.orange)
                } else if currentStage == "completed" {
                    Image(systemName: "checkmark.circle.fill")
                        .foregroundStyle(.green)
                        .font(.body)
                    Text("Complete")
                        .font(.callout.bold())
                        .foregroundStyle(.green)
                }
                Spacer()
            }

            Divider()

            // Stage Tracker
            stageTracker

            Divider()

            // Live Counters
            countersGrid

            Divider()

            // Interaction Indicators
            interactionIndicators

            Divider()

            // Activity Log
            activityLogView
        }
        .padding(12)
    }

    // MARK: - Stage Tracker

    private var stageTracker: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("PROCESSING STEPS")
                .font(.caption.bold())
                .foregroundStyle(.secondary)

            ForEach(stages, id: \.self) { stage in
                stageRow(stage)
            }
        }
    }

    @ViewBuilder
    private func stageRow(_ stage: String) -> some View {
        let state = stageState(stage)
        HStack(spacing: 8) {
            // Icon
            switch state {
            case .done:
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(.green)
                    .font(.callout)
            case .active:
                Image(systemName: "circle.fill")
                    .foregroundStyle(.orange)
                    .font(.callout)
                    .symbolEffect(.pulse)
            case .waiting:
                Image(systemName: "circle")
                    .foregroundStyle(.secondary)
                    .font(.callout)
            }

            // Label — human-readable
            Text(humanStageName(stage))
                .font(.callout)
                .foregroundStyle(state == .waiting ? .secondary : .primary)
                .frame(width: 130, alignment: .leading)

            // Detail
            stageDetail(stage, state: state)
                .font(.caption)
                .foregroundStyle(.secondary)

            Spacer()
        }
    }

    @ViewBuilder
    private func stageDetail(_ stage: String, state: StageState) -> some View {
        let live = daemon.ingestStatus?.live

        switch (stage, state) {
        case ("scanning", .active):
            if let scan = live?.scan {
                VStack(alignment: .leading, spacing: 1) {
                    Text("\(scan.done ?? 0)/\(scan.total ?? 0) paths")
                    if let path = scan.current_path, !path.isEmpty {
                        Text(path)
                            .foregroundStyle(.orange)
                            .lineLimit(1)
                            .truncationMode(.head)
                    }
                }
            }
        case ("extracting", .active):
            if let ext = live?.extract {
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 4) {
                        ProgressView(value: Double(ext.done ?? 0), total: max(Double(ext.total ?? 1), 1))
                            .frame(width: 60)
                        Text("\(ext.done ?? 0)/\(ext.total ?? 0)")
                    }
                    if let file = ext.current_file, !file.isEmpty {
                        Text(file)
                            .foregroundStyle(.orange)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
            }
        case ("chunking", .active):
            if let ch = live?.chunk {
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 4) {
                        ProgressView(value: Double(ch.done ?? 0), total: max(Double(ch.total ?? 1), 1))
                            .frame(width: 60)
                        Text("\(ch.chunks_created ?? 0) chunks")
                    }
                    if let file = ch.current_file, !file.isEmpty {
                        Text(file)
                            .foregroundStyle(.orange)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
            }
        case ("embedding", .active):
            if let em = live?.embed {
                HStack(spacing: 4) {
                    ProgressView(value: Double(em.embedded ?? 0), total: max(Double(em.total ?? 1), 1))
                        .frame(width: 60)
                    Text("\(em.embedded ?? 0)/\(em.total ?? 0) chunks")
                }
            }
        case ("annotating", .active):
            if let ann = live?.annotate {
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 4) {
                        ProgressView(value: Double(ann.done ?? 0), total: max(Double(ann.total ?? 1), 1))
                            .frame(width: 60)
                        Text("\(ann.done ?? 0)/\(ann.total ?? 0)")
                    }
                    if let file = ann.current_file, !file.isEmpty {
                        Text(file)
                            .foregroundStyle(.orange)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
            }
        case ("conceptualizing", .active):
            if let con = live?.conceptualize {
                Text(con.status ?? "building...")
            }
        case (_, .done):
            Image(systemName: "checkmark")
                .foregroundStyle(.green)
        default:
            Text("pending")
                .foregroundStyle(.tertiary)
        }
    }

    private enum StageState {
        case done, active, waiting
    }

    private func stageState(_ stage: String) -> StageState {
        guard daemon.isIngesting || currentStage == "completed" else {
            return .waiting
        }

        let current = currentStage
        guard let currentIdx = stages.firstIndex(of: current) else {
            // completed or unknown
            if current == "completed" { return .done }
            return .waiting
        }
        guard let stageIdx = stages.firstIndex(of: stage) else { return .waiting }

        if stageIdx < currentIdx { return .done }
        if stageIdx == currentIdx { return .active }
        return .waiting
    }

    // MARK: - Counters

    private var countersGrid: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("KNOWLEDGE BASE")
                .font(.caption.bold())
                .foregroundStyle(.secondary)

            LazyVGrid(columns: [
                GridItem(.flexible()),
                GridItem(.flexible()),
                GridItem(.flexible()),
            ], spacing: 8) {
                counterCell("Documents", value: daemon.ingestStatus?.total_assets ?? 0, icon: "doc")
                counterCell("Passages", value: daemon.chunkCount, icon: "text.quote")
                counterCell("Indexed", value: daemon.vectorCount, icon: "magnifyingglass")
                counterCell("Insights", value: daemon.annotationCount, icon: "tag")
                counterCell("Themes", value: daemon.conceptCount, icon: "brain")
                counterCell("Links", value: daemon.edgeCount, icon: "point.3.connected.trianglepath.dotted")
            }
        }
    }

    private func counterCell(_ label: String, value: Int, icon: String) -> some View {
        VStack(spacing: 3) {
            HStack(spacing: 4) {
                Image(systemName: icon)
                    .font(.caption)
                Text("\(value)")
                    .font(.body.bold().monospacedDigit())
                    .contentTransition(.numericText())
                    .animation(.easeInOut(duration: 0.3), value: value)
            }
            Text(label)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
    }

    // MARK: - Interaction Indicators

    private var interactionIndicators: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("CONNECTIONS")
                .font(.caption.bold())
                .foregroundStyle(.secondary)

            HStack(spacing: 20) {
                interactionBadge(
                    "Processing Engine",
                    active: daemon.isIngesting && daemon.isConnected,
                    detail: daemon.isConnected ? "Connected" : "Offline"
                )
                interactionBadge(
                    "AI Engine",
                    active: daemon.isIngesting && isLMStudioActive,
                    detail: isLMStudioActive ? humanStageName(currentStage) : "Idle"
                )
            }
        }
    }

    private var isLMStudioActive: Bool {
        let lmStages = ["embedding", "annotating", "conceptualizing"]
        return daemon.isIngesting && lmStages.contains(currentStage)
    }

    private func interactionBadge(_ label: String, active: Bool, detail: String) -> some View {
        HStack(spacing: 8) {
            Circle()
                .fill(active ? Color.green : Color.gray.opacity(0.4))
                .frame(width: 10, height: 10)
                .overlay {
                    if active {
                        Circle()
                            .fill(Color.green.opacity(0.3))
                            .frame(width: 18, height: 18)
                            .animation(.easeInOut(duration: 0.8).repeatForever(autoreverses: true), value: active)
                    }
                }

            VStack(alignment: .leading, spacing: 2) {
                Text(label)
                    .font(.caption.bold())
                Text(detail)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }

    // MARK: - Activity Log

    private var activityLogView: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("RECENT ACTIVITY")
                .font(.caption.bold())
                .foregroundStyle(.secondary)

            if daemon.activityLog.isEmpty {
                Text("No activity yet")
                    .font(.callout)
                    .foregroundStyle(.tertiary)
            } else {
                ScrollView {
                    ScrollViewReader { proxy in
                        LazyVStack(alignment: .leading, spacing: 2) {
                            ForEach(daemon.activityLog.suffix(30)) { entry in
                                logEntryRow(entry)
                                    .id(entry.id)
                            }
                        }
                        .onChange(of: daemon.activityLog.count) { _, _ in
                            if let last = daemon.activityLog.last {
                                withAnimation(.easeOut(duration: 0.2)) {
                                    proxy.scrollTo(last.id, anchor: .bottom)
                                }
                            }
                        }
                    }
                }
                .frame(maxHeight: 150)
            }
        }
    }

    private func logEntryRow(_ entry: ActivityLogEntry) -> some View {
        HStack(spacing: 8) {
            // Stage-colored left bar
            RoundedRectangle(cornerRadius: 1)
                .fill(stageColor(entry.stage))
                .frame(width: 3, height: 16)

            Text(entry.ts)
                .font(.caption.monospaced())
                .foregroundStyle(.tertiary)

            Text(entry.action)
                .font(.caption.bold())
                .foregroundStyle(stageColor(entry.stage))

            Text(entry.detail)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .truncationMode(.middle)

            Spacer()
        }
    }

    private func stageColor(_ stage: String) -> Color {
        switch stage {
        case "scanning": return .blue
        case "extracting": return .orange
        case "chunking": return .purple
        case "embedding": return .cyan
        case "annotating": return .green
        case "conceptualizing": return .pink
        case "completed": return .green
        default: return .gray
        }
    }
}
