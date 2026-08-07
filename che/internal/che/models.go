package che

// [>] 🤖🤖

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/telemetry"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

type Seams struct {
	FS      fsutil.FileSystemWriter
	Reader  fsutil.FileSystemReader
	Fetcher RemoteFetcher
	Ledger  *database.DB
}

var NewSeams = func(home string) Seams {
	db, err := database.Open(filepath.Join(fsutil.ResolveStateHome(home), "ops.db"))
	if err != nil {
		log.EmitTrace("ledger", "error", "open failed: "+err.Error())
		db = nil
	}
	return Seams{
		FS:      fsutil.FS{Home: home},
		Reader:  fsutil.OSReader{},
		Fetcher: gitFetcher{fetch: render.NewRemoteFetcher()},
		Ledger:  db,
	}
}

type RemoteFetcher interface {
	Fetch(ref string) (string, error)
}

type gitFetcher struct{ fetch func(string) (string, error) }

func (g gitFetcher) Fetch(ref string) (string, error) { return g.fetch(ref) }

func (p *ProfileReady) fetchRemote(ref string) (string, error) {
	_, span := p.tel.Span(p.opContext(), "fetch-remote", attribute.String("ref", ref))
	defer span.End()
	content, err := p.Fetcher.Fetch(ref)
	if err != nil {
		span.RecordError(err)
	}
	return content, err
}

// [<] 🤖🤖

// [>] 🤖🤖 package-level funcs

func PrepareApplicationOptions(ctx Context, opts options.Options) (Context, options.Options, error) {
	resolvedOptions := opts
	resolvedOptions.CheWorkingDirectory = cmp.Or(resolvedOptions.CheWorkingDirectory, ctx.Env["CHE_WORKING_DIRECTORY"])
	if resolvedOptions.CheWorkingDirectory != "" {
		next, err := changeDir(ctx.Cwd, resolvedOptions.CheWorkingDirectory)
		if err != nil {
			return ctx, resolvedOptions, fmt.Errorf("-C: %w", err)
		}
		ctx.Cwd = next
	}
	repoRoot, err := findRepoRoot(ctx)
	if err != nil {
		return ctx, resolvedOptions, err
	}
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return ctx, resolvedOptions, err
	}
	if err := resolvedOptions.Resolve(ctx.lookupEnv(), readUserLayer(fsutil.ResolveUserConfigPath(home)), readSpecLayer(filepath.Join(repoRoot, "che.yml"))); err != nil {
		return ctx, resolvedOptions, err
	}
	level, err := log.ParseLevel(resolvedOptions.LogLevel)
	if err != nil {
		return ctx, resolvedOptions, err
	}
	log.SetLevel(level)
	resolvedOptions.FillDefaultSetting("cheWorkingDirectory", ctx.Cwd)
	return ctx, resolvedOptions, nil
}

func changeDir(cwd, dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("chdir %s: not a directory", dir)
	}
	return dir, nil
}

func readUserLayer(path string) options.Layer {
	b, err := os.ReadFile(path)
	if err != nil {
		return options.Layer{}
	}
	var o spec.Options
	if err := yaml.Unmarshal(b, &o); err != nil {
		return options.Layer{}
	}
	return o
}

func readSpecLayer(path string) options.Layer {
	b, err := os.ReadFile(path)
	if err != nil {
		return options.Layer{}
	}
	var d struct {
		Options spec.Options `yaml:"options"`
	}
	if err := yaml.Unmarshal(b, &d); err != nil {
		return options.Layer{}
	}
	return d.Options
}

func PrepareSpecs(ctx Context, opts options.Options, src spec.SpecSourceRecipe) (*SpecReady, error) {
	_, span := ctx.Tel.Span(ctx.runContext(), "prepare-specs")
	defer span.End()
	repoRoot, err := findRepoRoot(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return nil, err
	}
	p := &specsPrep{
		ctx:       ctx,
		opts:      opts,
		home:      home,
		gitRoot:   repoRoot,
		tel:       ctx.Tel,
		seenSpecs: map[string]bool{},
		seenRefs:  map[string]bool{},
	}
	root, err := p.prepare(src, repoRoot, nil, true)
	if root != nil {
		root.Rejected = p.rejected
	}
	return root, err
}

type specsPrep struct {
	ctx       Context
	opts      options.Options
	home      string
	gitRoot   string
	tel       *telemetry.Telemetry
	seenSpecs map[string]bool
	seenRefs  map[string]bool
	rejected  []spec.Rejection
	specDone  *database.SpecDone
}

func (p *specsPrep) startSpec(db *database.DB, uri string) *database.SpecDone {
	if p.specDone != nil {
		return p.specDone
	}
	s, err := db.StartSpec(p.ctx.RunID, uri, p.ctx.Command)
	if err != nil {
		log.EmitTrace("ledger", "error", "start spec: "+err.Error())
		return nil
	}
	p.specDone = s
	return s
}

func (p *specsPrep) prepare(src spec.SpecSourceRecipe, anchor string, forced *spec.ProfileSourceRecipe, root bool) (*SpecReady, error) {
	recipe := &SpecRecipe{Source: src}
	if err := recipe.PrepareSpec(anchor, p.home); err != nil {
		return nil, err
	}
	if forced != nil {
		// [why] keyed on the resolved dir, not the raw URI: the same spec is
		key := recipe.sourceReady.DirectoryPath + "::" + forced.ProfileName
		if p.seenRefs[key] {
			log.EmitSkip(log.Levels.Trace, "init-remote-sources", "prepare", forced.ProfileName, "duplicate ref "+forced.String())
			return nil, nil
		}
		p.seenRefs[key] = true
	} else if !root && p.seenSpecs[recipe.sourceReady.DirectoryPath] {
		log.EmitSkip(log.Levels.Trace, "init-remote-sources", "prepare", recipe.sourceReady.DirectoryPath, "duplicate spec")
		return nil, nil
	}
	p.seenSpecs[recipe.sourceReady.DirectoryPath] = true
	if err := recipe.PrepareProfileRecipes(p.opts, root); err != nil {
		return nil, err
	}
	return recipe.PrepareProfiles(p, forced, root)
}

func findRepoRoot(ctx Context) (string, error) {
	root, err := fsutil.ResolveRepoRoot(ctx.Cwd)
	if err != nil {
		if fsutil.IsNoRepo(err) {
			return findSpecRoot(ctx.Cwd)
		}
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "che.yml")); err != nil {
		return "", fmt.Errorf("che.yml not found at repo root %s", root)
	}
	return root, nil
}

func findSpecRoot(dir string) (string, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "che.yml")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("che.yml not found walking up from %s (no git repo)", dir)
		}
		d = parent
	}
}

func resolveInvokingHome(ctx Context) (string, error) {
	if ctx.Euid == 0 {
		if name := ctx.Env["SUDO_USER"]; name != "" {
			h, err := fsutil.ResolveUserHome(name)
			if err != nil {
				return "", fmt.Errorf("lookup SUDO_USER %q: %w", name, err)
			}
			return h, nil
		}
	}
	home := ctx.Env["HOME"]
	if home == "" {
		return "", fmt.Errorf("HOME must be set")
	}
	return home, nil
}

func overlayEnv(base map[string]string, overlay map[string]string) map[string]string {
	if len(overlay) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(overlay))
	maps.Copy(out, base)
	maps.Copy(out, overlay)
	return out
}

// [<] 🤖🤖

// [>] 🤖🤖 SpecRecipe

type SpecRecipe struct {
	Source         spec.SpecSourceRecipe
	Include        []spec.SpecSourceRecipe
	Options        spec.Options
	Env            map[string]string
	ProfileRecipes []spec.ProfileRecipe
	sourceReady    spec.SpecSourceReady
}

func (r *SpecRecipe) PrepareSpec(anchor, home string) error {
	ready, err := r.Source.PrepareSource(anchor, home)
	if err != nil {
		return err
	}
	r.sourceReady = ready
	return nil
}

func (r *SpecRecipe) PrepareProfileRecipes(opts options.Options, root bool) error {
	if err := r.validateSchema(opts.ValidateSpecCLI); err != nil {
		return err
	}
	doc, err := spec.Load(r.sourceReady.DefinitionURI)
	if err != nil {
		return err
	}
	r.Options, r.Env, r.Include = doc.Options, doc.Env, doc.Include
	// [why] che-level (flag/env/user-config) seeds the spec default, cascading
	if root && r.Options.ProfileWorkingDirectory == "" {
		r.Options.ProfileWorkingDirectory = opts.ProfileWorkingDirectory
	}
	for i := range doc.ProfileRecipes {
		rec := &doc.ProfileRecipes[i]
		rec.Options = rec.Options.Over(r.Options)
		rec.Source.URI = r.Source.URI
		rec.Source.DirectoryPath = r.sourceReady.DirectoryPath
	}
	r.ProfileRecipes = doc.ProfileRecipes
	return nil
}

func resolveWorkingDir(env map[string]string, anchor, directory string) (string, error) {
	if directory == "" {
		directory = spec.DefaultWorkingDir
	}
	expanded := os.Expand(directory, func(k string) string { return env[k] })
	dir := fsutil.ExpandHome(expanded, env["HOME"])
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(anchor, dir)
	}
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("options.profileWorkingDirectory not found: %s (from %q)", dir, directory)
	}
	return dir, nil
}

func (r *SpecRecipe) validateSchema(cli options.ValidateSpecMode) error {
	b, err := os.ReadFile(r.sourceReady.DefinitionURI)
	if err != nil {
		return fmt.Errorf("spec not found: %s", r.sourceReady.DefinitionURI)
	}
	finds := spec.ValidateSchema(b)
	if len(finds) == 0 {
		return nil
	}
	// [why] flag/env wins over this spec's own options.validateSpec, then warn.
	mode := options.ValidateSpecMode(cmp.Or(string(cli), peekSpecValidateMode(b), string(options.ValidateSpec.Warn)))
	if mode == options.ValidateSpec.Error {
		return fmt.Errorf("schema violations in %s:\n%s", r.sourceReady.DefinitionURI, strings.Join(finds, "\n"))
	}
	for _, f := range finds {
		log.EmitWarn("validate-spec", "warning", f)
	}
	return nil
}

func peekSpecValidateMode(b []byte) string {
	var d struct {
		Options spec.Options `yaml:"options"`
	}
	if err := yaml.Unmarshal(b, &d); err != nil {
		return ""
	}
	return d.Options.ValidateSpec
}

func (r *SpecRecipe) candidateSummary() (all, autoDiscoverable string) {
	var profs, auto []string
	for _, rec := range r.ProfileRecipes {
		name := rec.Source.GetProfileName()
		if incs := rec.IncludedProfileRefs(); len(incs) > 0 {
			name += "[" + strings.Join(incs, ",") + "]"
		}
		profs = append(profs, name)
		if rec.Options.AutoDiscover != nil && *rec.Options.AutoDiscover {
			auto = append(auto, rec.Source.GetProfileName())
		}
	}
	return "[" + strings.Join(profs, ",") + "]", "[" + strings.Join(auto, ",") + "]"
}

func (r *SpecRecipe) PrepareProfiles(p *specsPrep, forced *spec.ProfileSourceRecipe, root bool) (*SpecReady, error) {
	ready := &SpecReady{Source: r.sourceReady, Options: r.Options, Env: r.Env, recipes: r.ProfileRecipes, tel: p.tel}
	if root { // [why] candidates log once, for the invoked spec only
		all, auto := r.candidateSummary()
		log.EmitTrace("discover-profiles", "listed-candidates", all)
		log.EmitTrace("discover-profiles", "listed-candidates", "auto-discoverable "+auto)
	}
	// [why] the spec-level env: overlay gates this spec's own runIf.
	eval := p.evalWith(r.Env)
	pass, _, err := spec.AllPass("spec "+r.sourceReady.DefinitionURI, r.Options.RunIf, p.opts.SkipRunIf, eval)
	if err != nil {
		return nil, err
	}
	if !pass {
		log.EmitSkip(log.Levels.Debug, "discover-profiles", "load-spec", r.sourceReady.DefinitionURI, "runIf failed")
		return ready, nil
	}
	lookup, err := r.composeIncludes(p, ready)
	if err != nil {
		return nil, err
	}
	names, err := r.selectEligibleNames(p, forced, root)
	if err != nil || len(names) == 0 {
		return ready, err
	}
	if err := r.assembleProfiles(p, ready, lookup, names, forced); err != nil {
		return nil, err
	}
	return ready, nil
}

func (p *specsPrep) evalWith(overlay map[string]string) func(string) (bool, error) {
	env := overlayEnv(p.ctx.Env, overlay)
	return spec.NewEvaluator(func(k string) string { return env[k] }).EvalRunIf
}

func (r *SpecRecipe) composeIncludes(p *specsPrep, ready *SpecReady) ([]spec.ProfileRecipe, error) {
	lookup := slices.Clone(r.ProfileRecipes)
	for _, inc := range r.Include {
		child, err := p.prepare(inc, r.sourceReady.DirectoryPath, nil, false)
		if err != nil {
			return nil, fmt.Errorf("include.sources %q: %w", inc.URI, err)
		}
		if child == nil {
			continue
		}
		ready.Include = append(ready.Include, child)
		for _, rec := range child.recipes {
			name := rec.Source.GetProfileName()
			if _, err := spec.FindRecipe(lookup, name); err == nil {
				return nil, fmt.Errorf("include.sources %q: profile %q collides with an already-composed profile of the same name", inc.URI, name)
			}
			lookup = append(lookup, rec)
		}
	}
	return lookup, nil
}

func (r *SpecRecipe) selectEligibleNames(p *specsPrep, forced *spec.ProfileSourceRecipe, root bool) ([]string, error) {
	if forced != nil {
		rec, err := spec.FindRecipe(r.ProfileRecipes, forced.ProfileName)
		if err != nil {
			return nil, fmt.Errorf("ref %s: %w", forced, err)
		}
		// [why] the ref entry's env overlays the launch env for its runIf gate.
		pass, failed, err := spec.AllPass(forced.ProfileName, rec.Options.OverRef(forced.Options).RunIf, p.opts.SkipRunIf, p.evalWith(forced.Env))
		if err != nil {
			return nil, fmt.Errorf("ref %s: %w", forced, err)
		}
		if !pass {
			p.rejected = append(p.rejected, spec.Rejection{Ref: forced.DisplayRef(), Cond: failed})
			return nil, nil
		}
		return []string{forced.ProfileName}, nil
	}
	var forcedProfiles []string
	if root {
		forcedProfiles = p.opts.Profiles
	}
	// [why] autoDiscover=false disables the discovery mechanism itself: only
	if len(forcedProfiles) == 0 && !p.opts.AutoDiscover {
		if root {
			return nil, fmt.Errorf("auto-discovery is disabled (options.autoDiscover=false): pass --profiles")
		}
		log.EmitSkip(log.Levels.Debug, "discover-profiles", "load-spec", r.sourceReady.DefinitionURI, "autoDiscover disabled")
		return nil, nil
	}
	names, rejected, err := spec.EligibleRecipes(r.ProfileRecipes, forcedProfiles, p.opts.SkipRunIf, p.evalWith(r.Env))
	p.rejected = append(p.rejected, rejected...)
	if err != nil {
		if !root && errors.Is(err, spec.ErrNoneEligible) {
			log.EmitSkip(log.Levels.Debug, "discover-profiles", "load-spec", r.sourceReady.DefinitionURI, "no eligible profile")
			return nil, nil
		}
		return nil, err
	}
	return names, nil
}

func (r *SpecRecipe) assembleProfiles(p *specsPrep, ready *SpecReady, lookup []spec.ProfileRecipe, names []string, forced *spec.ProfileSourceRecipe) error {
	for _, name := range names {
		rec, err := spec.FindRecipe(lookup, name)
		if err != nil {
			return err
		}
		env := r.Env
		var refCtx map[string]string
		if forced != nil {
			rec.Options = rec.Options.OverRef(forced.Options)
			env = fsutil.MergeMap(r.Env, forced.Env)
			refCtx = forced.Ctx
		}
		pr, refs, err := r.makeProfileReady(p, rec, lookup, env)
		if err == nil {
			pr.refCtx = refCtx
		}
		if err != nil {
			return err
		}
		ready.Profiles = append(ready.Profiles, pr)
		if p.opts.SkipRemoteRefs {
			continue
		}
		for _, ref := range refs {
			child, err := p.prepare(spec.SpecSourceRecipe{SourceRecipe: spec.SourceRecipe{URI: ref.URI, SpecFile: ref.SpecFile}}, r.sourceReady.DirectoryPath, &ref, false)
			if err != nil {
				return fmt.Errorf("ref %s: %w", ref, err)
			}
			if child != nil {
				ready.Include = append(ready.Include, child)
			}
		}
	}
	return nil
}

func (r *SpecRecipe) makeProfileReady(p *specsPrep, rec spec.ProfileRecipe, lookup []spec.ProfileRecipe, env map[string]string) (*ProfileReady, []spec.ProfileSourceRecipe, error) {
	name := rec.Source.GetProfileName()
	effectiveEnv := overlayEnv(p.ctx.Env, env)
	// [why] ${invokingSpecGitRoot} anchors dests at the top-level spec's checkout, letting a
	effectiveEnv = overlayEnv(effectiveEnv, map[string]string{"invokingSpecGitRoot": p.gitRoot})
	wd, err := resolveWorkingDir(effectiveEnv, rec.Source.DirectoryPath, rec.Options.ProfileWorkingDirectory)
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q: %w", name, err)
	}
	ops, sourced, err := rec.MakeProfile(lookup, wd)
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q: %w", name, err)
	}
	seams := NewSeams(p.home)
	specDone := p.startSpec(seams.Ledger, r.sourceReady.DefinitionURI)
	profileDone, err := seams.Ledger.StartProfile(specDone, rec.Source.String(), name, r.sourceReady.DefinitionURI, rec.Source.DirectoryPath)
	if err != nil {
		log.EmitTrace("ledger", "error", "start profile: "+err.Error())
	}
	pr := &ProfileReady{
		Source: spec.ProfileSourceReady{
			SourceReady: spec.SourceReady{DefinitionURI: r.sourceReady.DefinitionURI, DirectoryPath: rec.Source.DirectoryPath},
			ProfileName: name,
		},
		Options:     rec.Options,
		Env:         env,
		env:         effectiveEnv,
		Profiles:    sourced,
		ref:         rec.Source.DisplayRef(),
		workingDir:  wd,
		opts:        p.opts,
		home:        p.home,
		runID:       p.ctx.RunID,
		runTs:       p.ctx.RunTs,
		tel:         p.tel,
		command:     p.ctx.Command,
		specDone:    specDone,
		profileDone: profileDone,
		Seams:       seams,
	}
	pr.OperationsReady, err = pr.prepareOperations(ops)
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q: %w", name, err)
	}
	return pr, sourced, nil
}

// [<] 🤖🤖

// [>] 🤖🤖 SpecReady

type SpecReady struct {
	Source   spec.SpecSourceReady
	Include  []*SpecReady
	Options  spec.Options
	Env      map[string]string
	Profiles []*ProfileReady
	Rejected []spec.Rejection
	recipes  []spec.ProfileRecipe
	tel      *telemetry.Telemetry
}

func (s *SpecReady) LogRejected() {
	for _, r := range s.Rejected {
		log.Emit(log.Event{
			Level: log.Levels.Debug, Scope: "discover-profiles", Action: "run",
			Msg: "profile " + r.Ref, Reasons: []string{"runIf failed: " + r.Cond},
			Attrs: map[string]string{"profile": r.Ref, "condition": r.Cond},
		})
	}
}

func (s *SpecReady) LogDiscovered() {
	s.LogRejected()
	for _, p := range s.AllProfiles() {
		p.logDiscovered()
	}
}

func (s *SpecReady) AllProfiles() []*ProfileReady {
	out := slices.Clone(s.Profiles)
	for _, c := range s.Include {
		out = append(out, c.AllProfiles()...)
	}
	return out
}

func (s *SpecReady) ExecEach(ctx context.Context, opName string, fn func(context.Context, *ProfileReady) error) error {
	ctx, span := s.tel.Span(ctx, opName, attribute.String("op", opName))
	defer span.End()
	s.LogDiscovered()
	var fails []error
	for _, p := range s.AllProfiles() {
		// [why] a profile whose command ops carry no delta at all is skipped
		if ops := p.commandOps(opName); len(ops) == 0 {
			reasons := []string{"not defined"}
			if scoped := p.commandScopedOps(opName); len(scoped) > 0 {
				reasons = nil
				for _, op := range scoped {
					if r := p.skipOpsReason(op.Name()); !slices.Contains(reasons, r) {
						reasons = append(reasons, r)
					}
				}
			}
			log.EmitSkip(log.Levels.Debug, opName, "run", "profile "+p.Ref(), reasons...)
			continue
		} else if p.commandDelta(opName) == 0 && !p.isDryRunAll() {
			// [why] dry-run=all wants every dest's true state, so it bypasses
			log.EmitSkip(log.Levels.Debug, opName, "run", "profile "+p.Ref(), "no changes")
			continue
		}
		pctx, pspan := s.tel.Span(ctx, "profile", attribute.String("profile", p.Ref()))
		s.tel.CountProfile(pctx, p.Ref())
		log.EmitDebug(opName, "will-run", "profile "+p.Ref()+": "+p.describeOpDeltas(opName))
		p.emitHeading(log.Levels.Info, opName, "run-profile", "Run profile "+p.Ref(), 1)
		p.logDepth = 1
		err := fn(pctx, p)
		if err != nil {
			pspan.RecordError(err)
			fails = append(fails, fmt.Errorf("%s: %w", p.Ref(), err))
		}
		pspan.End()
		if errors.Is(err, ErrErrexit) {
			break
		}
	}
	for _, err := range fails {
		log.EmitError(opName, "fail", err.Error())
	}
	return errors.Join(fails...)
}

// [<] 🤖🤖

// [>] 🤖🤖 ProfileReady

type ProfileReady struct {
	Source          spec.ProfileSourceReady
	Options         spec.ProfileOptions
	Env             map[string]string
	Profiles        []spec.ProfileSourceRecipe
	OperationsReady []operationReady
	ref             string
	logDepth        int
	workingDir      string
	refCtx          map[string]string
	opts            options.Options
	home            string
	env             map[string]string
	runID           string
	runTs           string
	tel             *telemetry.Telemetry
	opCtx           context.Context
	command         string
	specDone        *database.SpecDone
	profileDone     *database.ProfileDone
	backedUp        bool
	backupArchive   string
	currentArchive  string
	currentSub      string
	Seams
}

type opInfo struct {
	kind   string
	target string
	srcRel string
	mode   string
	owner  string
}

func (p *ProfileReady) Ref() string { return p.ref }

// [>] 🤖🤖 execution-context accessors: options + minimal state, profile-scoped

func (p *ProfileReady) resolveRepoRoot() string { return p.Source.DirectoryPath }

func (p *ProfileReady) resolveRoot() string { return p.workingDir }

func (p *ProfileReady) resolveProfileName() string { return p.Source.GetProfileName() }

func (p *ProfileReady) isDryRun() bool { return p.opts.DryRun != options.DryRun.Off }

func (p *ProfileReady) isDryRunAll() bool { return p.opts.DryRun == options.DryRun.All }

func (p *ProfileReady) emit(level log.Level, scope, action, msg string) {
	log.Emit(log.Event{
		Level: level, Scope: scope, Action: action, Msg: msg,
		Attrs: map[string]string{"profile": p.Ref()}, Depth: p.logDepth,
	})
}

func (p *ProfileReady) emitSkip(level log.Level, scope, action, msg string, reasons ...string) {
	log.Emit(log.Event{
		Level: level, Scope: scope, Action: action, Msg: msg, Reasons: reasons,
		Attrs: map[string]string{"profile": p.Ref()}, Depth: p.logDepth,
	})
}

func (p *ProfileReady) emitHeading(level log.Level, scope, action, msg string, heading int) {
	log.Emit(log.Event{
		Level: level, Scope: scope, Action: action, Msg: msg,
		Attrs: map[string]string{"profile": p.Ref()}, Heading: heading,
	})
}

func (p *ProfileReady) emitDryRun(scope, action, msg string) {
	log.Emit(log.Event{
		Level: log.Levels.Info, Scope: scope, Action: action, Msg: msg, DryRun: true,
		Attrs: map[string]string{"profile": p.Ref(), "dryRun": string(p.opts.DryRun)}, Depth: p.logDepth,
	})
}

func (p *ProfileReady) mutate(scope, action, msg, dest string, info opInfo, fn func() error) error {
	prev := p.classifyDest(dest)
	if p.isDryRun() {
		if action == "create" && prev.Present {
			action = "overwrite"
		}
		p.emitDryRun(scope, action, msg)
		return nil
	}
	if err := fn(); err != nil {
		return err
	}
	p.emit(log.Levels.Info, scope, resolvePastAction(action, prev.Present), msg)
	p.recordOperation(dest, info, prev)
	if info.kind != "" && dest != "" {
		p.tel.CountUnit(p.opContext(), info.kind, deriveOpType(prev, p.classifyDest(dest)), p.command)
	}
	return nil
}

func (p *ProfileReady) opContext() context.Context {
	if p.opCtx != nil {
		return p.opCtx
	}
	return context.Background()
}

func (p *ProfileReady) classifyDest(dest string) database.Object {
	if dest == "" {
		return database.Object{Kind: "absent"}
	}
	fi, err := p.Reader.LstatPath(dest)
	if err != nil {
		return database.Object{Kind: "absent"}
	}
	obj := database.Object{Present: true, Mode: fsutil.FormatModeArg(fi.Mode().Perm())}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		obj.Kind = "link"
		obj.Target, _ = p.Reader.ReadLink(dest)
	case fi.IsDir():
		obj.Kind = "dir"
	default:
		obj.Kind = "file"
	}
	return obj
}

func (p *ProfileReady) recordOperation(dest string, info opInfo, prev database.Object) {
	if p.Ledger == nil || p.profileDone == nil || dest == "" || info.kind == "" {
		return
	}
	next := p.classifyDest(dest)
	op := database.OperationDone{
		OpType: deriveOpType(prev, next),
		Kind:   info.kind,
		Dest:   dest,
		Target: info.target,
		SrcRel: info.srcRel,
		Mode:   info.mode,
		Owner:  info.owner,
		Prev:   prev,
		Next:   next,
	}
	if backup, err := p.Ledger.EnsureBackup(p.specDone, p.currentArchive, p.currentSub); err == nil && backup != nil {
		op.BackupID = &backup.ID
	}
	if err := p.Ledger.RecordOperation(p.profileDone, op); err != nil {
		log.EmitTrace("ledger", "error", "record op: "+err.Error())
	}
}

func (p *ProfileReady) skippedOps() []string {
	return slices.Concat(p.opts.SkipOps, p.opts.RunSkipOps)
}

func (p *ProfileReady) logDiscovered() {
	attrs := map[string]string{"profile": p.Ref(), "workingDirectory": p.workingDir}
	var lines []string
	for _, op := range p.commandOps("run") {
		all, delta := op.counts(p)
		attrs[op.Name()+".delta"] = fmt.Sprint(delta)
		attrs[op.Name()+".all"] = fmt.Sprint(all)
		switch {
		case log.IsEnabled(log.Levels.Debug):
			lines = append(lines, fmt.Sprintf("%s: %s (%d declared)", op.Name(), formatChanges(delta), all))
		case delta > 0:
			lines = append(lines, fmt.Sprintf("%s: %s", op.Name(), formatChanges(delta)))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "no changes")
	}
	heading := fmt.Sprintf("Profile %s  (profile workdir: %s)",
		p.Ref(), abbreviateHome(p.workingDir, p.home))
	log.Emit(log.Event{Level: log.Levels.Info, Scope: "discover-profiles", Action: "discovered", Msg: heading, Attrs: attrs, Heading: 2})
	for _, l := range lines {
		log.Emit(log.Event{Level: log.Levels.Info, Scope: "discover-profiles", Msg: l, Depth: 1})
	}
}

func formatChanges(n int) string {
	switch n {
	case 0:
		return "no changes"
	case 1:
		return "1 change"
	default:
		return fmt.Sprintf("%d changes", n)
	}
}

func abbreviateHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func (p *ProfileReady) commandOps(opName string) []operationReady {
	skips := p.skippedOps()
	var out []operationReady
	for _, op := range p.commandScopedOps(opName) {
		if !slices.Contains(skips, op.Name()) {
			out = append(out, op)
		}
	}
	return out
}

func (p *ProfileReady) commandScopedOps(opName string) []operationReady {
	var out []operationReady
	for _, op := range p.OperationsReady {
		if !op.Selected() {
			continue
		}
		switch opName {
		case "run":
		case "backup":
			switch op.(type) {
			case *MakeLinksOperationReady, *MakeCopiesOperationReady, *RenderTemplatesOperationReady:
			default:
				continue
			}
		default:
			if op.Name() != opName {
				continue
			}
		}
		out = append(out, op)
	}
	return out
}

func (p *ProfileReady) commandDelta(opName string) int {
	total := 0
	for _, op := range p.commandOps(opName) {
		_, delta := op.counts(p)
		total += delta
	}
	return total
}

func (p *ProfileReady) describeOpDeltas(opName string) string {
	var parts []string
	for _, op := range p.commandOps(opName) {
		_, delta := op.counts(p)
		parts = append(parts, fmt.Sprintf("%s (%s)", op.Name(), formatChanges(delta)))
	}
	return strings.Join(parts, ", ")
}

func (p *ProfileReady) LogDiscovered() { p.logDiscovered() }

func resolvePastAction(action string, existed bool) string {
	if action != "create" {
		return action
	}
	if existed {
		return "overwritten"
	}
	return "created"
}

func (p *ProfileReady) skipOpsReason(op string) string {
	if slices.Contains(p.opts.SkipOps, op) {
		return "options.skipOps"
	}
	return "options.run.skipOps"
}

func (p *ProfileReady) wouldAction(dest string) string {
	if _, err := p.Reader.LstatPath(dest); err == nil {
		return "overwrite"
	}
	return "create"
}

func (p *ProfileReady) skipReasons(reasons ...string) []string {
	return reasons
}

func deriveOpType(prev, next database.Object) string {
	switch {
	case !prev.Present && next.Present:
		return "create"
	case prev.Present && !next.Present:
		return "remove"
	case prev.Kind == next.Kind && prev.Target == next.Target && prev.Mode == next.Mode:
		return "noop"
	default:
		return "update"
	}
}

func (p *ProfileReady) resolveSrc(relativePath string) string {
	return filepath.Join(p.resolveRoot(), relativePath)
}

func (p *ProfileReady) toDest(relativePath string) string {
	relativePath = p.expandEnv(relativePath)
	if strings.HasPrefix(relativePath, "/") {
		return relativePath
	}
	return "/" + relativePath
}

func (p *ProfileReady) expandEnv(path string) string {
	return os.Expand(path, func(k string) string {
		if k == "HOME" {
			return p.home
		}
		return p.env[k]
	})
}

func (p *ProfileReady) expandHome(path string) string {
	return fsutil.ExpandHome(p.expandEnv(path), p.home)
}

// [<] 🤖🤖

func (p *ProfileReady) withLogLevel(fn func() error) error {
	if p.Options.LogLevel == "" {
		return fn()
	}
	level, err := log.ParseLevel(p.Options.LogLevel)
	if err != nil {
		return fn() // [why] schema validation reports the bad value; the che level stays
	}
	prev := log.GetLevel()
	log.SetLevel(level)
	defer log.SetLevel(prev)
	return fn()
}

func (p *ProfileReady) ExecOperations(ctx context.Context) error {
	return p.withLogLevel(func() error {
		var fails []error
		skippedKinds := map[string]string{"make-links": "link", "make-copies": "copy", "render-templates": "render"}
		skipOps := slices.Concat(p.opts.SkipOps, p.opts.RunSkipOps)
		for _, op := range p.OperationsReady {
			// [why] a config-skipped op is a plain skip, never a sweep: skipping
			if slices.Contains(skipOps, op.Name()) {
				p.emitSkip(log.Levels.Debug, "run", "run-op", op.Name(), p.skipOpsReason(op.Name()))
				continue
			}
			if !op.Selected() {
				p.emitSkip(log.Levels.Debug, "run", "run-op", op.Name(), "not defined")
				if kind, ok := skippedKinds[op.Name()]; ok && !p.isDryRun() {
					fails = append(fails, p.sweepStale(kind, nil)) // [why] emptied op: sweep all prior dests of its kind
				}
				continue
			}
			if err := p.execOp(ctx, op); err != nil {
				fails = append(fails, fmt.Errorf("%s: %w", op.Name(), err))
				if errors.Is(err, ErrErrexit) {
					break
				}
			}
		}
		return errors.Join(fails...)
	})
}

func (p *ProfileReady) execOp(ctx context.Context, op operationReady) error {
	ctx, span := p.tel.Span(ctx, op.Name(), attribute.String("op", op.Name()))
	defer span.End()
	prev := p.opCtx
	p.opCtx = ctx
	defer func() { p.opCtx = prev }()
	p.tel.CountOperation(ctx, op.Name())
	defer p.enterOp(op.Name())()
	err := op.execOperation(p)
	if err != nil {
		span.RecordError(err)
		p.tel.CountError(ctx, op.Name())
	}
	return err
}

func (p *ProfileReady) enterOp(name string) func() {
	msg := name
	if op := p.opNamed(name); op != nil {
		if _, delta := op.counts(p); delta == 0 {
			msg += " (no changes)"
		}
	}
	prev := p.logDepth
	p.emitHeading(log.Levels.Info, "run", "running", msg, prev+1)
	p.logDepth = 1 // [why] headings carry the nesting; body lines sit one step in
	return func() { p.logDepth = prev }
}

func (p *ProfileReady) opNamed(name string) operationReady {
	for _, op := range p.OperationsReady {
		if op.Name() == name {
			return op
		}
	}
	return nil
}

func (p *ProfileReady) ExecOperation(ctx context.Context, op operationReady) error {
	return p.withLogLevel(func() error {
		if !op.Selected() {
			p.emitSkip(log.Levels.Debug, "run", "run-op", op.Name(), "not defined")
			return nil
		}
		return p.execOp(ctx, op)
	})
}

func (p *ProfileReady) ExecOperationNamed(ctx context.Context, name string) error {
	if slices.Contains(p.opts.SkipOps, name) {
		p.emitSkip(log.Levels.Debug, "run", "run-op", name, p.skipOpsReason(name))
		return nil
	}
	for _, op := range p.OperationsReady {
		if op.Name() == name {
			return p.ExecOperation(ctx, op)
		}
	}
	return nil
}

func (p *ProfileReady) ExecRunScripts(ctx context.Context, names []string) (int, error) {
	if slices.Contains(p.opts.SkipOps, "run-scripts") {
		p.emitSkip(log.Levels.Debug, "run", "run-op", "run-scripts", p.skipOpsReason("run-scripts"))
		return 0, nil
	}
	matched := 0
	err := p.withLogLevel(func() error {
		for _, op := range p.OperationsReady {
			rs, ok := op.(*RunScriptsOperationReady)
			if !ok {
				continue
			}
			sctx, span := p.tel.Span(ctx, "run-scripts", attribute.String("op", "run-scripts"))
			defer span.End()
			prev := p.opCtx
			p.opCtx = sctx
			defer func() { p.opCtx = prev }()
			defer p.enterOp("run-scripts")()
			scripts := filterScriptsByName(rs.Scripts, names)
			matched = len(scripts)
			return p.runScripts(scripts)
		}
		return nil
	})
	return matched, err
}

func filterScriptsByName(scripts, names []string) []string {
	if len(names) == 0 {
		return scripts
	}
	var out []string
	for _, s := range scripts {
		if slices.ContainsFunc(names, func(n string) bool { return strings.Contains(s, n) }) {
			out = append(out, s)
		}
	}
	return out
}

// [<] 🤖🤖

// [>] 🤖🤖 operation parents

type OperationReady struct{}

type operationReady interface {
	execOperation(p *ProfileReady) error
	Selected() bool
	Name() string
	counts(p *ProfileReady) (all, delta int)
}

func (p *ProfileReady) prepareOperations(ops spec.OperationRecipes) ([]operationReady, error) {
	scripts, err := p.resolveScripts(ops.RunScripts.Scripts)
	if err != nil {
		return nil, err
	}
	return []operationReady{
		&PruneLinksOperationReady{Dirs: ops.PruneLinks.Dirs},
		&MakeDirsOperationReady{Dirs: ops.MakeDirs.Dirs},
		&MakeLinksOperationReady{Links: ops.MakeLinks.Links, Dirs: ops.MakeLinks.Dirs},
		&MakeCopiesOperationReady{Copies: ops.MakeCopies.Copies, Dirs: ops.MakeCopies.Dirs},
		&RenderTemplatesOperationReady{Templates: ops.RenderTemplates.Templates, SkipSecrets: p.opts.RenderSkipSecrets},
		&InstallPackagesOperationReady{Packages: ops.InstallPackages.Packages},
		&RunScriptsOperationReady{Scripts: scripts},
	}, nil
}

// [<] 🤖🤖

// [>] 🤖🤖 per-kind operations

type PruneLinksOperationReady struct {
	OperationReady
	Dirs []string
}

func (o *PruneLinksOperationReady) Name() string   { return "prune-broken-links" }
func (o *PruneLinksOperationReady) Selected() bool { return len(o.Dirs) > 0 }
func (o *PruneLinksOperationReady) execOperation(p *ProfileReady) error {
	return p.pruneBrokenLinks()
}

func (o *PruneLinksOperationReady) counts(p *ProfileReady) (int, int) {
	links, err := p.installedLinks()
	if err != nil {
		return 0, 0
	}
	broken := 0
	for _, op := range links {
		if !p.linkSourcePresent(op) {
			broken++
		}
	}
	return len(links), broken
}

type MakeDirsOperationReady struct {
	OperationReady
	Dirs []spec.FileItem
}

func (o *MakeDirsOperationReady) Name() string                        { return "make-dirs" }
func (o *MakeDirsOperationReady) Selected() bool                      { return len(o.Dirs) > 0 }
func (o *MakeDirsOperationReady) execOperation(p *ProfileReady) error { return p.makeDirs(o.Dirs) }

func (o *MakeDirsOperationReady) counts(p *ProfileReady) (int, int) {
	all, delta := 0, 0
	for _, item := range o.Dirs {
		dest := p.toDest(item.Rel)
		if len(item.Dests) > 0 {
			dest = p.toDest(item.Dests[0].Path)
		}
		all++
		if !fsutil.IsDirSettled(p.Reader, dest) {
			delta++
		}
	}
	return all, delta
}

type MakeLinksOperationReady struct {
	OperationReady
	Links []spec.FileItem
	Dirs  []string
}

func (o *MakeLinksOperationReady) Name() string   { return "make-links" }
func (o *MakeLinksOperationReady) Selected() bool { return len(o.Links) > 0 }
func (o *MakeLinksOperationReady) execOperation(p *ProfileReady) error {
	return p.makeLinks(o.Links, o.Dirs)
}

func (o *MakeLinksOperationReady) counts(p *ProfileReady) (int, int) {
	all, delta := 0, 0
	for _, item := range o.Links {
		for _, dest := range p.resolveLinkDests(item) {
			all++
			if !fsutil.IsLinkSettled(p.Reader, p.resolveSrc(item.Rel), dest) {
				delta++
			}
		}
	}
	return all, delta
}

type MakeCopiesOperationReady struct {
	OperationReady
	Copies []spec.FileItem
	Dirs   []string
}

func (o *MakeCopiesOperationReady) Name() string   { return "make-copies" }
func (o *MakeCopiesOperationReady) Selected() bool { return len(o.Copies) > 0 }
func (o *MakeCopiesOperationReady) execOperation(p *ProfileReady) error {
	return p.makeCopies(o.Copies, o.Dirs)
}

func (o *MakeCopiesOperationReady) counts(p *ProfileReady) (int, int) {
	all, delta := 0, 0
	for _, item := range o.Copies {
		src := p.resolveSrc(item.Rel)
		for _, dest := range p.resolveCopyDests(item) {
			all++
			if !fsutil.IsSameContent(p.Reader, src, dest) {
				delta++
			}
		}
	}
	return all, delta
}

type RenderTemplatesOperationReady struct {
	OperationReady
	Templates   []spec.FileItem
	SkipSecrets bool
}

func (o *RenderTemplatesOperationReady) Name() string   { return "render-templates" }
func (o *RenderTemplatesOperationReady) Selected() bool { return len(o.Templates) > 0 }
func (o *RenderTemplatesOperationReady) execOperation(p *ProfileReady) error {
	return p.renderTemplates(o.Templates, o.SkipSecrets)
}

func (o *RenderTemplatesOperationReady) counts(p *ProfileReady) (int, int) {
	return p.renderCounts(o.Templates)
}

type InstallPackagesOperationReady struct {
	OperationReady
	Packages []spec.PackageRef
}

func (o *InstallPackagesOperationReady) Name() string   { return "install-packages" }
func (o *InstallPackagesOperationReady) Selected() bool { return len(o.Packages) > 0 }
func (o *InstallPackagesOperationReady) execOperation(p *ProfileReady) error {
	return p.installPackages(o.Packages)
}

func (o *InstallPackagesOperationReady) counts(_ *ProfileReady) (int, int) {
	return len(o.Packages), len(o.Packages)
}

type RunScriptsOperationReady struct {
	OperationReady
	Scripts []string
}

func (o *RunScriptsOperationReady) Name() string   { return "run-scripts" }
func (o *RunScriptsOperationReady) Selected() bool { return len(o.Scripts) > 0 }
func (o *RunScriptsOperationReady) execOperation(p *ProfileReady) error {
	return p.runScripts(o.Scripts)
}

func (o *RunScriptsOperationReady) counts(_ *ProfileReady) (int, int) {
	return len(o.Scripts), len(o.Scripts)
}

// [<] 🤖🤖
