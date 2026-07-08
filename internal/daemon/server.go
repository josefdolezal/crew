// Package daemon implements the crew coordination server. It listens on a
// unix socket (filesystem permissions are the auth boundary), owns the
// SQLite registry, and drives the session backend. Sessions live in the
// backend (tmux), not in this process: a daemon restart never kills agents.
package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/josefdolezal/crew/internal/backend"
	"github.com/josefdolezal/crew/internal/config"
	"github.com/josefdolezal/crew/internal/gitx"
	"github.com/josefdolezal/crew/internal/proto"
	"github.com/josefdolezal/crew/internal/runtime"
	"github.com/josefdolezal/crew/internal/store"
)

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

type Server struct {
	home    string
	store   *store.Store
	backend backend.Backend
	http    *http.Server

	readyMu      sync.Mutex
	ready        map[string]readyState
	epochCounter int
	stop         chan struct{}
}

// readyState tracks whether an agent's REPL came up, tagged with the
// spawn generation that owns it.
type readyState struct {
	epoch int
	ready bool
}

func New(home string) (*Server, error) {
	st, err := store.Open(config.DBPath(home))
	if err != nil {
		return nil, err
	}
	be, err := backend.NewTmux()
	if err != nil {
		st.Close()
		return nil, err
	}
	return &Server{
		home:    home,
		store:   st,
		backend: be,
		ready:   map[string]readyState{},
		stop:    make(chan struct{}),
	}, nil
}

// Run serves until the process is signaled or /shutdown is called.
func (s *Server) Run() error {
	sock := config.SocketPath(s.home)
	// Remove a stale socket from a previous daemon; if another daemon is
	// live on it, fail the bind check below instead of stealing it.
	if _, err := net.Dial("unix", sock); err == nil {
		return fmt.Errorf("another daemon is already listening on %s", sock)
	}
	_ = os.Remove(sock)

	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /agents", s.handleSpawn)
	mux.HandleFunc("GET /agents", s.handleList)
	mux.HandleFunc("GET /agents/{name}", s.handleGet)
	mux.HandleFunc("DELETE /agents/{name}", s.handleKill)
	mux.HandleFunc("GET /agents/{name}/snapshot", s.handleSnapshot)
	mux.HandleFunc("POST /agents/{name}/input", s.handleSend)
	mux.HandleFunc("GET /agents/{name}/wait", s.handleWait)
	mux.HandleFunc("POST /report", s.handleReport)
	mux.HandleFunc("POST /route", s.handleRoute)
	mux.HandleFunc("GET /inbox", s.handleInbox)
	mux.HandleFunc("POST /adopt", s.handleAdopt)
	mux.HandleFunc("DELETE /adopt", s.handleUnadopt)
	mux.HandleFunc("POST /shutdown", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "shutting down"})
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.http.Close()
		}()
	})

	s.reconcile()
	go s.watchdog(s.stop)

	s.http = &http.Server{Handler: mux}
	log.Printf("crew daemon listening on %s", sock)
	err = s.http.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	close(s.stop)
	s.store.Close()
	_ = os.Remove(sock)
	return err
}

func (s *Server) handleSpawn(w http.ResponseWriter, r *http.Request) {
	var req proto.SpawnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	if !nameRe.MatchString(req.Name) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid agent name %q (alphanumeric, dash, underscore)", req.Name))
		return
	}
	if _, err := s.store.Get(req.Name); err == nil {
		writeErr(w, http.StatusConflict, fmt.Errorf("agent %q already exists (crew kill %s first)", req.Name, req.Name))
		return
	}
	rt, err := runtime.Lookup(req.Runtime)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if fi, err := os.Stat(req.Cwd); err != nil || !fi.IsDir() {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("cwd %q is not a directory", req.Cwd))
		return
	}

	if req.Trust {
		if err := rt.PreTrust(req.Cwd); err != nil {
			log.Printf("pre-trust %s for %s: %v (startup watcher will handle dialogs)", req.Cwd, req.Name, err)
		}
	}

	task := req.Task
	if task != "" && rt.WantsPreamble() {
		task = runtime.WithPreamble(req.Name, task)
	}
	// Runtimes that can't take the task as a launch argument get it
	// injected by the startup watcher once the REPL is ready.
	argTask, pendingTask := task, ""
	if !rt.TaskAsArg() {
		argTask, pendingTask = "", task
	}

	crewBin, err := os.Executable()
	if err != nil {
		crewBin = "crew"
	}
	agent := proto.Agent{
		Name:      req.Name,
		Runtime:   req.Runtime,
		Model:     req.Model,
		Cwd:       req.Cwd,
		Parent:    req.Parent,
		Task:      req.Task,
		Backend:   s.backend.Name(),
		Session:   "crew:" + req.Name,
		Status:    proto.StatusRunning,
		Worktree:  req.Worktree,
		CreatedAt: time.Now(),
	}
	spec := backend.SessionSpec{
		Session: agent.Session,
		Command: rt.Command(runtime.Spec{Model: req.Model, Task: argTask, Yolo: req.Yolo}),
		Cwd:     req.Cwd,
		Env: map[string]string{
			"CREW_AGENT_NAME": agent.Name,
			"CREW_PARENT":     agent.Parent,
			"CREW_HOME":       s.home,
			"CREW_BIN":        crewBin,
		},
		LogFile: config.AgentLog(s.home, agent.Name),
	}
	if err := s.backend.Spawn(spec); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("spawn session: %w", err))
		return
	}
	if err := s.store.Insert(agent); err != nil {
		_ = s.backend.Kill(agent.Session)
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("persist agent: %w", err))
		return
	}
	// A re-used name must not inherit a previous agent's reports.
	_ = s.store.DeleteMessagesFrom(agent.Name)
	go s.watchStartup(agent, rt, req.Trust, pendingTask, s.newEpoch(agent.Name))
	log.Printf("spawned %s (runtime=%s model=%s parent=%s cwd=%s)", agent.Name, agent.Runtime, agent.Model, agent.Parent, agent.Cwd)
	writeJSON(w, http.StatusCreated, s.withLiveStatus(agent))
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	parent := r.URL.Query().Get("parent")
	out := make([]proto.Agent, 0, len(agents))
	for _, a := range agents {
		if parent != "" && a.Parent != parent {
			continue
		}
		out = append(out, s.withLiveStatus(a))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentOr404(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.withLiveStatus(agent))
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentOr404(w, r)
	if !ok {
		return
	}
	if err := s.backend.Kill(agent.Session); err != nil && !errors.Is(err, backend.ErrNoSession) {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("kill session: %w", err))
		return
	}
	if err := s.store.Delete(agent.Name); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Deliberately keep the agent's messages: an undrained report must
	// survive the kill. Spawn cleans up reports when the name is reused.
	s.forgetReady(agent.Name)
	resp := map[string]string{"status": "killed", "name": agent.Name}
	if agent.Worktree != "" {
		result, err := gitx.RemoveWorktreeIfClean(agent.Worktree)
		resp["worktree"] = string(result)
		if err != nil {
			resp["worktree_note"] = err.Error()
		} else if result == gitx.Kept {
			resp["worktree_note"] = agent.Worktree + " has uncommitted changes; left in place"
		}
	}
	log.Printf("killed %s", agent.Name)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentOr404(w, r)
	if !ok {
		return
	}
	screen, err := s.backend.Snapshot(agent.Session)
	if err != nil {
		if errors.Is(err, backend.ErrNoSession) {
			writeErr(w, http.StatusGone, fmt.Errorf("agent %q session is gone", agent.Name))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	activity, _ := s.backend.ActivityAt(agent.Session)
	live := s.withLiveStatus(agent)
	writeJSON(w, http.StatusOK, proto.Snapshot{
		Name:       agent.Name,
		Screen:     screen,
		ActivityAt: activity,
		Status:     live.Status,
	})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentOr404(w, r)
	if !ok {
		return
	}
	var req proto.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	var err2 error
	if req.Key != "" {
		err2 = s.backend.SendKey(agent.Session, req.Key)
	} else {
		err2 = s.injectToAgent(agent, req.From, req.Text)
	}
	if err := err2; err != nil {
		if errors.Is(err, backend.ErrNoSession) {
			writeErr(w, http.StatusGone, fmt.Errorf("agent %q session is gone", agent.Name))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent", "name": agent.Name})
}

func (s *Server) agentOr404(w http.ResponseWriter, r *http.Request) (proto.Agent, bool) {
	agent, err := s.store.Get(r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, fmt.Errorf("agent %q not found", r.PathValue("name")))
		return agent, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return agent, false
	}
	return agent, true
}

// withLiveStatus computes status from the backend on every read; the DB
// intentionally stores no status column so it can never go stale.
func (s *Server) withLiveStatus(a proto.Agent) proto.Agent {
	st, err := s.backend.State(a.Session)
	switch {
	case err != nil || !st.Exists:
		a.Status = proto.StatusGone
	case st.ProcessDead:
		a.Status = proto.StatusExited
	default:
		a.Status = proto.StatusRunning
	}
	return a
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, proto.ErrorResponse{Error: err.Error()})
}
