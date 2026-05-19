import Darwin
import Foundation

struct DaemonInfo: Codable {
    let name: String
    let daemon: String
    let daemonVersion: String
    let apiVersion: String
    let sourcePackVersion: String
    let buildCommit: String
    let pid: Int
    let startedAt: String
    let storagePath: String?
}

struct ScoreSession: Codable, Identifiable {
    let id: String
    let title: String?
    let status: String
    let attention: String
    let statusDetail: String?
    let statusUpdatedAt: String?
    let statusSource: String?
    let confidence: String
    let source: String
    let cwd: String?
    let workspaceRoots: [String]?
    let lastSeenAt: String?
    let lastActivityAt: String
}

struct ScoreSource: Codable, Identifiable {
    let id: String
    let name: String
    let status: String
    let supportLevel: String
    let lifecycle: ScoreSourceLifecycle?
    let diagnostics: [String]?
}

struct ScoreSourceLifecycle: Codable {
    let canDetectLiveness: Bool
    let canDetectStart: Bool
    let canDetectActivity: Bool
    let canDetectWaiting: Bool
    let canDetectTerminal: Bool
}

struct ScoreSnapshot: Codable {
    let daemon: DaemonInfo
    let sessions: [ScoreSession]
    let sources: [ScoreSource]
}

struct ScoreRPCError: Codable, Error {
    let code: Int
    let message: String
}

struct ScoreResponse<T: Decodable>: Decodable {
    let result: T?
    let error: ScoreRPCError?
}

final class ScoreClient {
    static let apiVersion = "score-jsonrpc/v1"

    func snapshot() throws -> ScoreSnapshot {
        let daemon: DaemonInfo = try call("daemon/info", params: [:])
        let sessions: [ScoreSession] = try call("sessions/list", params: ["all": false])
        let sources: [ScoreSource] = try call("sources/list", params: [:])
        return ScoreSnapshot(daemon: daemon, sessions: sessions, sources: sources)
    }

    func snapshotJSON() throws -> String {
        let snapshot = try snapshot()
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(snapshot)
        return String(decoding: data, as: UTF8.self)
    }

    func socketPath() -> String {
        if let override = ProcessInfo.processInfo.environment["SCORE_SOCKET"], !override.isEmpty {
            return override
        }
        if let runtimeDir = ProcessInfo.processInfo.environment["XDG_RUNTIME_DIR"], !runtimeDir.isEmpty {
            return "\(runtimeDir)/score.sock"
        }
        return "\(NSTemporaryDirectory())score-\(getuid()).sock"
    }

    private func call<T: Decodable>(_ method: String, params: [String: Any]) throws -> T {
        let request: [String: Any] = [
            "jsonrpc": "2.0",
            "id": 1,
            "method": method,
            "params": params
        ]
        let requestData = try JSONSerialization.data(withJSONObject: request)
        let responseData = try roundTrip(requestData + Data([0x0a]))
        let decoder = JSONDecoder()
        let response = try decoder.decode(ScoreResponse<T>.self, from: responseData)
        if let error = response.error {
            throw error
        }
        guard let result = response.result else {
            throw ScoreRPCError(code: -32603, message: "empty result")
        }
        return result
    }

    private func roundTrip(_ request: Data) throws -> Data {
        let fd = try connectSocket(path: socketPath())
        defer { close(fd) }

        try request.withUnsafeBytes { rawBuffer in
            guard let base = rawBuffer.baseAddress else { return }
            var written = 0
            while written < rawBuffer.count {
                let n = Darwin.write(fd, base.advanced(by: written), rawBuffer.count - written)
                if n < 0 {
                    throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
                }
                written += n
            }
        }

        var out = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        while true {
            let n = Darwin.read(fd, &buffer, buffer.count)
            if n < 0 {
                throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
            }
            if n == 0 {
                break
            }
            if let newline = buffer[..<n].firstIndex(of: 0x0a) {
                out.append(buffer, count: newline)
                break
            }
            out.append(buffer, count: n)
        }
        return out
    }

    private func connectSocket(path: String) throws -> Int32 {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        if fd < 0 {
            throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
        }

        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let maxPath = MemoryLayout.size(ofValue: address.sun_path)
        guard path.utf8.count < maxPath else {
            close(fd)
            throw ScoreRPCError(code: -1, message: "socket path is too long: \(path)")
        }

        path.withCString { source in
            withUnsafeMutableBytes(of: &address.sun_path) { rawBuffer in
                let destination = rawBuffer.bindMemory(to: CChar.self).baseAddress!
                strncpy(destination, source, maxPath - 1)
            }
        }

        let length = socklen_t(MemoryLayout<sa_family_t>.size + path.utf8.count + 1)
        let result = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPointer in
                Darwin.connect(fd, sockaddrPointer, length)
            }
        }
        if result < 0 {
            let error = POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
            close(fd)
            throw error
        }
        return fd
    }
}

extension Data {
    static func + (lhs: Data, rhs: Data) -> Data {
        var data = lhs
        data.append(rhs)
        return data
    }
}
