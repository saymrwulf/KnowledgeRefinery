import SwiftUI

enum WorkspaceSetupMode {
    case create
    case edit(Workspace)
}

struct WorkspaceSetupSheet: View {
    let mode: WorkspaceSetupMode
    @EnvironmentObject var store: WorkspaceStore
    @Environment(\.dismiss) private var dismiss

    @State private var name: String = ""
    @State private var colorTag: String = "blue"
    @State private var dataLakePaths: [String] = []

    private let colorOptions = ["blue", "green", "orange", "purple", "red", "yellow", "pink"]

    private var isEditing: Bool {
        if case .edit = mode { return true }
        return false
    }

    var body: some View {
        VStack(spacing: 16) {
            // Header
            HStack {
                Text(isEditing ? "Edit Workspace" : "New Workspace")
                    .font(.title2.bold())
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
            }

            // Name field
            VStack(alignment: .leading, spacing: 6) {
                Text("Name")
                    .font(.headline)
                TextField("Enter workspace name", text: $name)
                    .textFieldStyle(.roundedBorder)
            }

            // Color Tag
            VStack(alignment: .leading, spacing: 6) {
                Text("Color Tag")
                    .font(.headline)
                HStack(spacing: 12) {
                    ForEach(colorOptions, id: \.self) { color in
                        Button {
                            colorTag = color
                        } label: {
                            Circle()
                                .fill(swiftColor(for: color))
                                .frame(width: 28, height: 28)
                                .overlay {
                                    if colorTag == color {
                                        Image(systemName: "checkmark")
                                            .font(.caption.bold())
                                            .foregroundStyle(.white)
                                    }
                                }
                        }
                        .buttonStyle(.plain)
                    }
                }
            }

            // Data Lakes
            VStack(alignment: .leading, spacing: 6) {
                HStack {
                    Text("Data Lakes")
                        .font(.headline)
                    Spacer()
                    Button {
                        pickFolder()
                    } label: {
                        Label("Add Folder", systemImage: "plus")
                    }
                    .controlSize(.small)
                }

                if dataLakePaths.isEmpty {
                    Text("No folders added yet. Click \"Add Folder\" to add data sources.")
                        .foregroundStyle(.secondary)
                        .font(.caption)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.vertical, 8)
                } else {
                    VStack(spacing: 4) {
                        ForEach(dataLakePaths, id: \.self) { path in
                            HStack {
                                Image(systemName: "folder.fill")
                                    .foregroundStyle(.blue)
                                Text(URL(fileURLWithPath: path).lastPathComponent)
                                    .font(.callout)
                                Text(path)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                                    .truncationMode(.middle)
                                Spacer()
                                Button {
                                    dataLakePaths.removeAll { $0 == path }
                                } label: {
                                    Image(systemName: "xmark.circle.fill")
                                        .foregroundStyle(.secondary)
                                }
                                .buttonStyle(.plain)
                            }
                            .padding(.vertical, 4)
                            .padding(.horizontal, 8)
                            .background(Color(nsColor: .controlBackgroundColor))
                            .clipShape(RoundedRectangle(cornerRadius: 4))
                        }
                    }
                }
            }

            Spacer()

            Divider()

            // Footer
            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button(isEditing ? "Save" : "Create") {
                    save()
                }
                .buttonStyle(.borderedProminent)
                .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty)
                .keyboardShortcut(.defaultAction)
            }
        }
        .padding(20)
        .frame(width: 500, height: 480)
        .onAppear {
            if case .edit(let ws) = mode {
                name = ws.name
                colorTag = ws.colorTag
                dataLakePaths = ws.dataLakePaths
            }
        }
    }

    private func pickFolder() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = true
        panel.message = "Select folders to include as data lakes"
        if panel.runModal() == .OK {
            for url in panel.urls {
                let path = url.path
                if !dataLakePaths.contains(path) {
                    dataLakePaths.append(path)
                }
            }
        }
    }

    private func save() {
        let trimmedName = name.trimmingCharacters(in: .whitespaces)
        guard !trimmedName.isEmpty else { return }

        if case .edit(var ws) = mode {
            ws.name = trimmedName
            ws.colorTag = colorTag
            ws.dataLakePaths = dataLakePaths
            store.updateWorkspace(ws)
        } else {
            _ = store.createWorkspace(name: trimmedName, colorTag: colorTag, dataLakePaths: dataLakePaths)
        }
        dismiss()
    }

    private func swiftColor(for tag: String) -> Color {
        switch tag {
        case "blue": return .blue
        case "green": return .green
        case "orange": return .orange
        case "purple": return .purple
        case "red": return .red
        case "yellow": return .yellow
        case "pink": return .pink
        default: return .gray
        }
    }
}
