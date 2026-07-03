package daemon

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/josefdolezal/crew/internal/proto"
	"github.com/josefdolezal/crew/internal/runtime"
)

const (
	startupDeadline = 90 * time.Second
	startupTick     = 500 * time.Millisecond
	genericReadyMin = 15 * time.Second // Booting this long + quiet output => assume ready
	genericQuietFor = 3 * time.Second
	idleAfter       = 30 * time.Second // wait fallback: no output activity this long
	watchdogTick    = 15 * time.Second
)

// newEpoch registers a fresh generation for an agent name and returns it.
// Epochs are globally monotonic, so a killed-and-respawned name never
// collides with a stale startup watcher from its previous life.
func (s *Server) newEpoch(name string) int {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.epochCounter++
	s.ready[name] = readyState{epoch: s.epochCounter}
	return s.epochCounter
}

// currentEpoch returns the live generation for a name (0 if none).
func (s *Server) currentEpoch(name string) int {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	return s.ready[name].epoch
}

// markReadyIfCurrent flips the agent to ready only when the caller still
// owns the live generation; a stale watcher gets false and must abort.
func (s *Server) markReadyIfCurrent(name string, epoch int) bool {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	st, ok := s.ready[name]
	if !ok || st.epoch != epoch {
		return false
	}
	st.ready = true
	s.ready[name] = st
	return true
}

func (s *Server) isReady(name string) bool {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	return s.ready[name].ready
}

func (s *Server) forgetReady(name string) {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	delete(s.ready, name)
}

// reconcile restores in-memory readiness after a daemon restart: any
// registered agent whose session is alive is long past startup, so the
// idle/attention machinery must treat it as ready.
func (s *Server) reconcile() {
	agents, err := s.store.List()
	if err != nil {
		log.Printf("reconcile: list agents: %v", err)
		return
	}
	for _, a := range agents {
		st, err := s.backend.State(a.Session)
		if err != nil || !st.Exists || st.ProcessDead {
			continue
		}
		epoch := s.newEpoch(a.Name)
		s.markReadyIfCurrent(a.Name, epoch)
		log.Printf("reconcile: %s is running, marked ready", a.Name)
	}
}

// watchStartup gates the startup of a fresh session: it confirms startup
// dialogs (folder trust etc.) when trusted, marks the agent ready once the
// REPL appears, and injects a pending task for runtimes that cannot take
// it as a launch argument. epoch guards against the agent being killed
// and respawned under the same name while this watcher is still alive.
func (s *Server) watchStartup(agent proto.Agent, rt runtime.Runtime, trust bool, pendingTask string, epoch int) {
	start := time.Now()
	var lastEnter time.Time
	finish := func(how string) {
		if !s.markReadyIfCurrent(agent.Name, epoch) {
			log.Printf("watchStartup %s: superseded by a newer spawn, aborting", agent.Name)
			return
		}
		if pendingTask != "" {
			if err := s.backend.SendInput(agent.Session, pendingTask); err != nil {
				log.Printf("watchStartup %s: inject pending task: %v", agent.Name, err)
			}
		}
		log.Printf("agent %s ready (%s, %.1fs)", agent.Name, how, time.Since(start).Seconds())
	}

	for time.Since(start) < startupDeadline {
		time.Sleep(startupTick)
		if s.currentEpoch(agent.Name) != epoch {
			log.Printf("watchStartup %s: superseded by a newer spawn, aborting", agent.Name)
			return
		}
		st, err := s.backend.State(agent.Session)
		if err != nil || !st.Exists || st.ProcessDead {
			log.Printf("watchStartup %s: session ended during startup", agent.Name)
			return
		}
		screen, err := s.backend.Snapshot(agent.Session)
		if err != nil {
			continue
		}
		switch rt.Startup(screen) {
		case runtime.StartupDialog:
			if trust && time.Since(lastEnter) > time.Second {
				if err := s.backend.SendKey(agent.Session, "Enter"); err != nil {
					log.Printf("watchStartup %s: confirm dialog: %v", agent.Name, err)
				} else {
					log.Printf("agent %s: confirmed startup dialog", agent.Name)
				}
				lastEnter = time.Now()
			}
		case runtime.StartupReady:
			finish("pattern")
			return
		case runtime.StartupBooting:
			if time.Since(start) > genericReadyMin {
				if activity, err := s.backend.ActivityAt(agent.Session); err == nil && time.Since(activity) > genericQuietFor {
					finish("quiescence")
					return
				}
			}
		}
	}
	// Deadline hit: stop gating rather than wedging the pending task forever.
	finish("deadline")
}

// watchdog notices agents whose process ended or session vanished, and
// agents blocked on an interactive prompt (attention). Each transition
// posts one event to the orchestrator's inbox.
func (s *Server) watchdog(stop <-chan struct{}) {
	last := map[string]proto.AgentStatus{}
	lastAttention := map[string]bool{}
	tick := time.NewTicker(watchdogTick)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		agents, err := s.store.List()
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, a := range agents {
			live := s.withLiveStatus(a)
			seen[a.Name] = true
			prev, known := last[a.Name]
			last[a.Name] = live.Status
			if known && prev == proto.StatusRunning && live.Status != proto.StatusRunning {
				body := "agent " + a.Name + " " + string(live.Status)
				if live.Status == proto.StatusExited {
					body += " (process ended without crew report; peek/logs for its final screen)"
				}
				s.postEvent(a.Parent, body)
			}
			if live.Status == proto.StatusRunning {
				s.checkAttention(a, lastAttention)
			}
		}
		for name := range last {
			if !seen[name] {
				delete(last, name)
				delete(lastAttention, name)
			}
		}
	}
}

// checkAttention posts one inbox event when an agent newly hits an
// interactive prompt, and rearms once the prompt is answered.
func (s *Server) checkAttention(a proto.Agent, lastAttention map[string]bool) {
	rt, err := runtime.Lookup(a.Runtime)
	if err != nil || !s.isReady(a.Name) {
		return
	}
	screen, err := s.backend.Snapshot(a.Session)
	if err != nil {
		return
	}
	reason := rt.Attention(screen)
	now := reason != ""
	was := lastAttention[a.Name]
	lastAttention[a.Name] = now
	if now && !was {
		s.postEvent(a.Parent, fmt.Sprintf(
			"agent %s needs attention: %s (crew peek %s to see it; crew send %s '1' or --key Enter to answer)",
			a.Name, reason, a.Name, a.Name))
	}
}

func (s *Server) postEvent(recipient, body string) {
	_, err := s.store.InsertMessage(proto.Message{
		Sender:    "system",
		Recipient: recipient,
		Kind:      "event",
		Body:      body,
		CreatedAt: time.Now(),
	})
	if err != nil {
		log.Printf("watchdog: post event: %v", err)
	} else {
		log.Printf("watchdog: %s", body)
	}
}

// screenTail returns the last n lines of a rendered screen.
func screenTail(screen string, n int) string {
	lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
