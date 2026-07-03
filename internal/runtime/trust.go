package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josefdolezal/crew/internal/gitx"
)

// trustTarget is the path a runtime persists trust for: the git toplevel
// when dir is inside a repository (both claude and codex anchor trust
// there), otherwise dir itself.
func trustTarget(dir string) string {
	if top, err := gitx.Toplevel(dir); err == nil && top != "" {
		return top
	}
	return dir
}

// trustClaudeConfig marks projectPath as trusted in Claude Code's global
// config (~/.claude.json): projects[path].hasTrustDialogAccepted = true.
// The file also holds auth and is rewritten by live claude sessions, so:
// only write when the flag isn't already set, keep every other key
// untouched, and replace the file atomically. Returns whether it wrote.
func trustClaudeConfig(configPath, projectPath string) (bool, error) {
	cfg := map[string]any{}
	if b, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return false, fmt.Errorf("parse %s: %w", configPath, err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}

	projects, _ := cfg["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
		cfg["projects"] = projects
	}
	project, _ := projects[projectPath].(map[string]any)
	if project == nil {
		project = map[string]any{}
		projects[projectPath] = project
	}
	if trusted, _ := project["hasTrustDialogAccepted"].(bool); trusted {
		return false, nil
	}
	project["hasTrustDialogAccepted"] = true

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return false, err
	}
	return true, atomicWrite(configPath, out, 0o600)
}

// trustCodexConfig marks projectPath as trusted in Codex's config
// (~/.codex/config.toml) by appending a [projects."<path>"] section with
// trust_level = "trusted". If the section already exists - whatever its
// content - the file is left alone and the startup watcher remains the
// fallback. Returns whether it wrote.
func trustCodexConfig(configPath, projectPath string) (bool, error) {
	var text string
	if b, err := os.ReadFile(configPath); err == nil {
		text = string(b)
	} else if !os.IsNotExist(err) {
		return false, err
	}
	header := fmt.Sprintf("[projects.%q]", projectPath)
	if strings.Contains(text, header) {
		return false, nil
	}
	section := header + "\ntrust_level = \"trusted\"\n"
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if text != "" {
		text += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return false, err
	}
	return true, atomicWrite(configPath, []byte(text+section), 0o600)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode()
	}
	tmp := path + ".crew-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
