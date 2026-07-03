// Package proto holds the request/response types shared between the crew
// CLI client and the daemon. It has no dependencies on either side.
package proto

import "time"

// AgentStatus is the daemon's view of an agent session.
type AgentStatus string

const (
	StatusRunning AgentStatus = "running" // session alive, runtime process running
	StatusExited  AgentStatus = "exited"  // session alive, runtime process ended (remain-on-exit)
	StatusGone    AgentStatus = "gone"    // backend session no longer exists
)

// Agent is the registry record for a spawned session.
type Agent struct {
	Name    string      `json:"name"`
	Runtime string      `json:"runtime"`
	Model   string      `json:"model,omitempty"`
	Cwd     string      `json:"cwd"`
	Parent  string      `json:"parent"`
	Task    string      `json:"task,omitempty"`
	Backend string      `json:"backend"`
	Session string      `json:"session"`
	Status  AgentStatus `json:"status"`
	// Worktree is the path of the git worktree created for this agent by
	// `spawn --worktree` (empty otherwise). Kill removes it if clean.
	Worktree  string    `json:"worktree,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SpawnRequest asks the daemon to create a new agent session.
type SpawnRequest struct {
	Name    string `json:"name"`
	Runtime string `json:"runtime"`
	Model   string `json:"model,omitempty"`
	Cwd     string `json:"cwd"`
	Parent  string `json:"parent"`
	Task    string `json:"task,omitempty"`
	Yolo    bool   `json:"yolo,omitempty"`
	// Trust auto-confirms runtime startup dialogs (e.g. Claude's folder
	// trust prompt). The orchestrator chose the cwd deliberately.
	Trust bool `json:"trust,omitempty"`
	// Worktree is the path of a git worktree the client created for this
	// agent; recorded so kill can clean it up.
	Worktree string `json:"worktree,omitempty"`
}

// SendRequest injects text into an agent's stdin, or a single named key
// (e.g. "Enter", "Escape", "Down") when Key is set - used to answer
// interactive dialogs deterministically.
type SendRequest struct {
	Text string `json:"text"`
	From string `json:"from"`
	Key  string `json:"key,omitempty"`
}

// Snapshot is the rendered screen state of an agent session.
type Snapshot struct {
	Name       string      `json:"name"`
	Screen     string      `json:"screen"`
	ActivityAt time.Time   `json:"activity_at"`
	Status     AgentStatus `json:"status"`
}

// Message is an inbox entry: a routed message, an agent report, or a
// system event (e.g. "agent exited").
type Message struct {
	ID        int64      `json:"id"`
	Sender    string     `json:"sender"`
	Recipient string     `json:"recipient"`
	Kind      string     `json:"kind"`             // message | report | event
	Status    string     `json:"status,omitempty"` // reports: done | blocked
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}

// ReportRequest is an agent declaring its task outcome.
type ReportRequest struct {
	From    string `json:"from"`
	Status  string `json:"status"` // done | blocked
	Message string `json:"message"`
}

// PostRequest routes a message to a non-agent recipient's inbox
// (typically an agent messaging its orchestrator).
type PostRequest struct {
	From      string `json:"from"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
}

// WaitOutcome says why a wait returned.
type WaitOutcome string

const (
	WaitDone    WaitOutcome = "done"    // agent reported done
	WaitBlocked WaitOutcome = "blocked" // agent reported blocked
	WaitReady   WaitOutcome = "ready"   // runtime REPL is ready (--for ready)
	WaitExited  WaitOutcome = "exited"  // runtime process ended without reporting
	WaitIdle    WaitOutcome = "idle"    // LLM looks idle but never reported (fallback)
	// WaitAttention means the agent is blocked on an interactive prompt
	// (e.g. a permission dialog); answer it via `crew send` / attach,
	// then wait again.
	WaitAttention WaitOutcome = "attention"
	WaitTimeout   WaitOutcome = "timeout"
)

// WaitResult is the response of a blocking wait.
type WaitResult struct {
	Name    string      `json:"name"`
	Outcome WaitOutcome `json:"outcome"`
	Report  *Message    `json:"report,omitempty"`
	Screen  string      `json:"screen,omitempty"` // tail of the rendered screen for exited/idle/timeout
	Elapsed float64     `json:"elapsed_seconds"`
}

// ErrorResponse is the daemon's error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}
