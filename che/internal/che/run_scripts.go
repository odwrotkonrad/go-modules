package che

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
func (p *ProfileReady) resolveScripts(relativePaths []string) ([]string, error) {
	out := make([]string, len(relativePaths))
	for i, relativePath := range relativePaths {
		abs := filepath.Join(p.resolveRepoRoot(), relativePath)
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("run-scripts script not found: %s", relativePath)
		}
		out[i] = abs
	}
	return out, nil
}

// runScripts runs profile scripts in spec order. A failing script is logged
// and the rest still run; a per-script status report prints at the end, and
// the run returns an error if any script failed.
func (p *ProfileReady) runScripts(scripts []string) error {
	env := p.buildScriptsEnv()
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

// buildScriptsEnv materializes the child-process env from the captured profile
// env plus the Makefile $(ZSH) wrapper vars and che profile exports ([why] the
// one place captured env becomes a real []string: the child che spawns).
func (p *ProfileReady) buildScriptsEnv() []string {
	functions := filepath.Join(p.resolveRepoRoot(), "ci/zsh/functions")
	scripts := filepath.Join(p.resolveRepoRoot(), "ci/zsh/scripts")
	installs := filepath.Join(scripts, "installs")
	bootstrap := filepath.Join(scripts, "bootstrap")

	env := make([]string, 0, len(p.env)+1)
	for _, k := range slices.Sorted(maps.Keys(p.env)) {
		env = append(env, k+"="+p.env[k])
	}
	env = fsutil.PrependEnvVar(env, "FPATH", functions)
	env = fsutil.PrependEnvVar(env, "PATH", functions+":"+scripts+":"+installs+":"+bootstrap)
	env = append(env, "CONFIGS_PROFILE="+p.resolveProfileName())
	return env
}

// [<] 🤖🤖
