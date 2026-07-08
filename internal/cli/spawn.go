package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/josefdolezal/crew/internal/client"
	"github.com/josefdolezal/crew/internal/gitx"
	"github.com/josefdolezal/crew/internal/proto"
)

func spawnCmd() *cobra.Command {
	var (
		runtimeName string
		model       string
		cwd         string
		task        string
		taskFile    string
		yolo        bool
		trust       bool
		attach      bool
		worktree    bool
		noAdopt     bool
	)
	cmd := &cobra.Command{
		Use:   "spawn <name>",
		Short: "Spawn a new agent session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if task != "" && taskFile != "" {
				return fail(fmt.Errorf("--task and --task-file are mutually exclusive"))
			}
			if taskFile != "" {
				b, err := os.ReadFile(taskFile)
				if err != nil {
					return fail(fmt.Errorf("read task file: %w", err))
				}
				task = string(b)
			}
			if cwd == "" {
				var err error
				if cwd, err = canonicalCwd(); err != nil {
					return fail(err)
				}
			}
			abs, err := filepath.Abs(cwd)
			if err != nil {
				return fail(err)
			}
			if resolved, err := filepath.EvalSymlinks(abs); err == nil {
				abs = resolved
			}

			c, home, err := connect()
			if err != nil {
				return fail(err)
			}
			wtPath := ""
			if worktree {
				wtPath = filepath.Join(home, "worktrees", args[0])
				if err := os.MkdirAll(filepath.Dir(wtPath), 0o700); err != nil {
					return fail(err)
				}
				if err := gitx.AddWorktree(abs, wtPath, "crew/"+args[0]); err != nil {
					return fail(err)
				}
				abs = wtPath
			}
			agent, err := c.Spawn(proto.SpawnRequest{
				Name:     args[0],
				Runtime:  runtimeName,
				Model:    model,
				Cwd:      abs,
				Parent:   identity(),
				Task:     task,
				Yolo:     yolo,
				Trust:    trust,
				Worktree: wtPath,
			})
			if err != nil {
				if wtPath != "" {
					_, _ = gitx.RemoveWorktreeIfClean(wtPath)
				}
				return fail(err)
			}
			human := fmt.Sprintf("spawned %s (runtime=%s, session=%s)", agent.Name, agent.Runtime, agent.Session)
			if wtPath != "" {
				human += fmt.Sprintf("\nworktree: %s (branch crew/%s)", wtPath, agent.Name)
			}
			if pane, ok := autoAdopt(c, noAdopt); ok {
				human += fmt.Sprintf("\ninbox: pushed into this tmux pane (%s); disable: crew adopt --off", pane)
			}
			human += fmt.Sprintf("\nattach: crew attach %s", agent.Name)
			if err := okMsg(human, agent); err != nil {
				return err
			}
			if attach {
				return attachTo(c, agent.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&runtimeName, "runtime", "r", "claude", "runtime: claude | codex | pi | bash")
	cmd.Flags().StringVarP(&model, "model", "m", "", "model passed to the runtime (e.g. haiku)")
	cmd.Flags().StringVarP(&cwd, "cwd", "C", "", "working directory for the agent (default: current dir)")
	cmd.Flags().StringVarP(&task, "task", "t", "", "initial task prompt")
	cmd.Flags().StringVarP(&taskFile, "task-file", "f", "", "read initial task prompt from file")
	cmd.Flags().BoolVar(&yolo, "yolo", false, "skip runtime permission prompts (claude: --dangerously-skip-permissions)")
	cmd.Flags().BoolVar(&trust, "trust", true, "auto-confirm runtime startup dialogs (e.g. Claude's folder-trust prompt)")
	cmd.Flags().BoolVar(&worktree, "worktree", false, "run the agent in a fresh git worktree (branch crew/<name>) of the cwd repo; kill removes it if clean")
	cmd.Flags().BoolVar(&attach, "attach", false, "attach to the session after spawning")
	cmd.Flags().BoolVar(&noAdopt, "no-adopt", false, "don't register this tmux session for inbox push delivery")
	return cmd
}

// autoAdopt makes push delivery implicit: spawning from inside tmux
// registers the invoking pane as the inbox delivery target, so the
// orchestrator never has to remember `crew adopt`. The pane, not the
// session: injecting into a session lands on its active window, which is
// the wrong orchestrator when several run as windows of one session.
// Agents skip it - the daemon pushes to agent parents via the registry
// already. Best-effort.
func autoAdopt(c *client.Client, disabled bool) (string, bool) {
	pane := os.Getenv("TMUX_PANE")
	if disabled || pane == "" || os.Getenv("CREW_AGENT_NAME") != "" {
		return "", false
	}
	if err := c.Adopt(identity(), pane); err != nil {
		return "", false
	}
	return pane, true
}
