// Package backend abstracts the session substrate that hosts agent
// runtimes. The tmux backend ships first; an internal PTY backend
// (creack/pty + embedded VT emulator) can be added behind the same
// interface later.
package backend

import (
	"errors"
	"time"
)

var ErrNoSession = errors.New("session does not exist")

// SessionSpec describes a session to create.
type SessionSpec struct {
	// Session is the backend-level session identifier (e.g. tmux session name).
	Session string
	// Command is the full shell command line the session runs.
	Command string
	// Cwd is the working directory for the command.
	Cwd string
	// Env vars exported into the session (CREW_AGENT_NAME etc.).
	Env map[string]string
	// LogFile receives the raw output stream, if non-empty.
	LogFile string
}

// State reports whether a session and its process are alive.
type State struct {
	Exists      bool
	ProcessDead bool // session exists but the runtime process exited
}

// Backend is the session substrate contract.
type Backend interface {
	Name() string
	Spawn(spec SessionSpec) error
	SendInput(session, text string) error
	// SendKey sends a single named key (e.g. "Enter") without text,
	// used to confirm startup dialogs.
	SendKey(session, key string) error
	// Snapshot returns the rendered screen contents (not the raw byte stream).
	Snapshot(session string) (string, error)
	// ActivityAt returns the time of the last output activity.
	ActivityAt(session string) (time.Time, error)
	State(session string) (State, error)
	Kill(session string) error
	// AttachArgs returns argv to exec in the user's terminal to attach.
	AttachArgs(session string) []string
}
