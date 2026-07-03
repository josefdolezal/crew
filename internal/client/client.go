// Package client is the CLI-side HTTP client for the crew daemon,
// including transparent daemon autostart on first use.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/josefdolezal/crew/internal/config"
	"github.com/josefdolezal/crew/internal/proto"
)

type Client struct {
	home string
	http *http.Client
}

func New(home string) *Client {
	sock := config.SocketPath(home)
	return &Client{
		home: home,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", sock)
				},
			},
		},
	}
}

// Connect verifies the daemon is reachable, autostarting it if not.
func (c *Client) Connect() error {
	if c.healthy() {
		return nil
	}
	if err := c.autostart(); err != nil {
		return fmt.Errorf("daemon not running and autostart failed: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.healthy() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not become healthy within 5s (see %s)", config.DaemonLog(c.home))
}

func (c *Client) healthy() bool {
	resp, err := c.http.Get("http://crew/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// autostart re-execs this binary as a detached daemon process.
func (c *Client) autostart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(config.DaemonLog(c.home), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(exe, "daemon", "run")
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = detachedProcAttr()
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func (c *Client) Spawn(req proto.SpawnRequest) (proto.Agent, error) {
	var agent proto.Agent
	err := c.do("POST", "/agents", req, &agent)
	return agent, err
}

func (c *Client) List(parent string) ([]proto.Agent, error) {
	path := "/agents"
	if parent != "" {
		path += "?parent=" + url.QueryEscape(parent)
	}
	var agents []proto.Agent
	err := c.do("GET", path, nil, &agents)
	return agents, err
}

func (c *Client) Get(name string) (proto.Agent, error) {
	var agent proto.Agent
	err := c.do("GET", "/agents/"+name, nil, &agent)
	return agent, err
}

func (c *Client) Kill(name string) (map[string]string, error) {
	var res map[string]string
	err := c.do("DELETE", "/agents/"+name, nil, &res)
	return res, err
}

func (c *Client) Snapshot(name string) (proto.Snapshot, error) {
	var snap proto.Snapshot
	err := c.do("GET", "/agents/"+name+"/snapshot", nil, &snap)
	return snap, err
}

func (c *Client) Send(name string, req proto.SendRequest) error {
	return c.do("POST", "/agents/"+name+"/input", req, nil)
}

func (c *Client) Shutdown() error {
	return c.do("POST", "/shutdown", nil, nil)
}

func (c *Client) Report(req proto.ReportRequest) (proto.Message, error) {
	var msg proto.Message
	err := c.do("POST", "/report", req, &msg)
	return msg, err
}

// Route delivers a message to an agent's stdin or an identity's inbox;
// the daemon decides which.
func (c *Client) Route(req proto.PostRequest) (map[string]any, error) {
	var res map[string]any
	err := c.do("POST", "/route", req, &res)
	return res, err
}

func (c *Client) Inbox(recipient string, all, drain bool) ([]proto.Message, error) {
	q := url.Values{"recipient": {recipient}}
	if all {
		q.Set("all", "true")
	}
	if drain {
		q.Set("drain", "true")
	}
	var msgs []proto.Message
	err := c.do("GET", "/inbox?"+q.Encode(), nil, &msgs)
	return msgs, err
}

func (c *Client) Adopt(identity, session string) error {
	return c.do("POST", "/adopt", proto.AdoptRequest{Identity: identity, Session: session}, nil)
}

func (c *Client) Unadopt(identity string) error {
	return c.do("DELETE", "/adopt?identity="+url.QueryEscape(identity), nil, nil)
}

// Wait long-polls the daemon until the agent reports, exits, idles, or
// the timeout passes. It bypasses the client's default 30s HTTP timeout.
func (c *Client) Wait(name, waitFor string, timeout time.Duration) (proto.WaitResult, error) {
	var res proto.WaitResult
	q := url.Values{"for": {waitFor}, "timeout": {fmt.Sprintf("%d", int(timeout.Seconds()))}}
	req, err := http.NewRequest("GET", "http://crew/agents/"+name+"/wait?"+q.Encode(), nil)
	if err != nil {
		return res, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+15*time.Second)
	defer cancel()
	longClient := &http.Client{Transport: c.http.Transport}
	resp, err := longClient.Do(req.WithContext(ctx))
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e proto.ErrorResponse
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error != "" {
			return res, fmt.Errorf("%s", e.Error)
		}
		return res, fmt.Errorf("daemon returned %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(&res)
	return res, err
}

func (c *Client) do(method, path string, body, out any) error {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://crew"+path, buf)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e proto.ErrorResponse
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("daemon returned %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
