import Foundation

// MARK: - API Request/Response Models

struct AddVolumeRequest: Codable {
    let path: String
    let label: String?
}

struct VolumeResponse: Codable, Identifiable {
    let id: String
    let path: String
    let label: String?
    let added_at: String
    let last_scan_at: String?
}

struct StartIngestRequest: Codable {
    let paths: [String]?
}

struct IngestResponse: Codable {
    let job_id: String
    let status: String
}

struct IngestStatusResponse: Codable, Sendable {
    let running: Bool
    let current_job_id: String?
    let total_assets: Int
    let status_counts: [String: Int]?
    let latest_job: LatestJobInfo?
    let vector_count: Int?
    let chunk_count: Int?
    let annotation_count: Int?
    let concept_count: Int?
    let edge_count: Int?
    let live: LiveStageProgress?
    let activity_log: [ActivityLogEntry]?

    struct LatestJobInfo: Codable, Sendable {
        let job_id: String?
        let status: String?
        let progress: ProgressInfo?
    }

    struct ProgressInfo: Codable, Sendable {
        let stage: String?
        let stages: [String: AnyCodable]?
    }

    /// Current stage name from latest_job.progress.stage
    var currentStage: String? {
        latest_job?.progress?.stage
    }
}

/// Live progress for the currently-executing pipeline stage.
/// The daemon sends a dict with one key (the stage name) mapping to stage-specific fields.
struct LiveStageProgress: Codable, Sendable {
    let scan: LiveScanProgress?
    let extract: LiveFileProgress?
    let chunk: LiveChunkProgress?
    let embed: LiveEmbedProgress?
    let annotate: LiveAnnotateProgress?
    let conceptualize: LiveConceptualizeProgress?

    struct LiveScanProgress: Codable, Sendable {
        let current_path: String?
        let done: Int?
        let total: Int?
    }

    struct LiveFileProgress: Codable, Sendable {
        let current_file: String?
        let done: Int?
        let total: Int?
    }

    struct LiveChunkProgress: Codable, Sendable {
        let current_file: String?
        let done: Int?
        let total: Int?
        let chunks_created: Int?
    }

    struct LiveEmbedProgress: Codable, Sendable {
        let embedded: Int?
        let total: Int?
    }

    struct LiveAnnotateProgress: Codable, Sendable {
        let current_file: String?
        let done: Int?
        let total: Int?
        let annotated_chunks: Int?
    }

    struct LiveConceptualizeProgress: Codable, Sendable {
        let status: String?
        let concepts: Int?
    }
}

/// A timestamped event from the pipeline activity log ring buffer.
struct ActivityLogEntry: Codable, Identifiable, Sendable {
    let ts: String
    let stage: String
    let action: String
    let detail: String
    let counts: [String: Int]?

    var id: String { "\(ts)-\(stage)-\(action)-\(detail)" }
}

/// Type-erased Codable for flexible JSON dictionaries (stage details vary by stage).
struct AnyCodable: Codable, @unchecked Sendable {
    let value: Any

    init(_ value: Any) {
        self.value = value
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let intVal = try? container.decode(Int.self) {
            value = intVal
        } else if let doubleVal = try? container.decode(Double.self) {
            value = doubleVal
        } else if let stringVal = try? container.decode(String.self) {
            value = stringVal
        } else if let dictVal = try? container.decode([String: AnyCodable].self) {
            value = dictVal.mapValues { $0.value }
        } else if let arrayVal = try? container.decode([AnyCodable].self) {
            value = arrayVal.map { $0.value }
        } else if let boolVal = try? container.decode(Bool.self) {
            value = boolVal
        } else {
            value = NSNull()
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        if let intVal = value as? Int {
            try container.encode(intVal)
        } else if let doubleVal = value as? Double {
            try container.encode(doubleVal)
        } else if let stringVal = value as? String {
            try container.encode(stringVal)
        } else if let boolVal = value as? Bool {
            try container.encode(boolVal)
        } else {
            try container.encodeNil()
        }
    }
}

struct SearchRequest: Codable {
    let query: String
    let limit: Int
    let filter_asset_type: String?
}

struct SearchResultItem: Codable, Identifiable, Hashable, Equatable {
    let chunk_id: String
    let score: Double
    let text: String
    let asset_id: String
    let asset_path: String
    let evidence_anchor: String
    let topics: String?
    let summary: String?
    let sentiment: String?
    let entities: [String]?

    var id: String { chunk_id }

    var filename: String {
        URL(fileURLWithPath: asset_path).lastPathComponent
    }
}

struct EvidenceResponse: Codable {
    let asset_id: String
    let path: String
    let filename: String
    let mime_type: String?
    let size_bytes: Int
    let exists: Bool
    let evidence_anchor: EvidenceAnchorInfo?
    let chunk_text: String?

    struct EvidenceAnchorInfo: Codable {
        let asset_id: String?
        let page: Int?
        let chapter: String?
    }
}

struct HealthResponse: Codable {
    let status: String
    let lm_studio: String
    let vector_count: Int
    let db: String
    let chat_model: String?
    let embedding_model: String?
    let data_dir: String?
    let port: Int?
    let watched_volumes: [String]?
    let context_length: Int?
}

struct AssetInfo: Codable, Identifiable, Hashable, Equatable {
    let id: String
    let path: String
    let filename: String
    let mime_type: String?
    let size_bytes: Int
    let status: String
}

// MARK: - Universe / Concept Models

struct UniverseSnapshot: Codable {
    let lod: String
    let nodes: [UniverseNode]
    let edges: [UniverseEdge]
    let node_count: Int
    let edge_count: Int
}

struct UniverseNode: Codable, Identifiable, Hashable {
    let id: String
    let label: String
    let level: Int
    let type: String
    let size: Double
    let color: String
    let cluster: Int
    let description: String?
    let parent_id: String?

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        label = try c.decode(String.self, forKey: .label)
        level = try c.decode(Int.self, forKey: .level)
        type = try c.decode(String.self, forKey: .type)
        size = try c.decodeIfPresent(Double.self, forKey: .size) ?? 10
        color = try c.decodeIfPresent(String.self, forKey: .color) ?? "hsl(200,50%,50%)"
        cluster = try c.decodeIfPresent(Int.self, forKey: .cluster) ?? 0
        description = try c.decodeIfPresent(String.self, forKey: .description)
        parent_id = try c.decodeIfPresent(String.self, forKey: .parent_id)
    }

    private enum CodingKeys: String, CodingKey {
        case id, label, level, type, size, color, cluster, description, parent_id
    }
}

struct UniverseEdge: Codable {
    let source: String
    let target: String
    let weight: Double
    let type: String
}

struct ConceptInfo: Codable, Identifiable {
    let id: String
    let level: Int
    let label: String?
    let description: String?
    let parent_id: String?
    let exemplar_chunk_ids: [String]?
    let model_id: String?
    let created_at: String?
}

struct ConceptDetail: Codable {
    let id: String
    let level: Int
    let label: String?
    let description: String?
    let member_count: Int
    let members: [ConceptMember]

    struct ConceptMember: Codable, Identifiable {
        let chunk_id: String
        let text: String
        let asset_id: String
        let summary: String?
        var id: String { chunk_id }
    }
}

struct WhyExplanation: Codable {
    let concept_id: String
    let label: String?
    let description: String?
    let pipeline_version: String?
    let model_id: String?
    let evidence: [EvidenceItem]
    let explanation: String

    struct EvidenceItem: Codable, Identifiable {
        let chunk_id: String
        let chunk_text: String
        let asset_path: String?
        let asset_filename: String?
        let annotation_summary: String?
        let topics: [String]?
        var id: String { chunk_id }
    }
}
