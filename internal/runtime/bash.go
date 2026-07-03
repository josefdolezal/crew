package runtime

import "strings"

// Bash launches a plain interactive shell. Model and yolo are ignored; a
// task, if given, is injected by the daemon once the prompt is ready.
type Bash struct{}

func (Bash) Name() string        { return "bash" }
func (Bash) SignInbound() bool   { return false }
func (Bash) WantsPreamble() bool { return false }
func (Bash) TaskAsArg() bool     { return false }

func (Bash) Command(spec Spec) string {
	return "bash -i"
}

func (Bash) Startup(screen string) StartupState {
	if promptVisible(screen) {
		return StartupReady
	}
	return StartupBooting
}

func (Bash) LooksIdle(screen string) bool   { return promptVisible(screen) }
func (Bash) Attention(screen string) string { return "" }

func promptVisible(screen string) bool {
	trimmed := strings.TrimRight(screen, " \n\t")
	return strings.HasSuffix(trimmed, "$") || strings.HasSuffix(trimmed, "#") || strings.HasSuffix(trimmed, "%")
}
