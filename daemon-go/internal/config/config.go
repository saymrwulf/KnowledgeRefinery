package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type LMStudioConfig struct {
	BaseURL           string  `json:"base_url"`
	Timeout           float64 `json:"timeout"`
	MaxRetries        int     `json:"max_retries"`
	EmbeddingBatchSize int    `json:"embedding_batch_size"`
}

type PipelineConfig struct {
	Version                 string `json:"version"`
	ChunkTargetTokens       int    `json:"chunk_target_tokens"`
	ChunkMinTokens          int    `json:"chunk_min_tokens"`
	ChunkMaxTokens          int    `json:"chunk_max_tokens"`
	ChunkOverlapTokens      int    `json:"chunk_overlap_tokens"`
	MaxConcurrentExtractions int   `json:"max_concurrent_extractions"`
	MaxConcurrentEmbeddings  int   `json:"max_concurrent_embeddings"`
	MaxFileSizeBytes        int64  `json:"max_file_size_bytes"`
	ScanBatchSize           int    `json:"scan_batch_size"`
}

type SandboxConfig struct {
	MaxOutputBytes    int64 `json:"max_output_bytes"`
	MaxFiles          int   `json:"max_files"`
	MaxRecursionDepth int   `json:"max_recursion_depth"`
	MaxCPUSeconds     int   `json:"max_cpu_seconds"`
	MaxRSSBytes       int64 `json:"max_rss_bytes"`
}

type Config struct {
	DataDir       string         `json:"data_dir"`
	DBPath        string         `json:"db_path"`
	VectorDir     string         `json:"vector_dir"`
	ThumbnailsDir string         `json:"thumbnails_dir"`
	TempDir       string         `json:"temp_dir"`
	Host          string         `json:"host"`
	Port          int            `json:"port"`
	LMStudio      LMStudioConfig `json:"lm_studio"`
	Pipeline      PipelineConfig `json:"pipeline"`
	Sandbox       SandboxConfig  `json:"sandbox"`
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".knowledge-refinery")
	return Config{
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "refinery.db"),
		VectorDir:     filepath.Join(dataDir, "vectors"),
		ThumbnailsDir: filepath.Join(dataDir, "thumbnails"),
		TempDir:       filepath.Join(dataDir, "tmp"),
		Host:          "127.0.0.1",
		Port:          8742,
		LMStudio: LMStudioConfig{
			BaseURL:           "http://127.0.0.1:1234/v1",
			Timeout:           120.0,
			MaxRetries:        3,
			EmbeddingBatchSize: 32,
		},
		Pipeline: PipelineConfig{
			Version:                 "v1.0",
			ChunkTargetTokens:       600,
			ChunkMinTokens:          400,
			ChunkMaxTokens:          800,
			ChunkOverlapTokens:      50,
			MaxConcurrentExtractions: 4,
			MaxConcurrentEmbeddings:  2,
			MaxFileSizeBytes:        500 * 1024 * 1024,
			ScanBatchSize:           1000,
		},
		Sandbox: SandboxConfig{
			MaxOutputBytes:    100 * 1024 * 1024,
			MaxFiles:          10000,
			MaxRecursionDepth: 5,
			MaxCPUSeconds:     300,
			MaxRSSBytes:       2 * 1024 * 1024 * 1024,
		},
	}
}

func LoadConfig() Config {
	cfg := DefaultConfig()

	if dataDir := os.Getenv("KR_DATA_DIR"); dataDir != "" {
		cfg.DataDir = dataDir
		cfg.DBPath = filepath.Join(dataDir, "refinery.db")
		cfg.VectorDir = filepath.Join(dataDir, "vectors")
		cfg.ThumbnailsDir = filepath.Join(dataDir, "thumbnails")
		cfg.TempDir = filepath.Join(dataDir, "tmp")
	}
	if lmURL := os.Getenv("KR_LM_STUDIO_URL"); lmURL != "" {
		cfg.LMStudio.BaseURL = lmURL
	}
	if port := os.Getenv("KR_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	cfg.EnsureDirs()
	return cfg
}

func (c *Config) EnsureDirs() {
	for _, d := range []string{c.DataDir, c.VectorDir, c.ThumbnailsDir, c.TempDir} {
		os.MkdirAll(d, 0o755)
	}
}
