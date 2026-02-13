import SwiftUI

struct ConceptBrowserView: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var concepts: [ConceptInfo] = []
    @State private var selectedConcept: ConceptInfo?
    @State private var detail: ConceptDetail?
    @State private var whyExplanation: WhyExplanation?
    @State private var isLoading = false

    var body: some View {
        HSplitView {
            // Concept list — narrow sidebar
            VStack(spacing: 0) {
                HStack {
                    Text("Concepts")
                        .font(.headline)
                    Spacer()
                    Button("Refresh") { loadConcepts() }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                }
                .padding(12)
                .background(.bar)

                Divider()

                if concepts.isEmpty && !isLoading {
                    VStack(spacing: 12) {
                        Image(systemName: "circle.hexagongrid")
                            .font(.system(size: 36))
                            .foregroundStyle(.tertiary)
                        Text("No concepts yet")
                            .foregroundStyle(.secondary)
                        Text("Run ingestion first")
                            .font(.caption)
                            .foregroundStyle(.tertiary)
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else {
                    List(concepts, selection: $selectedConcept) { concept in
                        HStack(spacing: 8) {
                            Circle()
                                .fill(colorForLevel(concept.level))
                                .frame(width: 8, height: 8)
                            Text(concept.label ?? "Unlabeled")
                                .font(.body)
                                .lineLimit(2)
                            Spacer()
                            Text("L\(concept.level)")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                        }
                        .tag(concept)
                    }
                    .listStyle(.inset)
                }
            }
            .frame(minWidth: 200, idealWidth: 260, maxWidth: 320)

            // Detail panel — gets majority of space
            if let concept = selectedConcept {
                ConceptDetailPanel(
                    concept: concept,
                    detail: detail,
                    whyExplanation: whyExplanation
                )
                .frame(minWidth: 500)
            } else {
                VStack(spacing: 12) {
                    Image(systemName: "arrow.left")
                        .font(.title)
                        .foregroundStyle(.tertiary)
                    Text("Select a concept")
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .onChange(of: selectedConcept) { _, newVal in
            if let c = newVal {
                loadDetail(c.id)
                loadWhy(c.id)
            }
        }
        .onAppear { loadConcepts() }
    }

    private func loadConcepts() {
        guard daemon.isConnected else { return }
        isLoading = true
        Task {
            do {
                let items = try await daemon.client.listConcepts()
                await MainActor.run {
                    concepts = items
                    isLoading = false
                }
            } catch {
                await MainActor.run { isLoading = false }
            }
        }
    }

    private func loadDetail(_ id: String) {
        Task {
            do {
                let d = try await daemon.client.getConceptDetail(conceptId: id)
                await MainActor.run { detail = d }
            } catch { }
        }
    }

    private func loadWhy(_ id: String) {
        Task {
            do {
                let w = try await daemon.client.whyConcept(conceptId: id)
                await MainActor.run { whyExplanation = w }
            } catch { }
        }
    }

    private func colorForLevel(_ level: Int) -> Color {
        switch level {
        case 0: return .blue
        case 1: return .cyan
        case 2: return .teal
        default: return .gray
        }
    }
}

struct ConceptDetailPanel: View {
    let concept: ConceptInfo
    let detail: ConceptDetail?
    let whyExplanation: WhyExplanation?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                // Header — compact, not wasteful
                VStack(alignment: .leading, spacing: 6) {
                    Text(concept.label ?? "Unlabeled")
                        .font(.title2.bold())
                    if let desc = concept.description {
                        Text(desc)
                            .font(.body)
                            .foregroundStyle(.secondary)
                    }
                    HStack(spacing: 16) {
                        Label("Level \(concept.level)", systemImage: "circle.hexagongrid")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        if let model = concept.model_id {
                            Label(model, systemImage: "cpu")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }

                Divider()

                // Members — full text, no truncation
                if let d = detail {
                    VStack(alignment: .leading, spacing: 10) {
                        Text("Members (\(d.member_count))")
                            .font(.headline)

                        ForEach(d.members) { member in
                            VStack(alignment: .leading, spacing: 4) {
                                if let summary = member.summary {
                                    Text(summary)
                                        .font(.body)
                                }
                                Text(member.text)
                                    .font(.callout)
                                    .foregroundStyle(.secondary)
                            }
                            .padding(10)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .background(Color(nsColor: .controlBackgroundColor))
                            .cornerRadius(6)
                        }
                    }
                }

                // Why explanation — full text, no truncation
                if let why = whyExplanation {
                    VStack(alignment: .leading, spacing: 10) {
                        Text("Why This Concept?")
                            .font(.headline)

                        Text(why.explanation)
                            .font(.body)
                            .foregroundStyle(.secondary)

                        if !why.evidence.isEmpty {
                            Text("Evidence")
                                .font(.subheadline.bold())
                                .padding(.top, 4)

                            ForEach(why.evidence) { ev in
                                VStack(alignment: .leading, spacing: 4) {
                                    HStack {
                                        Image(systemName: "doc.text")
                                            .foregroundStyle(.blue)
                                        Text(ev.asset_filename ?? "Unknown file")
                                            .font(.body.bold())
                                    }
                                    if let summary = ev.annotation_summary {
                                        Text(summary)
                                            .font(.callout)
                                            .italic()
                                    }
                                    Text(ev.chunk_text)
                                        .font(.callout)
                                        .foregroundStyle(.secondary)
                                }
                                .padding(10)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .background(Color(nsColor: .controlBackgroundColor))
                                .cornerRadius(6)
                            }
                        }
                    }
                }
            }
            .padding(20)
        }
    }
}

extension ConceptInfo: Hashable, Equatable {
    static func == (lhs: ConceptInfo, rhs: ConceptInfo) -> Bool { lhs.id == rhs.id }
    func hash(into hasher: inout Hasher) { hasher.combine(id) }
}
