package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/josefdolezal/crew/internal/config"
)

func logsCmd() *cobra.Command {
	var (
		tail int
		raw  bool
	)
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Print an agent's raw output log (ANSI-stripped; --raw to keep escapes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := config.Home()
			if err != nil {
				return fail(err)
			}
			b, err := os.ReadFile(config.AgentLog(home, args[0]))
			if err != nil {
				return fail(fmt.Errorf("no log for %q: %w", args[0], err))
			}
			out := string(b)
			if !raw {
				out = stripANSI(out)
			}
			lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
			if tail > 0 && len(lines) > tail {
				lines = lines[len(lines)-tail:]
			}
			text := strings.Join(lines, "\n")
			if jsonOut {
				return printJSON(map[string]any{"name": args[0], "lines": lines})
			}
			fmt.Println(text)
			return nil
		},
	}
	cmd.Flags().IntVarP(&tail, "tail", "n", 200, "print at most the last N lines (0 = all)")
	cmd.Flags().BoolVar(&raw, "raw", false, "keep ANSI escape sequences")
	return cmd
}

var (
	ansiCSI  = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	ansiOSC  = regexp.MustCompile(`\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
	ansiMisc = regexp.MustCompile(`\x1b[@-_]`)
)

// stripANSI removes escape sequences and resolves carriage-return
// overwrites (TUIs redraw lines with \r) so the log reads as plain text.
func stripANSI(s string) string {
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiCSI.ReplaceAllString(s, "")
	s = ansiMisc.ReplaceAllString(s, "")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		// CRLF ending first, then mid-line \r = TUI overwrite: keep the
		// final rendition.
		line = strings.TrimRight(line, "\r")
		if i := strings.LastIndex(line, "\r"); i >= 0 {
			line = line[i+1:]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
