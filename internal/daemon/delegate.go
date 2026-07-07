package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/josefdolezal/crew/internal/proto"
	"github.com/josefdolezal/crew/internal/runtime"
	"github.com/josefdolezal/crew/internal/store"
)

// handleReport is an agent declaring its task outcome; the report lands in
// its orchestrator's inbox and unblocks `crew wait`.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var req proto.ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	if req.Status != "done" && req.Status != "blocked" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("status must be done or blocked, got %q", req.Status))
		return
	}
	agent, err := s.store.Get(req.From)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, fmt.Errorf("reporter %q is not a registered agent", req.From))
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	msg := proto.Message{
		Sender:    agent.Name,
		Recipient: agent.Parent,
		Kind:      "report",
		Status:    req.Status,
		Body:      req.Message,
		CreatedAt: time.Now(),
	}
	id, err := s.deliver(msg)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	msg.ID = id
	log.Printf("report from %s: %s - %s", agent.Name, req.Status, req.Message)
	writeJSON(w, http.StatusCreated, msg)
}

// handleRoute delivers a message: to an agent's stdin if the recipient is
// a registered agent, otherwise to the recipient identity's inbox.
func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	var req proto.PostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	recipient := req.Recipient
	if recipient == "parent" {
		sender, err := s.store.Get(req.From)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("only agents have a parent; %q is not a registered agent", req.From))
			return
		}
		recipient = sender.Parent
	}
	if agent, err := s.store.Get(recipient); err == nil {
		if err := s.injectToAgent(agent, req.From, req.Body); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"delivery": "stdin", "recipient": agent.Name})
		return
	}
	id, err := s.deliver(proto.Message{
		Sender:    req.From,
		Recipient: recipient,
		Kind:      "message",
		Body:      req.Body,
		CreatedAt: time.Now(),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"delivery": "inbox", "recipient": recipient, "id": id})
}

// deliver stores an inbox message and, when the recipient has adopted a
// session (`crew adopt`), injects a one-line notification into it -
// push instead of waiting to be polled. The inbox row stays the source
// of truth either way.
func (s *Server) deliver(m proto.Message) (int64, error) {
	id, err := s.store.InsertMessage(m)
	if err != nil {
		return 0, err
	}
	m.ID = id
	s.pushToAdopted(m)
	return id, nil
}

func (s *Server) pushToAdopted(m proto.Message) {
	// A recipient that is itself an agent (nested orchestration) gets the
	// notification injected into its session directly - no adoption needed.
	if agent, err := s.store.Get(m.Recipient); err == nil {
		if err := s.backend.SendInput(agent.Session, notificationLine(m)); err != nil {
			log.Printf("push to agent %s: %v", m.Recipient, err)
		}
		return
	}
	session, err := s.store.Delivery(m.Recipient)
	if err != nil {
		return
	}
	if st, err := s.backend.State(session); err != nil || !st.Exists {
		log.Printf("adopt: session %s for %s is gone, deregistering", session, m.Recipient)
		_ = s.store.DeleteDelivery(m.Recipient)
		return
	}
	if err := s.backend.SendInput(session, notificationLine(m)); err != nil {
		log.Printf("adopt: deliver to %s (%s): %v", m.Recipient, session, err)
	}
}

// notificationLine renders an inbox message as a single injectable line.
// Long bodies are truncated - the full content is in the inbox.
func notificationLine(m proto.Message) string {
	body := strings.Join(strings.Fields(m.Body), " ")
	if len(body) > 300 {
		body = body[:300] + "… (crew inbox for full text)"
	}
	switch m.Kind {
	case "report":
		return fmt.Sprintf("[crew] %s reported %s: %s", m.Sender, m.Status, body)
	case "event":
		return "[crew] " + body
	default:
		return fmt.Sprintf("[crew] message from %s: %s", m.Sender, body)
	}
}

func (s *Server) handleAdopt(w http.ResponseWriter, r *http.Request) {
	var req proto.AdoptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	if req.Identity == "" || req.Session == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("identity and session are required"))
		return
	}
	if st, err := s.backend.State(req.Session); err != nil || !st.Exists {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("tmux session %q not found", req.Session))
		return
	}
	if err := s.store.SetDelivery(req.Identity, req.Session); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("adopted session %s for %s", req.Session, req.Identity)
	writeJSON(w, http.StatusOK, map[string]string{"identity": req.Identity, "session": req.Session})
}

func (s *Server) handleUnadopt(w http.ResponseWriter, r *http.Request) {
	identity := r.URL.Query().Get("identity")
	if identity == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("identity query parameter is required"))
		return
	}
	if err := s.store.DeleteDelivery(identity); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"identity": identity, "status": "unadopted"})
}

func (s *Server) injectToAgent(agent proto.Agent, from, text string) error {
	if rt, err := runtime.Lookup(agent.Runtime); err == nil && rt.SignInbound() && from != "" {
		text = "[" + from + "] " + text
	}
	return s.backend.SendInput(agent.Session, text)
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	recipient := r.URL.Query().Get("recipient")
	if recipient == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("recipient query parameter is required"))
		return
	}
	all := r.URL.Query().Get("all") == "true"
	drain := r.URL.Query().Get("drain") == "true"
	msgs, err := s.store.Inbox(recipient, !all, drain)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if msgs == nil {
		msgs = []proto.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// handleWait long-polls until the agent reports, its process ends, it
// looks idle without reporting, or the timeout passes.
func (s *Server) handleWait(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentOr404(w, r)
	if !ok {
		return
	}
	waitFor := r.URL.Query().Get("for")
	if waitFor == "" {
		waitFor = "done"
	}
	if waitFor != "done" && waitFor != "ready" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("for must be done or ready, got %q", waitFor))
		return
	}
	timeout := 600 * time.Second
	if v := r.URL.Query().Get("timeout"); v != "" {
		sec, err := strconv.Atoi(v)
		if err != nil || sec <= 0 {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid timeout %q", v))
			return
		}
		timeout = time.Duration(sec) * time.Second
	}
	rt, err := runtime.Lookup(agent.Runtime)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	start := time.Now()
	deadline := start.Add(timeout)
	finish := func(outcome proto.WaitOutcome, report *proto.Message, withScreen bool) {
		res := proto.WaitResult{
			Name:    agent.Name,
			Outcome: outcome,
			Report:  report,
			Elapsed: time.Since(start).Seconds(),
		}
		if withScreen {
			if screen, err := s.backend.Snapshot(agent.Session); err == nil {
				res.Screen = screenTail(screen, 25)
			}
		}
		writeJSON(w, http.StatusOK, res)
	}

	for {
		st, err := s.backend.State(agent.Session)
		if err != nil || !st.Exists {
			finish(proto.WaitExited, nil, false)
			return
		}
		if waitFor == "done" {
			if report, err := s.store.NextUnreadReport(agent.Name); err == nil {
				// Consume the report so the next wait blocks for the
				// next round instead of latching onto this one.
				_ = s.store.MarkRead(report.ID)
				outcome := proto.WaitDone
				if report.Status == "blocked" {
					outcome = proto.WaitBlocked
				}
				finish(outcome, &report, false)
				return
			}
		}
		if st.ProcessDead {
			finish(proto.WaitExited, nil, true)
			return
		}
		if waitFor == "ready" && s.isReady(agent.Name) {
			finish(proto.WaitReady, nil, false)
			return
		}
		// Attention: the agent is blocked on an interactive prompt
		// (permission dialog etc.); surface it immediately instead of
		// hanging until idle/timeout.
		if waitFor == "done" && s.isReady(agent.Name) {
			if screen, err := s.backend.Snapshot(agent.Session); err == nil && rt.Attention(screen) != "" {
				finish(proto.WaitAttention, nil, true)
				return
			}
		}
		// Idle fallback: the REPL became ready, then output went quiet
		// and the screen looks like a waiting prompt, yet no report came.
		if waitFor == "done" && s.isReady(agent.Name) {
			if activity, err := s.backend.ActivityAt(agent.Session); err == nil && time.Since(activity) >= idleAfter {
				if screen, err := s.backend.Snapshot(agent.Session); err == nil && rt.LooksIdle(screen) {
					finish(proto.WaitIdle, nil, true)
					return
				}
			}
		}
		if time.Now().After(deadline) {
			finish(proto.WaitTimeout, nil, true)
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(time.Second):
		}
	}
}
