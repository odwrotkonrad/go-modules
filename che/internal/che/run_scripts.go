package che

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

type scriptResult struct {
	script string
	status string
}

var ErrErrexit = errors.New("errexit")

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

func (p *ProfileReady) runScripts(scripts []string) error {
	env := p.buildScriptsEnv()
	var results []scriptResult
	var failed []string
	errexitHit := false
	for _, script := range scripts {
		if p.isDryRun() {
			p.emitDryRun("run-scripts", "run", script)
			continue
		}
		p.emit(log.Levels.Debug, "run-scripts", "will-run", script)
		sctx, span := p.tel.Span(p.opContext(), "run-script", attribute.String("script", script))
		c := execx.Cmd{Ctx: sctx, Argv: []string{script}, Env: env, Stdout: os.Stdout, Stderr: os.Stderr}
		status := "ok"
		if err := execx.Default.Exec(c); err != nil {
			span.RecordError(err)
			p.emit(log.Levels.Error, "run-scripts", "fail", fmt.Sprintf("%s: %v", script, err))
			status = "fail"
			failed = append(failed, script)
		}
		span.End()
		p.tel.CountUnit(sctx, "script", status, p.command)
		results = append(results, scriptResult{script, status})
		if status == "fail" && p.opts.Errexit {
			errexitHit = true
			break
		}
	}

	for _, r := range results {
		action := "ran"
		if r.status == "fail" {
			action = "failed"
		}
		p.emit(log.Levels.Info, "run-scripts", action, r.script)
	}

	if errexitHit {
		return fmt.Errorf("scripts failed: %s: %w", strings.Join(failed, ", "), ErrErrexit)
	}
	if len(failed) > 0 {
		return fmt.Errorf("scripts failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

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
