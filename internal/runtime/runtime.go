// Package runtime maps a runtime name (claude, codex, pi, bash) to the
// concrete launch command for an agent session, plus screen-probing
// heuristics the daemon uses for readiness and idle detection. Adding a
// runtime = one small file implementing Runtime and a case in Lookup.
package runtime

import (
	"fmt"
	"strings"
)

// Spec carries the per-spawn options an adapter may honor.
type Spec struct {
	Model string
	Task  string
	Yolo  bool // skip permission prompts, where the runtime supports it
}

// StartupState classifies a rendered screen during session startup.
type StartupState int

const (
	StartupBooting StartupState = iota // nothing recognizable yet
	StartupDialog                      // a confirm dialog blocks the REPL (e.g. folder trust)
	StartupReady                       // REPL is ready for input
)

// Runtime builds the shell command line that launches an agent runtime
// interactively (sessions must stay attachable) and interprets its screen.
type Runtime interface {
	Name() string
	Command(spec Spec) string
	// SignInbound reports whether injected messages should carry a
	// "[sender]" prefix. LLM runtimes want to know who is speaking; a
	// shell would try to execute the prefix.
	SignInbound() bool
	// WantsPreamble reports whether the crew protocol preamble should be
	// prepended to the task (LLM runtimes yes, shells no).
	WantsPreamble() bool
	// TaskAsArg reports whether the task can be passed as a launch
	// argument. Otherwise the daemon injects it once the REPL is ready.
	TaskAsArg() bool
	// Startup classifies the rendered screen while the session boots.
	// Return StartupBooting when unsure; the daemon has a generic
	// activity-quiescence fallback.
	Startup(screen string) StartupState
	// LooksIdle reports whether the rendered screen looks like the
	// runtime is waiting for input (used with activity quiescence as the
	// wait fallback when an agent never reports).
	LooksIdle(screen string) bool
	// Attention returns a non-empty reason when the screen shows an
	// interactive prompt blocking the agent mid-task (e.g. a permission
	// dialog). The daemon bridges it to the orchestrator: `crew wait`
	// returns early with outcome "attention" and the watchdog posts an
	// inbox event. Return "" when unsure.
	Attention(screen string) string
	// PreTrust persists trust for dir in the runtime's own config before
	// launch, so its folder-trust dialog never appears. Best-effort: on
	// error the startup watcher still auto-confirms dialogs on screen.
	PreTrust(dir string) error
}

func Lookup(name string) (Runtime, error) {
	switch name {
	case "claude":
		return Claude{}, nil
	case "codex":
		return Codex{}, nil
	case "pi":
		return Pi{}, nil
	case "bash", "shell":
		return Bash{}, nil
	default:
		return nil, fmt.Errorf("unknown runtime %q (supported: claude, codex, pi, bash)", name)
	}
}

// WithPreamble prepends the crew protocol contract to a task. It
// references $CREW_BIN (exported into the session) so it works even when
// crew is not on PATH.
func WithPreamble(agentName, task string) string {
	return fmt.Sprintf(`You are crew agent %q, spawned by an orchestrator to complete the task below.

Protocol (the crew CLI is available as "$CREW_BIN"):
- When the task is finished, run: "$CREW_BIN" report --status done -m "<one-line summary of the outcome>"
- If you cannot proceed, run: "$CREW_BIN" report --status blocked -m "<what you need>"
- To message your orchestrator mid-task, run: "$CREW_BIN" send parent "<text>"
- Messages prefixed like [orchestrator@...] arrive from your orchestrator; treat them as instructions.
- Do not exit the session yourself; the orchestrator terminates it.

Task:
%s`, agentName, task)
}

// shellQuote wraps s in single quotes, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
