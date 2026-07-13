package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

// RunScripts runs profile scripts in spec order. A failing script is logged
// and the rest still run; a per-script status report prints at the end, and
// the run returns an error if any script failed.
func (h Host) RunScripts(scripts []string) error {
	env := h.scriptsEnv()
	var results []scriptResult
	var failed []string
	for _, script := range scripts {
		h.log("run-scripts", script)
		if h.IsDryRun() {
			continue
		}
		c := execx.Cmd{Argv: []string{script}, Env: env, Stdout: os.Stdout, Stderr: os.Stderr}
		status := "ok"
		if err := execx.Default.Exec(c); err != nil {
			h.log("run-scripts(fail)", fmt.Sprintf("%s: %v", script, err))
			status = "fail"
			failed = append(failed, script)
		}
		results = append(results, scriptResult{script, status})
	}

	for _, r := range results {
		h.log("run-scripts(report)", fmt.Sprintf("%s %s", r.status, r.script))
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
	env = prependEnvVar(env, "FPATH", fns)
	env = prependEnvVar(env, "PATH", fns+":"+scripts+":"+installs+":"+bootstrap)
	env = append(env, "CONFIGS_PROFILE="+h.Profile)
	return env
}

// prependEnvVar sets key=value:<existing> in env copy.
func prependEnvVar(env []string, key, value string) []string {
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
