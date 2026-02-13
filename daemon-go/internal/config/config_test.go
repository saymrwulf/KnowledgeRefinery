package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Port != 8742 {
		t.Errorf("expected port 8742, got %d", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.LMStudio.BaseURL != "http://127.0.0.1:1234/v1" {
		t.Errorf("expected LM Studio URL, got %s", cfg.LMStudio.BaseURL)
	}
	if cfg.Pipeline.ChunkTargetTokens != 600 {
		t.Errorf("expected 600 target tokens, got %d", cfg.Pipeline.ChunkTargetTokens)
	}
	if cfg.Pipeline.MaxFileSizeBytes != 500*1024*1024 {
		t.Errorf("expected 500MB max file size, got %d", cfg.Pipeline.MaxFileSizeBytes)
	}
}

func TestLoadConfigEnvVars(t *testing.T) {
	t.Setenv("KR_DATA_DIR", "/tmp/test-kr-data")
	t.Setenv("KR_PORT", "9999")
	t.Setenv("KR_LM_STUDIO_URL", "http://localhost:5555/v1")

	cfg := LoadConfig()

	if cfg.DataDir != "/tmp/test-kr-data" {
		t.Errorf("expected data dir /tmp/test-kr-data, got %s", cfg.DataDir)
	}
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.LMStudio.BaseURL != "http://localhost:5555/v1" {
		t.Errorf("expected LM Studio URL override, got %s", cfg.LMStudio.BaseURL)
	}

	// Clean up
	os.RemoveAll("/tmp/test-kr-data")
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.DataDir = dir
	cfg.VectorDir = dir + "/vectors"
	cfg.ThumbnailsDir = dir + "/thumbnails"
	cfg.TempDir = dir + "/tmp"

	cfg.EnsureDirs()

	for _, d := range []string{cfg.VectorDir, cfg.ThumbnailsDir, cfg.TempDir} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", d)
		}
	}
}
