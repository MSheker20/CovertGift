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

func TestAllowDBResetDefaultsToOff(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ALLOW_DB_RESET", "")
	if Load().AllowDBReset {
		t.Fatal("reset must be off when ALLOW_DB_RESET is unset")
	}
	t.Setenv("ALLOW_DB_RESET", "false")
	if Load().AllowDBReset {
		t.Fatal("reset must be off when ALLOW_DB_RESET=false")
	}
	t.Setenv("ALLOW_DB_RESET", "true")
	if !Load().AllowDBReset {
		t.Fatal("ALLOW_DB_RESET=true ignored")
	}
}
