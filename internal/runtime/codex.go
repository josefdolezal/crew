package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// Codex launches OpenAI Codex CLI interactively, task as the positional
// prompt argument.
type Codex struct{}

func (Codex) Name() string        { return "codex" }
func (Codex) SignInbound() bool   { return true }
func (Codex) WantsPreamble() bool { return true }
func (Codex) TaskAsArg() bool     { return true }

func (Codex) PreTrust(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	_, err = trustCodexConfig(filepath.Join(home, ".codex", "config.toml"), trustTarget(dir))
	return err
}

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

func (Codex) Startup(screen string) StartupState {
	// Directory-trust onboarding: Enter selects "Yes, continue". The
	// hooks-review dialog is NOT auto-confirmed (Enter would pick
	// "Review hooks"); it surfaces via Attention instead.
	if strings.Contains(screen, "Do you trust the contents of this directory") {
		return StartupDialog
	}
	if strings.Contains(screen, "Esc to interrupt") || (strings.Contains(screen, "send") && strings.Contains(screen, "⏎")) {
		return StartupReady
	}
	return StartupBooting
}

func (Codex) LooksIdle(screen string) bool {
	return !strings.Contains(screen, "Esc to interrupt")
}

// Attention catches codex confirmation dialogs (hooks review, approval
// prompts): a selected numbered option rendered as "› 1." plus an
// enter-to-confirm footer.
func (Codex) Attention(screen string) string {
	if strings.Contains(screen, "› 1.") && strings.Contains(screen, "Press enter") {
		return "confirmation dialog"
	}
	return ""
}
