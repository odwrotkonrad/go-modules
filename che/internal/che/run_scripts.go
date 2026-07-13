package che

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

// scriptResult pairs a script with its run status ("ok" | "fail").
type scriptResult struct {
	script string
	status string
}

// resolveScripts maps spec-resolved script rels (globs already expanded by
// spec.Resolve) to absolute paths, IN SPEC ORDER, verifying each exists.
func (p *ProfileReady) resolveScripts(rels []string) ([]string, error) {
	out := make([]string, len(rels))
	for i, rel := range rels {
		abs := filepath.Join(p.repoRoot(), rel)
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("run-scripts script not found: %s", rel)
		}
		out[i] = abs
	}
	return out, nil
}

// runScripts runs profile scripts in spec order. A failing script is logged
// and the rest still run; a per-script status report prints at the end, and
// the run returns an error if any script failed.
func (p *ProfileReady) runScripts(scripts []string) error {
	env := p.scriptsEnv()
	var results []scriptResult
	var failed []string
	for _, script := range scripts {
		p.logMsg("run-scripts", script)
		if p.isDryRun() {
			continue
		}
		c := execx.Cmd{Argv: []string{script}, Env: env, Stdout: os.Stdout, Stderr: os.Stderr}
		status := "ok"
		if err := execx.Default.Exec(c); err != nil {
			p.logMsg("run-scripts(fail)", fmt.Sprintf("%s: %v", script, err))
			status = "fail"
			failed = append(failed, script)
		}
		results = append(results, scriptResult{script, status})
	}

	for _, r := range results {
		p.logMsg("run-scripts(report)", fmt.Sprintf("%s %s", r.status, r.script))
	}

	if len(failed) > 0 {
		return fmt.Errorf("scripts failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

// scriptsEnv mirrors Makefile $(ZSH) wrapper env, che profile exports.
func (p *ProfileReady) scriptsEnv() []string {
	fns := filepath.Join(p.repoRoot(), "ci/zsh/functions")
	scripts := filepath.Join(p.repoRoot(), "ci/zsh/scripts")
	installs := filepath.Join(scripts, "installs")
	bootstrap := filepath.Join(scripts, "bootstrap")

	env := os.Environ()
	env = fsutil.PrependEnvVar(env, "FPATH", fns)
	env = fsutil.PrependEnvVar(env, "PATH", fns+":"+scripts+":"+installs+":"+bootstrap)
	env = append(env, "CONFIGS_PROFILE="+p.profileName())
	return env
}

// [<] 🤖🤖
