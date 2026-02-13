import Foundation

actor DaemonClient {
    private let baseURL: URL
    private let session: URLSession

    init(baseURL: URL = URL(string: "http://127.0.0.1:8742")!) {
        self.baseURL = baseURL
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        self.session = URLSession(configuration: config)
    }

    // MARK: - Health

    func healthCheck() async throws -> HealthResponse {
        return try await get("/health")
    }

    // MARK: - Volumes

    func addVolume(path: String, label: String? = nil) async throws -> VolumeResponse {
        let body = AddVolumeRequest(path: path, label: label)
        return try await post("/volumes/add", body: body)
    }

    func listVolumes() async throws -> [VolumeResponse] {
        return try await get("/volumes/list")
    }

    // MARK: - Ingest

    func startIngest(paths: [String]? = nil) async throws -> IngestResponse {
        let body = StartIngestRequest(paths: paths)
        return try await post("/ingest/start", body: body)
    }

    func ingestStatus() async throws -> IngestStatusResponse {
        return try await get("/ingest/status")
    }

    // MARK: - Search

    func search(query: String, limit: Int = 20) async throws -> [SearchResultItem] {
        let body = SearchRequest(query: query, limit: limit, filter_asset_type: nil)
        return try await post("/search", body: body)
    }

    // MARK: - Evidence

    func getEvidence(assetId: String) async throws -> EvidenceResponse {
        return try await get("/evidence/\(assetId)")
    }

    func getChunkEvidence(chunkId: String) async throws -> EvidenceResponse {
        return try await get("/evidence/chunk/\(chunkId)")
    }

    func listAssets() async throws -> [AssetInfo] {
        return try await get("/evidence/assets/all")
    }

    // MARK: - Universe

    func universeSnapshot(lod: String = "macro") async throws -> UniverseSnapshot {
        return try await get("/universe/snapshot?lod=\(lod)")
    }

    // MARK: - Concepts

    func listConcepts(level: Int? = nil) async throws -> [ConceptInfo] {
        let path = level != nil ? "/concepts/list?level=\(level!)" : "/concepts/list"
        return try await get(path)
    }

    func getConceptDetail(conceptId: String) async throws -> ConceptDetail {
        return try await get("/concepts/\(conceptId)")
    }

    func whyConcept(conceptId: String) async throws -> WhyExplanation {
        return try await get("/concepts/\(conceptId)/why")
    }

    // MARK: - HTTP Helpers

    private func buildURL(_ path: String) -> URL {
        // Use string concatenation to preserve query parameters
        // (appendingPathComponent encodes '?' as '%3F')
        URL(string: baseURL.absoluteString + path)!
    }

    private func get<T: Decodable>(_ path: String) async throws -> T {
        var request = URLRequest(url: buildURL(path))
        request.httpMethod = "GET"
        let (data, response) = try await session.data(for: request)
        try checkResponse(response, data: data)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func post<B: Encodable, T: Decodable>(_ path: String, body: B) async throws -> T {
        var request = URLRequest(url: buildURL(path))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(body)
        let (data, response) = try await session.data(for: request)
        try checkResponse(response, data: data)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func checkResponse(_ response: URLResponse, data: Data) throws {
        guard let http = response as? HTTPURLResponse else {
            throw DaemonError.invalidResponse
        }
        guard (200...299).contains(http.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? "no body"
            throw DaemonError.httpError(statusCode: http.statusCode, message: body)
        }
    }
}

enum DaemonError: LocalizedError {
    case invalidResponse
    case httpError(statusCode: Int, message: String)
    case notConnected

    var errorDescription: String? {
        switch self {
        case .invalidResponse:
            return "Invalid response from daemon"
        case .httpError(let code, let msg):
            return "HTTP \(code): \(msg)"
        case .notConnected:
            return "Not connected to daemon"
        }
    }
}
