package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List agents (yours by default, --all for everyone's)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := connect()
			if err != nil {
				return fail(err)
			}
			parent := identity()
			if all {
				parent = ""
			}
			agents, err := c.List(parent)
			if err != nil {
				return fail(err)
			}
			if jsonOut {
				return printJSON(agents)
			}
			if len(agents) == 0 {
				fmt.Println("no agents (try --all, or spawn one: crew spawn <name> -t '<task>')")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tRUNTIME\tMODEL\tSTATUS\tAGE\tCWD\tPARENT")
			for _, a := range agents {
				age := time.Since(a.CreatedAt).Round(time.Second)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", a.Name, a.Runtime, orDash(a.Model), a.Status, age, a.Cwd, a.Parent)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include agents spawned by other orchestrators")
	return cmd
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
