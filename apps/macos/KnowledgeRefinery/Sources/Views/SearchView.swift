import SwiftUI
import QuickLook

struct SearchView: View {
    @EnvironmentObject var daemon: DaemonManager
    @State private var query = ""
    @State private var results: [SearchResultItem] = []
    @State private var isSearching = false
    @State private var errorMessage: String?
    @State private var selectedResult: SearchResultItem?
    @State private var previewURL: URL?

    var body: some View {
        VStack(spacing: 0) {
            // Search bar
            HStack {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                TextField("Search your knowledge base...", text: $query)
                    .textFieldStyle(.plain)
                    .font(.title3)
                    .onSubmit { performSearch() }

                if isSearching {
                    ProgressView()
                        .controlSize(.small)
                }

                Button("Search") { performSearch() }
                    .buttonStyle(.borderedProminent)
                    .disabled(query.isEmpty || isSearching)
            }
            .padding()
            .background(.bar)

            Divider()

            if let error = errorMessage {
                VStack {
                    Image(systemName: "exclamationmark.triangle")
                        .font(.largeTitle)
                        .foregroundStyle(.orange)
                    Text(error)
                        .foregroundStyle(.secondary)
                }
                .padding()
            }

            if results.isEmpty && !isSearching && errorMessage == nil {
                VStack(spacing: 12) {
                    Image(systemName: "text.magnifyingglass")
                        .font(.system(size: 48))
                        .foregroundStyle(.tertiary)
                    Text("Search your ingested corpus")
                        .font(.title2)
                        .foregroundStyle(.secondary)
                    Text("Enter a query to find semantically similar content")
                        .foregroundStyle(.tertiary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                // Results list
                HSplitView {
                    List(results, selection: $selectedResult) { result in
                        SearchResultRow(result: result)
                            .tag(result)
                    }
                    .listStyle(.inset)
                    .frame(minWidth: 300)

                    // Detail panel
                    if let selected = selectedResult {
                        SearchDetailView(result: selected)
                    } else {
                        Text("Select a result to see details")
                            .foregroundStyle(.secondary)
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                    }
                }
            }
        }
        .navigationTitle("Search")
        .quickLookPreview($previewURL)
    }

    private func performSearch() {
        guard !query.isEmpty, daemon.isConnected else { return }
        isSearching = true
        errorMessage = nil

        Task {
            do {
                let items = try await daemon.client.search(query: query)
                await MainActor.run {
                    results = items
                    isSearching = false
                }
            } catch {
                await MainActor.run {
                    errorMessage = error.localizedDescription
                    isSearching = false
                }
            }
        }
    }
}

struct SearchResultRow: View {
    let result: SearchResultItem

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Image(systemName: iconForFile(result.asset_path))
                    .foregroundStyle(.blue)
                Text(result.filename)
                    .font(.headline)
                    .lineLimit(1)
                Spacer()
                Text(String(format: "%.2f", result.score))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(.fill.tertiary)
                    .clipShape(Capsule())
            }

            Text(result.text)
                .font(.body)
                .lineLimit(3)
                .foregroundStyle(.secondary)

            if let topics = result.topics, !topics.isEmpty {
                Text(topics)
                    .font(.caption)
                    .foregroundStyle(.blue)
            }
        }
        .padding(.vertical, 4)
    }

    private func iconForFile(_ path: String) -> String {
        let ext = URL(fileURLWithPath: path).pathExtension.lowercased()
        switch ext {
        case "pdf": return "doc.richtext"
        case "txt", "md", "markdown": return "doc.text"
        case "html", "htm": return "globe"
        case "jpg", "jpeg", "png", "webp", "heic": return "photo"
        default: return "doc"
        }
    }
}

struct SearchDetailView: View {
    let result: SearchResultItem
    @EnvironmentObject var daemon: DaemonManager
    @State private var previewURL: URL?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                // File info
                GroupBox("Source") {
                    VStack(alignment: .leading, spacing: 8) {
                        LabeledContent("File", value: result.filename)
                        LabeledContent("Path", value: result.asset_path)
                        LabeledContent("Score", value: String(format: "%.4f", result.score))
                        if let topics = result.topics, !topics.isEmpty {
                            LabeledContent("Topics", value: topics)
                        }
                        if let sentiment = result.sentiment {
                            HStack {
                                Text("Sentiment:")
                                sentimentBadge(sentiment)
                            }
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }

                // LLM Summary
                if let summary = result.summary, !summary.isEmpty {
                    GroupBox("LLM Summary") {
                        Text(summary)
                            .font(.body)
                            .italic()
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }

                // Entities
                if let entities = result.entities, !entities.isEmpty {
                    GroupBox("Entities") {
                        FlowLayout(spacing: 6) {
                            ForEach(entities, id: \.self) { entity in
                                Text(entity)
                                    .font(.caption)
                                    .padding(.horizontal, 8)
                                    .padding(.vertical, 3)
                                    .background(.blue.opacity(0.1))
                                    .foregroundStyle(.blue)
                                    .clipShape(Capsule())
                            }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }

                // Open in Quick Look
                Button("Open in Quick Look") {
                    let url = URL(fileURLWithPath: result.asset_path)
                    if FileManager.default.fileExists(atPath: url.path) {
                        previewURL = url
                    }
                }
                .buttonStyle(.bordered)

                // Chunk text
                GroupBox("Content") {
                    Text(result.text)
                        .font(.body)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }

                // Evidence anchor
                GroupBox("Evidence Anchor") {
                    Text(result.evidence_anchor)
                        .font(.system(.caption, design: .monospaced))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            .padding()
        }
        .quickLookPreview($previewURL)
    }

    @ViewBuilder
    private func sentimentBadge(_ sentiment: String) -> some View {
        let color: Color = switch sentiment {
        case "positive": .green
        case "negative": .red
        case "mixed": .orange
        default: .gray
        }
        Text(sentiment)
            .font(.caption)
            .padding(.horizontal, 8)
            .padding(.vertical, 2)
            .background(color.opacity(0.15))
            .foregroundStyle(color)
            .clipShape(Capsule())
    }
}

struct FlowLayout: Layout {
    var spacing: CGFloat = 6

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let result = arrangeSubviews(proposal: proposal, subviews: subviews)
        return result.size
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let result = arrangeSubviews(proposal: proposal, subviews: subviews)
        for (index, position) in result.positions.enumerated() {
            subviews[index].place(at: CGPoint(x: bounds.minX + position.x, y: bounds.minY + position.y), proposal: .unspecified)
        }
    }

    private func arrangeSubviews(proposal: ProposedViewSize, subviews: Subviews) -> (positions: [CGPoint], size: CGSize) {
        let maxWidth = proposal.width ?? .infinity
        var positions: [CGPoint] = []
        var x: CGFloat = 0
        var y: CGFloat = 0
        var rowHeight: CGFloat = 0
        var maxX: CGFloat = 0

        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if x + size.width > maxWidth && x > 0 {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            positions.append(CGPoint(x: x, y: y))
            rowHeight = max(rowHeight, size.height)
            x += size.width + spacing
            maxX = max(maxX, x)
        }

        return (positions, CGSize(width: maxX, height: y + rowHeight))
    }
}
