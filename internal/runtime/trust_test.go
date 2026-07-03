package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustClaudeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude.json")

	// Fresh file.
	wrote, err := trustClaudeConfig(path, "/repo/a")
	if err != nil || !wrote {
		t.Fatalf("fresh: wrote=%v err=%v", wrote, err)
	}

	// Existing config: preserve unrelated keys, add second project.
	seed := map[string]any{
		"oauthAccount": map[string]any{"token": "keep-me"},
		"projects": map[string]any{
			"/repo/a": map[string]any{"hasTrustDialogAccepted": true, "history": []any{"x"}},
		},
	}
	b, _ := json.Marshal(seed)
	os.WriteFile(path, b, 0o600)

	wrote, err = trustClaudeConfig(path, "/repo/b")
	if err != nil || !wrote {
		t.Fatalf("add project: wrote=%v err=%v", wrote, err)
	}
	var cfg map[string]any
	b, _ = os.ReadFile(path)
	json.Unmarshal(b, &cfg)
	if cfg["oauthAccount"].(map[string]any)["token"] != "keep-me" {
		t.Error("unrelated keys must be preserved")
	}
	projects := cfg["projects"].(map[string]any)
	if projects["/repo/a"].(map[string]any)["history"] == nil {
		t.Error("existing project fields must be preserved")
	}
	if projects["/repo/b"].(map[string]any)["hasTrustDialogAccepted"] != true {
		t.Error("new project must be trusted")
	}

	// Already trusted: no write.
	if wrote, err = trustClaudeConfig(path, "/repo/a"); err != nil || wrote {
		t.Fatalf("already trusted: wrote=%v err=%v", wrote, err)
	}
}

func TestTrustCodexConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("model = \"gpt-5\"\n\n[projects.\"/repo/a\"]\ntrust_level = \"trusted\"\n"), 0o600)

	// Existing section: untouched.
	if wrote, err := trustCodexConfig(path, "/repo/a"); err != nil || wrote {
		t.Fatalf("existing: wrote=%v err=%v", wrote, err)
	}

	// New section appended, prior content intact.
	wrote, err := trustCodexConfig(path, "/repo/b")
	if err != nil || !wrote {
		t.Fatalf("append: wrote=%v err=%v", wrote, err)
	}
	b, _ := os.ReadFile(path)
	text := string(b)
	for _, want := range []string{"model = \"gpt-5\"", "[projects.\"/repo/a\"]", "[projects.\"/repo/b\"]\ntrust_level = \"trusted\"\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("config missing %q:\n%s", want, text)
		}
	}
}
