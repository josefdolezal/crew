package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/josefdolezal/crew/internal/client"
	"github.com/josefdolezal/crew/internal/proto"
)

func killCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <name>",
		Short: "Kill an agent session and remove it from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			res, err := c.Kill(args[0])
			if err != nil {
				return fail(err)
			}
			human := "killed " + args[0]
			if note := res["worktree_note"]; note != "" {
				human += "\n" + note
			} else if res["worktree"] == "removed" {
				human += "\nworktree removed (was clean)"
			}
			return okMsg(human, res)
		},
	}
}

func attachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "Attach your terminal to an agent's session (tmux detach: ctrl-b d)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			return attachTo(c, args[0])
		},
	}
}

// attachTo replaces the CLI process with the backend's attach command.
func attachTo(c *client.Client, name string) error {
	agent, err := c.Get(name)
	if err != nil {
		return fail(err)
	}
	if agent.Status == proto.StatusGone {
		return fail(fmt.Errorf("agent %q session is gone", name))
	}
	// M1 supports the tmux backend only, so attach is a plain tmux exec.
	// Agents live as "crew:<name>" windows in one session; select the
	// window first so the client lands on it. Pre-window agents stored a
	// plain session name.
	tmuxArgs := []string{"tmux", "attach-session", "-t", "=" + agent.Session}
	if sess, window, ok := strings.Cut(agent.Session, ":"); ok {
		tmuxArgs = []string{"tmux", "select-window", "-t", "=" + sess + ":=" + window, ";", "attach-session", "-t", "=" + sess}
	}
	path, err := exec.LookPath(tmuxArgs[0])
	if err != nil {
		return fail(err)
	}
	return syscall.Exec(path, tmuxArgs, os.Environ())
}

func peekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "peek <name>",
		Short: "Print the agent's current rendered screen",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			snap, err := c.Snapshot(args[0])
			if err != nil {
				return fail(err)
			}
			if jsonOut {
				return printJSON(snap)
			}
			fmt.Print(snap.Screen)
			return nil
		},
	}
}

func sendCmd() *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "send <recipient> [text]",
		Short: "Send a message: to an agent's stdin, or to an orchestrator's inbox",
		Long:  "Agent recipients get the text injected into their session (signed with your\nidentity). `crew send parent <text>` reaches your orchestrator's inbox; any\nother non-agent recipient is treated as an inbox identity too.\n\n--key sends a single named key (Enter, Escape, Up, Down, ...) to an agent's\nsession instead of text - for answering interactive dialogs.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			if key != "" {
				if len(args) > 1 {
					return fail(fmt.Errorf("--key and a text argument are mutually exclusive"))
				}
				if err := c.Send(args[0], proto.SendRequest{Key: key}); err != nil {
					return fail(err)
				}
				return okMsg(fmt.Sprintf("sent key %s to %s", key, args[0]), map[string]string{"delivery": "key", "recipient": args[0], "key": key})
			}
			if len(args) < 2 {
				return fail(fmt.Errorf("text argument is required unless --key is used"))
			}
			res, err := c.Route(proto.PostRequest{From: identity(), Recipient: args[0], Body: args[1]})
			if err != nil {
				return fail(err)
			}
			return okMsg(fmt.Sprintf("delivered to %v via %v", res["recipient"], res["delivery"]), res)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "send a named key (Enter, Escape, ...) instead of text")
	return cmd
}
