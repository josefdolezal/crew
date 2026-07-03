package runtime

import (
	"strings"
	"testing"
)

func TestClaudeCommand(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{"bare", Spec{}, "claude"},
		{"model", Spec{Model: "haiku"}, "claude --model 'haiku'"},
		{"yolo", Spec{Yolo: true}, "claude --dangerously-skip-permissions"},
		{"task", Spec{Task: "fix the bug"}, "claude 'fix the bug'"},
		{
			"task with single quotes",
			Spec{Task: "don't break"},
			`claude 'don'"'"'t break'`,
		},
		{
			"all",
			Spec{Model: "haiku", Task: "go", Yolo: true},
			"claude --model 'haiku' --dangerously-skip-permissions 'go'",
		},
	}
	for _, tt := range tests {
		if got := (Claude{}).Command(tt.spec); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCodexCommand(t *testing.T) {
	got := (Codex{}).Command(Spec{Model: "gpt-5", Task: "go", Yolo: true})
	want := "codex --model 'gpt-5' --dangerously-bypass-approvals-and-sandbox 'go'"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClaudeStartup(t *testing.T) {
	c := Claude{}
	if got := c.Startup("... Enter to confirm · Esc to cancel"); got != StartupDialog {
		t.Errorf("trust dialog: got %v, want StartupDialog", got)
	}
	if got := c.Startup("❯ input\n? for shortcuts"); got != StartupReady {
		t.Errorf("idle repl: got %v, want StartupReady", got)
	}
	if got := c.Startup("✻ Thinking (esc to interrupt)"); got != StartupReady {
		t.Errorf("working repl: got %v, want StartupReady", got)
	}
	if got := c.Startup("loading..."); got != StartupBooting {
		t.Errorf("booting: got %v, want StartupBooting", got)
	}
	if c.LooksIdle("✻ Thinking (esc to interrupt)\n? for shortcuts") {
		t.Error("working screen should not look idle")
	}
	if !c.LooksIdle("❯ \n? for shortcuts") {
		t.Error("idle screen should look idle")
	}
}

func TestClaudeAttention(t *testing.T) {
	c := Claude{}
	perm := "Bash command\n  rm -rf build\n  Do you want to proceed?\n❯ 1. Yes\n  2. No"
	if got := c.Attention(perm); got == "" {
		t.Error("permission prompt should need attention")
	}
	plan := "Would you like to proceed?\n❯ 1. Yes, and auto-accept edits\n  2. No"
	if got := c.Attention(plan); got == "" {
		t.Error("plan approval should need attention")
	}
	if got := c.Attention("⏺ I want to explain...\n? for shortcuts"); got != "" {
		t.Errorf("idle screen should not need attention, got %q", got)
	}
}

func TestWithPreamble(t *testing.T) {
	got := WithPreamble("worker-1", "fix the bug")
	for _, want := range []string{`agent "worker-1"`, "$CREW_BIN", "report --status done", "send parent", "fix the bug"} {
		if !contains(got, want) {
			t.Errorf("preamble missing %q", want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}

func TestLookup(t *testing.T) {
	for _, name := range []string{"claude", "codex", "pi", "bash", "shell"} {
		if _, err := Lookup(name); err != nil {
			t.Errorf("Lookup(%q): %v", name, err)
		}
	}
	if _, err := Lookup("nope"); err == nil {
		t.Error("Lookup(nope): expected error")
	}
}
