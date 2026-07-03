package runtime

import "strings"

// Codex launches OpenAI Codex CLI interactively, task as the positional
// prompt argument.
type Codex struct{}

func (Codex) Name() string        { return "codex" }
func (Codex) SignInbound() bool   { return true }
func (Codex) WantsPreamble() bool { return true }
func (Codex) TaskAsArg() bool     { return true }

func (Codex) Command(spec Spec) string {
	parts := []string{"codex"}
	if spec.Model != "" {
		parts = append(parts, "--model", shellQuote(spec.Model))
	}
	if spec.Yolo {
		parts = append(parts, "--dangerously-bypass-approvals-and-sandbox")
	}
	if spec.Task != "" {
		parts = append(parts, shellQuote(spec.Task))
	}
	return strings.Join(parts, " ")
}

// Codex TUI patterns are not pinned down; report Booting/uncertain and
// let the daemon's activity-quiescence fallback decide readiness.
func (Codex) Startup(screen string) StartupState {
	if strings.Contains(screen, "Esc to interrupt") || (strings.Contains(screen, "send") && strings.Contains(screen, "⏎")) {
		return StartupReady
	}
	return StartupBooting
}

func (Codex) LooksIdle(screen string) bool {
	return !strings.Contains(screen, "Esc to interrupt")
}

// Codex approval-prompt patterns are not pinned down yet.
func (Codex) Attention(screen string) string { return "" }
