package config

import "testing"

func TestGeminiSettingsComeFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("GEMINI_MODEL", " custom-primary ")
	t.Setenv("GEMINI_RESEARCH_MODEL", " custom-research ")
	t.Setenv("GEMINI_FALLBACK_MODEL", " custom-backup ")
	t.Setenv("GEMINI_SEARCH_ENABLED", "false")
	cfg := Load()
	if cfg.GeminiModel != "custom-primary" || cfg.GeminiResearchModel != "custom-research" || cfg.GeminiFallbackModel != "custom-backup" || cfg.GeminiSearchEnabled {
		t.Fatalf("incorrect model settings")
	}
	t.Setenv("GEMINI_SEARCH_ENABLED", "true")
	if !Load().GeminiSearchEnabled {
		t.Fatal("search setting ignored")
	}
	t.Setenv("GEMINI_SEARCH_ENABLED", "")
	t.Setenv("GEMINI_MODEL", "")
	cfg = Load()
	if cfg.GeminiSearchEnabled || cfg.GeminiModel != "" {
		t.Fatal("unexpected implicit model/search default")
	}
}
