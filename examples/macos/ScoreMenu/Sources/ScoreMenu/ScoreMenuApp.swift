import AppKit
import SwiftUI

@main
struct ScoreMenuApp: App {
    @StateObject private var model = ScoreMenuModel()

    var body: some Scene {
        MenuBarExtra("Score", systemImage: model.menuIcon) {
            ScorePopover(model: model)
                .frame(width: 420)
                .task {
                    await model.refresh()
                    model.startPolling()
                }
        }
        .menuBarExtraStyle(.window)
    }
}

@MainActor
final class ScoreMenuModel: ObservableObject {
    enum DaemonState {
        case loading
        case running
        case stale
        case unreachable(String)
    }

    @Published var state: DaemonState = .loading
    @Published var snapshot: ScoreSnapshot?
    @Published var lastUpdated: Date?

    private let client = ScoreClient()
    private var pollTask: Task<Void, Never>?

    var menuIcon: String {
        switch state {
        case .running:
            return "waveform.path.ecg"
        case .stale:
            return "exclamationmark.triangle"
        case .loading:
            return "clock"
        case .unreachable:
            return "xmark.circle"
        }
    }

    func startPolling() {
        guard pollTask == nil else { return }
        pollTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(3))
                await self?.refresh()
            }
        }
    }

    func refresh() async {
        do {
            let snapshot = try await Task.detached {
                try ScoreClient().snapshot()
            }.value
            self.snapshot = snapshot
            self.lastUpdated = Date()
            if snapshot.daemon.apiVersion == ScoreClient.apiVersion {
                self.state = .running
            } else {
                self.state = .stale
            }
        } catch {
            self.snapshot = nil
            self.state = .unreachable(error.localizedDescription)
        }
    }

    func copyJSON() {
        do {
            let json = try client.snapshotJSON()
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(json, forType: .string)
        } catch {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(error.localizedDescription, forType: .string)
        }
    }

    func openDocs() {
        NSWorkspace.shared.open(URL(string: "https://github.com/fridiculous/the-score")!)
    }
}

struct ScorePopover: View {
    @ObservedObject var model: ScoreMenuModel

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            header
            Divider()
            content
            Divider()
            HStack {
                Button("Open docs") {
                    model.openDocs()
                }
                Button("Copy JSON") {
                    model.copyJSON()
                }
                Spacer()
                Button("Refresh") {
                    Task { await model.refresh() }
                }
            }
        }
        .padding(14)
    }

    @ViewBuilder
    private var header: some View {
        switch model.state {
        case .loading:
            StatusRow(symbol: "clock", title: "scored loading", detail: nil)
        case .running:
            if let daemon = model.snapshot?.daemon {
                StatusRow(symbol: "checkmark.circle", title: "scored running", detail: daemonDetail(daemon))
            }
        case .stale:
            if let daemon = model.snapshot?.daemon {
                StatusRow(symbol: "exclamationmark.triangle", title: "stale daemon API", detail: daemonDetail(daemon))
            }
        case .unreachable(let message):
            VStack(alignment: .leading, spacing: 8) {
                StatusRow(symbol: "xmark.circle", title: "scored unreachable", detail: message)
                Text("Start it with: score start")
                    .font(.system(.body, design: .monospaced))
                    .textSelection(.enabled)
            }
        }
    }

    @ViewBuilder
    private var content: some View {
        if let snapshot = model.snapshot {
            if snapshot.sessions.isEmpty {
                Text("No active sessions")
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 10) {
                        ForEach(groupedSessions(snapshot.sessions), id: \.key) { group in
                            SessionGroupView(title: group.key, sessions: group.value, sources: snapshot.sources)
                        }
                    }
                }
                .frame(maxHeight: 460)
            }
        } else if case .unreachable = model.state {
            EmptyView()
        } else {
            ProgressView()
                .frame(maxWidth: .infinity)
        }
    }

    private func daemonDetail(_ daemon: DaemonInfo) -> String {
        let storage = daemon.storagePath ?? "unknown storage"
        return "v\(daemon.daemonVersion) api=\(daemon.apiVersion) storage=\(storage)"
    }

    private func groupedSessions(_ sessions: [ScoreSession]) -> [(key: String, value: [ScoreSession])] {
        let grouped = Dictionary(grouping: sessions) { session in
            "\(session.source) / \(session.status) / \(workspaceLabel(session))"
        }
        return grouped
            .map { ($0.key, $0.value.sorted { $0.id < $1.id }) }
            .sorted { $0.key < $1.key }
    }
}

struct SessionGroupView: View {
    let title: String
    let sessions: [ScoreSession]
    let sources: [ScoreSource]

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title)
                .font(.headline)
            ForEach(sessions) { session in
                VStack(alignment: .leading, spacing: 4) {
                    HStack {
                        Text(session.title ?? session.id)
                            .font(.body.weight(.semibold))
                            .lineLimit(1)
                        Spacer()
                        Text(session.confidence)
                            .font(.caption)
                            .foregroundStyle(confidenceColor(session.confidence))
                    }
                    Text(session.statusDetail ?? session.id)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                    if let diagnostic = diagnostic(for: session.source) {
                        Text(diagnostic)
                            .font(.caption2)
                            .foregroundStyle(.orange)
                            .lineLimit(2)
                    }
                }
                .padding(8)
                .background(.quaternary, in: RoundedRectangle(cornerRadius: 8))
            }
        }
    }

    private func diagnostic(for sourceID: String) -> String? {
        sources.first(where: { $0.id == sourceID })?.diagnostics?.first
    }

    private func confidenceColor(_ confidence: String) -> Color {
        switch confidence {
        case "high":
            return .green
        case "medium":
            return .blue
        case "low":
            return .orange
        default:
            return .secondary
        }
    }
}

struct StatusRow: View {
    let symbol: String
    let title: String
    let detail: String?

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: symbol)
                .frame(width: 18)
            VStack(alignment: .leading, spacing: 3) {
                Text(title)
                    .font(.headline)
                if let detail {
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                }
            }
        }
    }
}

private func workspaceLabel(_ session: ScoreSession) -> String {
    if let cwd = session.cwd, !cwd.isEmpty {
        return URL(fileURLWithPath: cwd).lastPathComponent
    }
    if let root = session.workspaceRoots?.first {
        return URL(fileURLWithPath: root).lastPathComponent
    }
    return "-"
}
