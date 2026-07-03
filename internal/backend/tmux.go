package backend

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Tmux implements Backend on top of tmux (>= 3.2, for new-session -e).
type Tmux struct {
	bin string
}

func NewTmux() (*Tmux, error) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return nil, fmt.Errorf("tmux not found in PATH: %w", err)
	}
	return &Tmux{bin: bin}, nil
}

func (t *Tmux) Name() string { return "tmux" }

func (t *Tmux) run(args ...string) (string, error) {
	out, err := exec.Command(t.bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (t *Tmux) Spawn(spec SessionSpec) error {
	args := []string{"new-session", "-d", "-s", spec.Session, "-c", spec.Cwd}
	for k, v := range spec.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, spec.Command)
	if _, err := t.run(args...); err != nil {
		return err
	}
	// Keep the pane around after the runtime exits so its final output
	// stays inspectable; State reports it as ProcessDead.
	if _, err := t.run("set-option", "-t", spec.Session, "remain-on-exit", "on"); err != nil {
		return err
	}
	if spec.LogFile != "" {
		pipeCmd := fmt.Sprintf("cat >> %s", shellQuote(spec.LogFile))
		if _, err := t.run("pipe-pane", "-o", "-t", spec.Session, pipeCmd); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tmux) SendInput(session, text string) error {
	// -l sends the text literally so tmux never interprets key names or
	// control sequences embedded in the payload.
	if _, err := t.run("send-keys", "-t", session, "-l", text); err != nil {
		return err
	}
	// TUI composers with paste-burst detection (codex) fold an Enter that
	// arrives within the same input burst into the pasted text instead of
	// submitting - the message then sits rendered-but-unsubmitted forever.
	// A short pause separates the submit keypress from the paste.
	time.Sleep(300 * time.Millisecond)
	_, err := t.run("send-keys", "-t", session, "Enter")
	return err
}

func (t *Tmux) SendKey(session, key string) error {
	_, err := t.run("send-keys", "-t", session, key)
	return err
}

func (t *Tmux) Snapshot(session string) (string, error) {
	if err := t.ensure(session); err != nil {
		return "", err
	}
	out, err := t.run("capture-pane", "-p", "-J", "-t", session)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n") + "\n", nil
}

func (t *Tmux) ActivityAt(session string) (time.Time, error) {
	if err := t.ensure(session); err != nil {
		return time.Time{}, err
	}
	out, err := t.run("display-message", "-p", "-t", session, "#{window_activity}")
	if err != nil {
		return time.Time{}, err
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse window_activity: %w", err)
	}
	return time.Unix(sec, 0), nil
}

func (t *Tmux) State(session string) (State, error) {
	if err := t.ensure(session); err != nil {
		return State{Exists: false}, nil
	}
	out, err := t.run("display-message", "-p", "-t", session, "#{pane_dead}")
	if err != nil {
		return State{Exists: true}, err
	}
	return State{Exists: true, ProcessDead: strings.TrimSpace(out) == "1"}, nil
}

func (t *Tmux) Kill(session string) error {
	if err := t.ensure(session); err != nil {
		return err
	}
	_, err := t.run("kill-session", "-t", session)
	return err
}

func (t *Tmux) AttachArgs(session string) []string {
	return []string{t.bin, "attach-session", "-t", session}
}

func (t *Tmux) ensure(session string) error {
	if err := exec.Command(t.bin, "has-session", "-t", "="+session).Run(); err != nil {
		return ErrNoSession
	}
	return nil
}

// shellQuote wraps s in single quotes, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
