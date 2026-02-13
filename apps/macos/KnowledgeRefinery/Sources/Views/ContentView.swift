import SwiftUI

struct ContentView: View {
    @EnvironmentObject var daemon: DaemonManager

    var body: some View {
        NavigationSplitView {
            SidebarView()
        } detail: {
            SearchView()
        }
        .toolbar {
            ToolbarItem(placement: .automatic) {
                ConnectionStatusView()
            }
        }
        // Connection is managed by WorkspaceDetailView
    }
}

struct SidebarView: View {
    var body: some View {
        List {
            Section("Explore") {
                NavigationLink(destination: SearchView()) {
                    Label("Search", systemImage: "magnifyingglass")
                }
                NavigationLink(destination: UniverseView()) {
                    Label("Universe", systemImage: "globe")
                }
                NavigationLink(destination: ConceptBrowserView()) {
                    Label("Themes", systemImage: "circle.hexagongrid")
                }
            }
            Section("Manage") {
                NavigationLink(destination: IngestStatusView()) {
                    Label("Processing", systemImage: "gearshape.arrow.triangle.2.circlepath")
                }
                NavigationLink(destination: VolumeManagerView()) {
                    Label("Source Folders", systemImage: "folder")
                }
                NavigationLink(destination: AssetsView()) {
                    Label("Documents", systemImage: "doc.on.doc")
                }
            }
        }
        .listStyle(.sidebar)
        .navigationTitle("Workspace")
    }
}

struct ConnectionStatusView: View {
    @EnvironmentObject var daemon: DaemonManager

    var body: some View {
        HStack(spacing: 8) {
            Circle()
                .fill(daemon.isConnected ? Color.green : Color.red)
                .frame(width: 10, height: 10)
            Text(daemon.isConnected ? "Ready" : "Offline")
                .font(.callout)
                .foregroundStyle(.secondary)

            if daemon.isConnected {
                Circle()
                    .fill(daemon.isLMStudioAvailable ? Color.green : Color.orange)
                    .frame(width: 10, height: 10)
                Text("AI Engine")
                    .font(.callout)
                    .foregroundStyle(.secondary)
            }
        }
    }
}
