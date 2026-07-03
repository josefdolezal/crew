package runtime

import "strings"

// Pi launches the pi coding agent interactively, task as the positional
// message argument. Pi has no permission-prompt system, so Yolo is a no-op.
type Pi struct{}

func (Pi) Name() string        { return "pi" }
func (Pi) SignInbound() bool   { return true }
func (Pi) WantsPreamble() bool { return true }
func (Pi) TaskAsArg() bool     { return true }

func (Pi) Command(spec Spec) string {
	parts := []string{"pi"}
	if spec.Model != "" {
		parts = append(parts, "--model", shellQuote(spec.Model))
	}
	if spec.Task != "" {
		parts = append(parts, shellQuote(spec.Task))
	}
	return strings.Join(parts, " ")
}

// Pi TUI patterns are not pinned down; rely on the daemon's
// activity-quiescence fallback.
func (Pi) Startup(screen string) StartupState { return StartupBooting }
func (Pi) LooksIdle(screen string) bool       { return true }
func (Pi) Attention(screen string) string     { return "" }
func (Pi) PreTrust(dir string) error          { return nil }
