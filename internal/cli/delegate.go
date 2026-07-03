package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/josefdolezal/crew/internal/proto"
)

func waitCmd() *cobra.Command {
	var (
		waitFor string
		timeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "wait <name>...",
		Short: "Block until agents report done/blocked (or turn ready/idle/exited)",
		Long:  "Blocks until each agent reports via `crew report`, its process ends, or it looks\nidle without reporting (fallback). Exit code 0 only if every agent reported done.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			results := make([]proto.WaitResult, 0, len(args))
			allDone := true
			for _, name := range args {
				res, err := c.Wait(name, waitFor, timeout)
				if err != nil {
					return fail(fmt.Errorf("wait %s: %w", name, err))
				}
				results = append(results, res)
				ok := res.Outcome == proto.WaitDone || (waitFor == "ready" && res.Outcome == proto.WaitReady)
				if !ok {
					allDone = false
				}
				if !jsonOut {
					printWaitResult(res)
				}
			}
			if jsonOut {
				if err := printJSON(results); err != nil {
					return err
				}
			}
			if !allDone {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&waitFor, "for", "done", "what to wait for: done | ready")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "give up after this long (e.g. 90s, 15m)")
	return cmd
}

func printWaitResult(res proto.WaitResult) {
	fmt.Printf("%s: %s (%.0fs)\n", res.Name, res.Outcome, res.Elapsed)
	if res.Outcome == proto.WaitAttention {
		fmt.Printf("  agent is blocked on an interactive prompt; answer with: crew send %s '1' (or --key Enter), then wait again\n", res.Name)
	}
	if res.Report != nil && res.Report.Body != "" {
		fmt.Printf("  report: %s\n", res.Report.Body)
	}
	if res.Screen != "" {
		fmt.Printf("  screen tail:\n%s\n", indent(res.Screen, "  | "))
	}
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return prefix + strings.Join(lines, "\n"+prefix)
}

func reportCmd() *cobra.Command {
	var (
		status  string
		message string
		as      string
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report your task outcome to your orchestrator (run by agents)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			from := as
			if from == "" {
				from = os.Getenv("CREW_AGENT_NAME")
			}
			if from == "" {
				return fail(fmt.Errorf("report is for agents: CREW_AGENT_NAME is not set (or pass --as <name>)"))
			}
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			msg, err := c.Report(proto.ReportRequest{From: from, Status: status, Message: message})
			if err != nil {
				return fail(err)
			}
			return okMsg(fmt.Sprintf("reported %s to %s", status, msg.Recipient), msg)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "outcome: done | blocked (required)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "one-line summary of the outcome (required)")
	_ = cmd.MarkFlagRequired("status")
	_ = cmd.MarkFlagRequired("message")
	cmd.Flags().StringVar(&as, "as", "", "override reporter identity (testing)")
	return cmd
}

func adoptCmd() *cobra.Command {
	var off bool
	cmd := &cobra.Command{
		Use:   "adopt",
		Short: "Deliver your inbox into this tmux session as it arrives (calypso-style push)",
		Long:  "Run inside a tmux session to register it as your identity's delivery target:\nreports, agent messages, and events are injected as [crew] lines the moment\nthey arrive - no polling, no blocked wait. The inbox remains the source of\ntruth; long bodies are truncated in the injected line.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			id := identity()
			if off {
				if err := c.Unadopt(id); err != nil {
					return fail(err)
				}
				return okMsg("stopped delivering to this session", map[string]string{"identity": id, "status": "unadopted"})
			}
			if os.Getenv("TMUX") == "" {
				return fail(fmt.Errorf("adopt requires running inside tmux (fallback: poll with crew wait / crew inbox)"))
			}
			out, err := exec.Command("tmux", "display-message", "-p", "#S").Output()
			if err != nil {
				return fail(fmt.Errorf("detect tmux session: %w", err))
			}
			session := strings.TrimSpace(string(out))
			if err := c.Adopt(id, session); err != nil {
				return fail(err)
			}
			return okMsg(
				fmt.Sprintf("inbox for %s now delivers into tmux session %q (undo: crew adopt --off)", id, session),
				map[string]string{"identity": id, "session": session},
			)
		},
	}
	cmd.Flags().BoolVar(&off, "off", false, "deregister this identity's delivery session")
	return cmd
}

func inboxCmd() *cobra.Command {
	var (
		all   bool
		drain bool
	)
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Read messages sent to you (reports, agent messages, exit events)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			msgs, err := c.Inbox(identity(), all, drain)
			if err != nil {
				return fail(err)
			}
			if jsonOut {
				return printJSON(msgs)
			}
			if len(msgs) == 0 {
				fmt.Println("inbox empty (unread; --all includes read)")
				return nil
			}
			for _, m := range msgs {
				tag := m.Kind
				if m.Status != "" {
					tag += ":" + m.Status
				}
				fmt.Printf("[%s] %s  %s: %s\n", m.CreatedAt.Format("15:04:05"), tag, m.Sender, m.Body)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include already-read messages")
	cmd.Flags().BoolVar(&drain, "drain", false, "mark returned messages as read")
	return cmd
}
