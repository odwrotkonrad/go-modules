package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// scriptResult pairs a script with its run status.
type scriptResult struct {
	script string
	status string // "ok" | "fail"
}

// RunScripts runs profile scripts in spec order. A failing script is logged
// and the rest still run; a per-script status report prints at the end, and
// the run returns an error if any script failed.
func (h Host) RunScripts(scripts []string) error {
	env := h.scriptsEnv()
	var results []scriptResult
	var failed []string
	for _, script := range scripts {
		h.fs.Log("run-scripts", script)
		if h.DryRun() {
			continue
		}
		c := exec.Command(script)
		c.Env = env
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			h.fs.Log("run-scripts(fail)", fmt.Sprintf("%s: %v", script, err))
			results = append(results, scriptResult{script, "fail"})
			failed = append(failed, script)
		} else {
			results = append(results, scriptResult{script, "ok"})
		}
	}

	for _, r := range results {
		h.fs.Log("run-scripts(report)", fmt.Sprintf("%s %s", r.status, r.script))
	}

	if len(failed) > 0 {
		return fmt.Errorf("scripts failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

// scriptsEnv mirrors Makefile $(ZSH) wrapper env, che profile exports.
func (h Host) scriptsEnv() []string {
	fns := filepath.Join(h.RepoRoot, "ci/zsh/functions")
	scripts := filepath.Join(h.RepoRoot, "ci/zsh/scripts")
	installs := filepath.Join(scripts, "installs")
	bootstrap := filepath.Join(scripts, "bootstrap")

	env := os.Environ()
	env = prepend(env, "FPATH", fns)
	env = prepend(env, "PATH", fns+":"+scripts+":"+installs+":"+bootstrap)
	env = append(env, "CONFIGS_PROFILE="+h.Profile)
	return env
}

// prepend sets key=value:<existing> in env copy.
func prepend(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, kv := range env {
		if rest, ok := strings.CutPrefix(kv, prefix); ok {
			out = append(out, prefix+value+":"+rest)
			found = true
		} else {
			out = append(out, kv)
		}
	}
	if !found {
		out = append(out, prefix+value)
	}
	return out
}

// [<] 🤖🤖
