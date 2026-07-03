package runtime

import "strings"

// Claude launches Claude Code interactively. The task is passed as the
// positional prompt argument, which starts the interactive REPL with the
// prompt already submitted - no send-keys race on startup.
type Claude struct{}

func (Claude) Name() string        { return "claude" }
func (Claude) SignInbound() bool   { return true }
func (Claude) WantsPreamble() bool { return true }
func (Claude) TaskAsArg() bool     { return true }

func (Claude) Command(spec Spec) string {
	parts := []string{"claude"}
	if spec.Model != "" {
		parts = append(parts, "--model", shellQuote(spec.Model))
	}
	if spec.Yolo {
		parts = append(parts, "--dangerously-skip-permissions")
	}
	if spec.Task != "" {
		parts = append(parts, shellQuote(spec.Task))
	}
	return strings.Join(parts, " ")
}

func (Claude) Startup(screen string) StartupState {
	// Startup dialogs (folder trust, theme picker, ...) all render an
	// "Enter to confirm" footer before the REPL exists.
	if strings.Contains(screen, "Enter to confirm") {
		return StartupDialog
	}
	// "? for shortcuts" = idle REPL; "esc to interrupt" = REPL already
	// working (task was submitted as the launch argument).
	if strings.Contains(screen, "? for shortcuts") || strings.Contains(screen, "esc to interrupt") {
		return StartupReady
	}
	return StartupBooting
}

func (Claude) LooksIdle(screen string) bool {
	return strings.Contains(screen, "? for shortcuts") && !strings.Contains(screen, "esc to interrupt")
}

// Attention detects permission / confirmation dialogs mid-task: tool
// permission prompts ("Do you want to proceed?", "Do you want to make
// this edit...") and plan approval ("Would you like to proceed?"). All
// render a selected numbered option ("❯ 1.").
func (Claude) Attention(screen string) string {
	hasQuestion := strings.Contains(screen, "Do you want") || strings.Contains(screen, "Would you like")
	if hasQuestion && strings.Contains(screen, "❯ 1.") {
		return "permission prompt"
	}
	return ""
}
