// Package cli defines the crew command tree. Output is human-readable by
// default; every command supports --json for LLM/scripted callers.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/josefdolezal/crew/internal/client"
	"github.com/josefdolezal/crew/internal/config"
	"github.com/josefdolezal/crew/internal/version"
)

var jsonOut bool

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "crew",
		Short:         "Spawn and steer interactive coding-agent sessions (claude, codex, ...)",
		Long:          "crew lets an orchestrator (human or LLM) delegate work to interactive agent\nsessions running in tmux. Sessions survive daemon restarts and are always\nattachable with `crew attach <name>`.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Full(),
	}
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "machine-readable JSON output")
	root.AddCommand(
		spawnCmd(), listCmd(), killCmd(), attachCmd(), peekCmd(), sendCmd(),
		waitCmd(), reportCmd(), inboxCmd(), adoptCmd(), logsCmd(), daemonCmd(),
	)
	return root
}

// connect resolves crew home and returns a connected client, autostarting
// the daemon when needed.
func connect() (*client.Client, string, error) {
	home, err := config.Home()
	if err != nil {
		return nil, "", err
	}
	c := client.New(home)
	if err := c.Connect(); err != nil {
		return nil, "", err
	}
	return c, home, nil
}

// identity returns the caller's actor identity: agents identify via
// CREW_AGENT_NAME; orchestrators via CREW_IDENTITY when pinned explicitly
// (recommended for long-lived sessions that may change cwd), otherwise
// scoped to their cwd so two sessions in different worktrees are distinct
// orchestrators.
func identity() string {
	if name := os.Getenv("CREW_AGENT_NAME"); name != "" {
		return name
	}
	if id := os.Getenv("CREW_IDENTITY"); id != "" {
		return id
	}
	cwd, err := canonicalCwd()
	if err != nil {
		cwd = "unknown"
	}
	return "orchestrator@" + cwd
}

// canonicalCwd asks the kernel for the working directory instead of
// trusting $PWD: shells can report a case/symlink variant of the same
// directory, which would fork one orchestrator into several identities.
func canonicalCwd() (string, error) {
	if wd, err := syscall.Getwd(); err == nil {
		return wd, nil
	}
	return os.Getwd()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func fail(err error) error {
	if jsonOut {
		_ = printJSON(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
	return err
}

func okMsg(human string, v any) error {
	if jsonOut {
		return printJSON(v)
	}
	fmt.Println(human)
	return nil
}
