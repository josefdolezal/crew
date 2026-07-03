package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/josefdolezal/crew/internal/client"
	"github.com/josefdolezal/crew/internal/config"
	"github.com/josefdolezal/crew/internal/daemon"
)

func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the crew daemon (it autostarts on first use; these are for manual control)",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "Run the daemon in the foreground",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				home, err := config.Home()
				if err != nil {
					return err
				}
				srv, err := daemon.New(home)
				if err != nil {
					return err
				}
				return srv.Run()
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the running daemon (agent sessions keep running)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				home, err := config.Home()
				if err != nil {
					return fail(err)
				}
				if err := client.New(home).Shutdown(); err != nil {
					return fail(fmt.Errorf("daemon not reachable: %w", err))
				}
				return okMsg("daemon stopped (sessions unaffected)", map[string]string{"status": "stopped"})
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Check whether the daemon is running",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				home, err := config.Home()
				if err != nil {
					return fail(err)
				}
				if _, err := client.New(home).List(""); err != nil {
					return okMsg("daemon: not running ("+config.SocketPath(home)+")", map[string]string{"status": "down"})
				}
				return okMsg("daemon: running ("+config.SocketPath(home)+")", map[string]string{"status": "up"})
			},
		},
	)
	return cmd
}
