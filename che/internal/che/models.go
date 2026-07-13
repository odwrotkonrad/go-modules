// Package che assembles executable profiles from spec recipes: options
// preparation, spec composition (include.sources + sourced include.profiles
// refs, fully recursive), per-profile operation preparation and execution.
package che

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// Domain model:
//
//	SpecRecipe  one che.yml, declared: Source + top-level options/env/include
//	            + parsed ProfileRecipes
//	SpecReady   its resolved counterpart: Include tree (composed specs,
//	            recursive) + the profiles THIS spec assembled itself
//	ProfileReady  one resolved profile, ready to install onto the OS:
//	              effective options, env overlay, prepared operations
//	Operations  per-subcommand Recipe/Ready pairs: a Recipe (spec) carries
//	            UNRESOLVED subjects, its Ready the RESOLVED ones; execOperation
//	            runs each op's logic against its owning *ProfileReady
//
// cli holds opts (options.Options) and the root *SpecReady as separately
// initialized values: PrepareApplicationOptions, then PrepareSpecs.

// Seams are the fs surfaces a profile drives to touch the host: mutating
// writer, dest-facing reader, remote template fetcher. Exported so tests can
// inject record-only mocks via the NewSeams swap point.
type Seams struct {
	FS      fsutil.FileSystemWriter
	Reader  fsutil.FileSystemReader
	Fetcher RemoteFetcher
	Ledger  *database.DB // ops ledger; nil records nothing (tests, closed store)
}

// NewSeams builds a profile's real fs seams (home-anchored writer, OS reader,
// git remote fetcher, ops ledger); tests swap it to inject record-only mocks
// with a nil Ledger. A failed ledger Open degrades to nil (recording off), never
// aborting the run.
var NewSeams = func(home string) Seams {
	db, err := database.Open(filepath.Join(fsutil.ResolveStateHome(home), "ops.db"))
	if err != nil {
		log.Debug("ledger", "open failed: "+err.Error(), log.Off)
		db = nil
	}
	return Seams{
		FS:      fsutil.FS{Home: home},
		Reader:  fsutil.OSReader{},
		Fetcher: gitFetcher{fetch: render.NewRemoteFetcher()},
		Ledger:  db,
	}
}

// RemoteFetcher fetches a remote template source ref's content
// (<repo>//<path>[?ref=<ref>], marker stripped).
type RemoteFetcher interface {
	Fetch(ref string) (string, error)
}

// gitFetcher is the live RemoteFetcher: shallow in-memory git clones, one
// clone cache shared across the profile's renders.
type gitFetcher struct{ fetch func(string) (string, error) }

func (g gitFetcher) Fetch(ref string) (string, error) { return g.fetch(ref) }

// [<] 🤖🤖

// [>] 🤖🤖 package-level funcs

// PrepareApplicationOptions finalizes the runtime options: chdir (-C), locate the repo,
// then resolve with per-field precedence flags > env vars > the user-config
// file ($XDG_CONFIG_HOME/che/config.yml) > local che.yml options: > defaults.
// ctx carries the captured launch world (env/cwd); -C shifts ctx.Cwd forward.
func PrepareApplicationOptions(ctx Context, opts options.Options) (Context, options.Options, error) {
	resolvedOptions := opts
	resolvedOptions.Dir = cmp.Or(resolvedOptions.Dir, ctx.Env["CHE_DIR"])
	if resolvedOptions.Dir != "" {
		next, err := changeDir(ctx.Cwd, resolvedOptions.Dir)
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
	log.SetDebug(resolvedOptions.Debug)
	return ctx, resolvedOptions, nil
}

// changeDir resolves the -C target against cwd (absolute stays, relative joins),
// verifying it is an existing dir; returns the shifted working directory.
func changeDir(cwd, dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("chdir %s: not a directory", dir)
	}
	return dir, nil
}

// readUserLayer leniently reads the user-config file: a bare options: object
// ($XDG_CONFIG_HOME/che/config.yml) mirroring the che.yml options: block
// ([why] absent file -> empty layer; parse errors surface later).
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

// readSpecLayer leniently reads the local spec's options: block into a
// resolution layer ([why] absent file / parse errors surface later, at
// PrepareSpecs).
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

// PrepareSpecs resolves the root spec and its whole Include tree (top-level
// include.sources + sourced include.profiles refs), fully recursive,
// cycle-guarded, deduped by source URI + profile name. ctx carries the captured
// launch world, held once on specsPrep for the whole prepare pass.
func PrepareSpecs(ctx Context, opts options.Options, src spec.SpecSourceRecipe) (*SpecReady, error) {
	repoRoot, err := findRepoRoot(ctx)
	if err != nil {
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
		seenSpecs: map[string]bool{},
		seenRefs:  map[string]bool{},
	}
	return p.prepare(src, repoRoot, nil, true)
}

// specsPrep threads the shared composition state through the recursion.
type specsPrep struct {
	ctx       Context
	opts      options.Options
	home      string
	seenSpecs map[string]bool    // resolved spec dirs (include.sources cycle/dup guard)
	seenRefs  map[string]bool    // <uri>::<profile> (sourced-ref dedup)
	specDone  *database.SpecDone // the run's ledger row, created lazily by the first recording profile
}

// startSpec lazily creates (once per run) the ledger SpecDone row on the shared
// ledger handle, keyed by the run's TsLayout stamp. Later profiles reuse it.
func (p *specsPrep) startSpec(db *database.DB, uri string) *database.SpecDone {
	if p.specDone != nil {
		return p.specDone
	}
	s, err := db.StartSpec(p.ctx.RunID, uri, p.ctx.Command)
	if err != nil {
		log.Debug("ledger", "start spec: "+err.Error(), log.Off)
		return nil
	}
	p.specDone = s
	return s
}

// prepare runs one spec through the full pipeline. forced pins the spec to a
// single referenced profile (sourced include.profiles ref); root marks the
// user-invoked spec (zero eligible profiles is fatal there only). Returns nil
// for an include.sources duplicate.
func (p *specsPrep) prepare(src spec.SpecSourceRecipe, anchor string, forced *spec.ProfileSourceRecipe, root bool) (*SpecReady, error) {
	recipe := &SpecRecipe{Source: src}
	if err := recipe.PrepareSpec(anchor, p.home); err != nil {
		return nil, err
	}
	if forced != nil {
		// [why] keyed on the resolved dir, not the raw URI: the same spec is
		// referenced under different relative URIs across hops.
		key := recipe.sourceReady.DirectoryPath + "::" + forced.ProfileName
		if p.seenRefs[key] {
			log.Debug("source("+forced.ProfileName+")", "skip duplicate "+forced.String(), log.Off)
			return nil, nil
		}
		p.seenRefs[key] = true
	} else if !root && p.seenSpecs[recipe.sourceReady.DirectoryPath] {
		log.Debug("source(spec)", "skip duplicate "+recipe.sourceReady.DirectoryPath, log.Off)
		return nil, nil
	}
	p.seenSpecs[recipe.sourceReady.DirectoryPath] = true
	if err := recipe.PrepareProfileRecipes(p.opts); err != nil {
		return nil, err
	}
	return recipe.PrepareProfiles(p, forced, root)
}

// findRepoRoot: git toplevel of ctx.Cwd, che.yml must live there (che's
// defining marker).
func findRepoRoot(ctx Context) (string, error) {
	root, err := fsutil.ResolveRepoRoot(ctx.Cwd)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "che.yml")); err != nil {
		return "", fmt.Errorf("che.yml not found at repo root %s", root)
	}
	return root, nil
}

// resolveInvokingHome resolves the invoking user's home. Under sudo (EUID 0,
// SUDO_USER set), looks up that user's home from passwd so dest paths derive
// from the real user, not /var/root. Otherwise uses $HOME.
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

// overlayEnv derives a new env map from base with overlay's keys applied
// (overlay wins), the pure replacement for the old process-env shadowing. Empty
// overlay returns base unchanged.
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

// SpecRecipe is one declared che.yml: where it comes from, its reserved
// top-level blocks, its parsed profile recipes.
type SpecRecipe struct {
	Source         spec.SpecSourceRecipe   // where this che.yml comes from
	Include        []spec.SpecSourceRecipe // top-level include.sources: other specs composed in
	Options        spec.Options            // top-level options: block
	Env            map[string]string       // top-level env: block
	ProfileRecipes []spec.ProfileRecipe
	sourceReady    spec.SpecSourceReady
}

// PrepareSpec resolves the source (filesystem expand / remote clone) and
// locates che.yml at its DirectoryPath.
func (r *SpecRecipe) PrepareSpec(anchor, home string) error {
	ready, err := r.Source.PrepareSource(anchor, home)
	if err != nil {
		return err
	}
	r.sourceReady = ready
	return nil
}

// PrepareProfileRecipes parses and schema-validates the spec, then stamps each
// recipe with its Source (effective directory in Source.DirectoryPath, option
// cascade applied: profile > spec).
func (r *SpecRecipe) PrepareProfileRecipes(opts options.Options) error {
	if err := r.validateSchema(opts.ValidateSpecCLI); err != nil {
		return err
	}
	doc, err := spec.Load(r.sourceReady.DefinitionURI)
	if err != nil {
		return err
	}
	r.Options, r.Env, r.Include = doc.Options, doc.Env, doc.Include
	// [why] che-level (flag/env/user-config) seeds the spec default, cascading
	// down to each profile: profile > spec > user-config.
	if r.Options.WorkingDirectory == "" {
		r.Options.WorkingDirectory = opts.WorkingDirectory
	}
	if r.Options.AutoDiscover == nil {
		r.Options.AutoDiscover = opts.AutoDiscover
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

// resolveWorkingDir resolves a profile's effective options.workingDirectory
// onto the checkout anchor: empty -> the checkout itself, else env-expanded
// (~/, $VAR, env vars against env), relative -> under the checkout. Must be an
// existing dir.
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
		return "", fmt.Errorf("options.workingDirectory not found: %s (from %q)", dir, directory)
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
		log.Msg("validate(che.yml)", f, log.Off)
	}
	return nil
}

// peekSpecValidateMode leniently peeks a spec's own options.validateSpec from
// its bytes ("" if absent or unparseable; the schema check surfaces real errors).
func peekSpecValidateMode(b []byte) string {
	var d struct {
		Options spec.Options `yaml:"options"`
	}
	if err := yaml.Unmarshal(b, &d); err != nil {
		return ""
	}
	return d.Options.ValidateSpec
}

// PrepareProfiles assembles the SpecReady: spec-level execIf gate, top-level
// include.sources composed (their recipes joining the local-ref lookup),
// per-recipe eligibility, one MakeProfile per eligible recipe, sourced refs
// spawning nested SpecReadys (recursive).
func (r *SpecRecipe) PrepareProfiles(p *specsPrep, forced *spec.ProfileSourceRecipe, root bool) (*SpecReady, error) {
	ready := &SpecReady{Source: r.sourceReady, Options: r.Options, Env: r.Env, recipes: r.ProfileRecipes}
	// [why] the spec-level env: overlay gates this spec's own execIf.
	eval := p.evalWith(r.Env)
	pass, err := spec.AllPass("spec "+r.sourceReady.DefinitionURI, r.Options.ExecIf, p.opts.SkipExecIf, eval)
	if err != nil {
		return nil, err
	}
	if !pass {
		log.Msg("spec(skip)", r.sourceReady.DefinitionURI+" (execIf failed)", log.Off)
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

// evalWith builds an execIf evaluator whose env: source reads the captured
// launch env overlaid with the given profile/spec/ref env ([why] the overlay
// wins, matching the old process-env shadowing without mutating it).
func (p *specsPrep) evalWith(overlay map[string]string) func(string) (bool, error) {
	env := overlayEnv(p.ctx.Env, overlay)
	return spec.NewEvaluator(func(k string) string { return env[k] }).EvalExecIf
}

// composeIncludes prepares each include.sources spec (own anchor, own env
// overlay), nests it under Include, and joins its recipes into the local-ref
// lookup (bare-name collision: error).
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

// selectEligibleNames picks this spec's own profiles to assemble: the forced
// ref's one profile (execIf gated inside its env overlay, skip on fail), or
// EligibleRecipes (zero eligible fatal only at the root spec).
func (r *SpecRecipe) selectEligibleNames(p *specsPrep, forced *spec.ProfileSourceRecipe, root bool) ([]string, error) {
	if forced != nil {
		rec, err := spec.FindRecipe(r.ProfileRecipes, forced.ProfileName)
		if err != nil {
			return nil, fmt.Errorf("ref %s: %w", forced, err)
		}
		// [why] the ref entry's env overlays the launch env for its execIf gate.
		pass, err := spec.AllPass(forced.ProfileName, rec.Options.OverRef(forced.Options).ExecIf, p.opts.SkipExecIf, p.evalWith(forced.Env))
		if err != nil {
			return nil, fmt.Errorf("ref %s: %w", forced, err)
		}
		if !pass {
			log.Msg("source("+forced.ProfileName+")", fmt.Sprintf("skip %s (execIf failed)", forced), log.Off)
			return nil, nil
		}
		return []string{forced.ProfileName}, nil
	}
	var forcedProfiles []string
	if root {
		forcedProfiles = p.opts.Profiles
	}
	names, err := spec.EligibleRecipes(r.ProfileRecipes, forcedProfiles, p.opts.SkipExecIf, p.evalWith(r.Env))
	if err != nil {
		if !root && errors.Is(err, spec.ErrNoneEligible) {
			log.Debug("spec(skip)", r.sourceReady.DefinitionURI+" (no eligible profile)", log.Off)
			return nil, nil
		}
		return nil, err
	}
	return names, nil
}

// assembleProfiles runs MakeProfile per eligible recipe, prepares its
// operations, and recurses into its sourced refs.
func (r *SpecRecipe) assembleProfiles(p *specsPrep, ready *SpecReady, lookup []spec.ProfileRecipe, names []string, forced *spec.ProfileSourceRecipe) error {
	for _, name := range names {
		rec, err := spec.FindRecipe(lookup, name)
		if err != nil {
			return err
		}
		env := r.Env
		if forced != nil {
			rec.Options = rec.Options.OverRef(forced.Options)
			env = fsutil.MergeMap(r.Env, forced.Env)
		}
		pr, refs, err := r.makeProfileReady(p, rec, lookup, env)
		if err != nil {
			return err
		}
		ready.Profiles = append(ready.Profiles, pr)
		if p.opts.SkipRemoteRefs {
			continue
		}
		for _, ref := range refs {
			log.Debug("source("+ref.ProfileName+")", "run "+ref.String(), log.Off)
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

// makeProfileReady resolves one recipe into an executable profile: MakeProfile
// emits the operation recipes, the profile (anchored at the recipe's directory,
// its fs seams built) prepares them (subjects resolved). The profile captures
// the effective launch env (launch env overlaid with its own env: block) so
// downstream op methods read p.env, never the process.
func (r *SpecRecipe) makeProfileReady(p *specsPrep, rec spec.ProfileRecipe, lookup []spec.ProfileRecipe, env map[string]string) (*ProfileReady, []spec.ProfileSourceRecipe, error) {
	name := rec.Source.GetProfileName()
	effectiveEnv := overlayEnv(p.ctx.Env, env)
	wd, err := resolveWorkingDir(effectiveEnv, rec.Source.DirectoryPath, rec.Options.WorkingDirectory)
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
		log.Debug("ledger", "start profile: "+err.Error(), log.Off)
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
		ref:         rec.Source.String(),
		workingDir:  wd,
		opts:        p.opts,
		home:        p.home,
		runID:       p.ctx.RunID,
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

// SpecReady is one resolved spec: the composed specs (a tree) and ONLY the
// profiles this one spec assembled itself.
type SpecReady struct {
	Source   spec.SpecSourceReady
	Include  []*SpecReady // composed specs, resolved recursively
	Options  spec.Options
	Env      map[string]string
	Profiles []*ProfileReady
	recipes  []spec.ProfileRecipe // this spec's recipes, joining the includer's lookup
}

// AllProfiles flattens the tree depth-first: own profiles, then each composed
// spec's.
func (s *SpecReady) AllProfiles() []*ProfileReady {
	out := slices.Clone(s.Profiles)
	for _, c := range s.Include {
		out = append(out, c.AllProfiles()...)
	}
	return out
}

// ExecEach runs fn over every profile in the tree. A failing profile does not
// stop the rest: failures collect (ref-wrapped), report as
// "<op>(report): fail <ref>: <err>" lines after all profiles, and join into
// the returned error.
func (s *SpecReady) ExecEach(opName string, fn func(*ProfileReady) error) error {
	var fails []error
	for _, p := range s.AllProfiles() {
		if err := fn(p); err != nil {
			fails = append(fails, fmt.Errorf("%s: %w", p.Ref(), err))
		}
	}
	for _, err := range fails {
		log.Msg(opName+"(report)", "fail "+err.Error(), log.Off)
	}
	return errors.Join(fails...)
}

// [<] 🤖🤖

// [>] 🤖🤖 ProfileReady

// ProfileReady is one resolved profile, ready to install onto the OS: its
// effective options, env overlay, prepared operations, and the fs seams the op
// logic drives. It IS the execution context — op methods hang off it.
type ProfileReady struct {
	Source          spec.ProfileSourceReady
	Options         spec.ProfileOptions        // effective values after the cascade
	Env             map[string]string          // spec env: merged under the ref entry's env (ref wins)
	Profiles        []spec.ProfileSourceRecipe // sourced refs, consumed by PrepareSpecs
	OperationsReady []operationReady           // prepared, in run order
	ref             string                     // display ref: bare name local, <source>::<name> sourced
	workingDir      string                     // resolved load-ops source tree (options.workingDirectory cascade)
	opts            options.Options
	home            string
	env             map[string]string     // captured launch env overlaid with Env, read by expandEnv/buildScriptsEnv
	runID           string                // the run's TsLayout stamp (backup filenames + ledger run)
	specDone        *database.SpecDone    // the run's ledger row (nil when not recording)
	profileDone     *database.ProfileDone // this profile's ledger row (nil when not recording)
	currentArchive  string                // archive path the in-flight op's archiveBefore wrote ("" -> no backup)
	currentSub      string                // that archive's sub (Backup.Sub)
	Seams                                 // fs writer/reader/fetcher, defaulted in prepare, test-injectable
}

// opInfo carries the kind-specific columns a mutate call fills for its
// OperationDone row: only the fields the op's Kind uses are set.
type opInfo struct {
	kind   string // link | copy | render | dir | chmod | chown | rm
	target string // link target / render header ("" if n/a)
	srcRel string // copy/render source rel ("" if n/a)
	mode   string // octal mode applied ("" if n/a)
	owner  string // "owner:group" applied ("" if n/a)
}

// Ref is the profile's display ref (report lines, discover).
func (p *ProfileReady) Ref() string { return p.ref }

// [>] 🤖🤖 execution-context accessors: options + minimal state, profile-scoped

// resolveRepoRoot is the checkout anchor (che.yml + repo-relative scripts/templates).
func (p *ProfileReady) resolveRepoRoot() string { return p.Source.DirectoryPath }

// resolveRoot is the resolved load-ops source tree (the options.workingDirectory
// value; host op sources resolve against it, the HOME/ folder maps onto it).
func (p *ProfileReady) resolveRoot() string { return p.workingDir }

// resolveProfileName is the resolved profile name (CONFIGS_PROFILE, plist domain).
func (p *ProfileReady) resolveProfileName() string { return p.Source.GetProfileName() }

// buildLogSubtype is the trailing log subtype word: sourced profiles carry
// "profile=<name>" ([why] disambiguates interleaved runs), local ones none.
func (p *ProfileReady) buildLogSubtype() string {
	if p.ref != p.resolveProfileName() {
		return "profile=" + p.resolveProfileName()
	}
	return ""
}

// isDryRun reports whether this is any dry run (delta or all).
func (p *ProfileReady) isDryRun() bool { return p.opts.DryRun != options.DryRun.Off }

// isDryRunAll reports the dry-run=all mode (every dest re-reports, never skips).
func (p *ProfileReady) isDryRunAll() bool { return p.opts.DryRun == options.DryRun.All }

// logMsg logs title/msg at the profile's dry-run mode, carrying its log subtype.
func (p *ProfileReady) logMsg(title, msg string) {
	log.MsgSub(title, msg, toLogMode(p.opts.DryRun), p.buildLogSubtype())
}

// mutate is the one dry-run+log gate for every mutating op: dry run logs only
// (fs untouched); real run executes fn, then logs. On a real run it also records
// the dest mutation into the ops ledger (classify prev before fn, next after),
// keyed by info.kind. dest "" / dry-run / no ledger -> nothing recorded.
func (p *ProfileReady) mutate(title, msg, dest string, info opInfo, fn func() error) error {
	if p.isDryRun() {
		p.logMsg(title, msg)
		return nil
	}
	prev := p.classifyDest(dest)
	if err := fn(); err != nil {
		return err
	}
	p.logMsg(title, msg)
	p.recordOperation(dest, info, prev)
	return nil
}

// classifyDest reads a dest's current state (link/file/dir/absent) for the
// ledger prev/next Object.
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

// recordOperation classifies the dest's post-fn state, derives op_type, resolves
// the in-flight archive as the Backup, and writes the OperationDone. Guarded off
// when not recording (nil ledger/profileDone, no dest, no kind).
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
		log.Debug("ledger", "record op: "+err.Error(), log.Off)
	}
}

// deriveOpType maps a prev/next Object pair to the op_type discriminator.
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

func toLogMode(m options.DryRunMode) log.DryRun {
	switch m {
	case options.DryRun.Delta:
		return log.Delta
	case options.DryRun.All:
		return log.All
	default:
		return log.Off
	}
}

// resolveSrc maps a workingDirectory-relative path to its absolute source path.
func (p *ProfileReady) resolveSrc(relativePath string) string {
	return filepath.Join(p.resolveRoot(), relativePath)
}

// toDest maps a working-tree rel path to its live dest. Env vars expand first
// (so specs can write $HOME/... dests), $HOME resolving to the invoking user's
// home (p.home, correct under sudo where the process $HOME differs). Then: an
// already-absolute path stays (make-extra-dirs entries, $HOME-rooted dests),
// the HOME tree folder maps onto p.home, everything else is a system-root path.
func (p *ProfileReady) toDest(relativePath string) string {
	relativePath = p.expandEnv(relativePath)
	if relativePath == "HOME" {
		return p.home
	}
	if rest, ok := strings.CutPrefix(relativePath, "HOME/"); ok {
		return filepath.Join(p.home, rest)
	}
	if strings.HasPrefix(relativePath, "/") {
		return relativePath
	}
	return "/" + relativePath
}

// expandEnv expands env vars in path from the captured profile env, with
// $HOME/${HOME} bound to p.home ([why] the invoking user's home, not the
// process env, which diverges under sudo).
func (p *ProfileReady) expandEnv(path string) string {
	return os.Expand(path, func(k string) string {
		if k == "HOME" {
			return p.home
		}
		return p.env[k]
	})
}

// expandHome expands env vars ($HOME bound to p.home) then the ~/ prefix, so a
// dest may be written with $VAR / $HOME or ~/.
func (p *ProfileReady) expandHome(path string) string {
	return fsutil.ExpandHome(p.expandEnv(path), p.home)
}

// [<] 🤖🤖

// withDebugLevel runs fn under the profile's effective debug level ([why] a
// profile's options.debug overrides the che-level level for its ops).
func (p *ProfileReady) withDebugLevel(fn func() error) error {
	debug := p.opts.Debug
	if p.Options.Debug != nil {
		debug = *p.Options.Debug
	}
	log.SetDebug(debug)
	defer log.SetDebug(p.opts.Debug)
	return fn()
}

// ExecOperations executes ALL of the profile's operations, in run order:
// Selected() gated (all(skip) debug line), errors join, a failing op does not
// stop the rest. After the ops, it reconciles the ledger: any recorded dest of
// an install kind (link/copy/render) whose op produced nothing this run — the op
// was fully emptied and thus deselected — is swept (removed + archived). Ops that
// ran already swept their own stale dests inline; this covers the emptied-op case
// so a removed-entirely op still prunes its orphans.
func (p *ProfileReady) ExecOperations() error {
	return p.withDebugLevel(func() error {
		var fails []error
		skippedKinds := map[string]string{"make-links": "link", "make-copies": "copy", "render-templates": "render"}
		for _, op := range p.OperationsReady {
			if !op.Selected() {
				log.Debug("all(skip)", op.Name()+" (nothing selected)", log.Off)
				if kind, ok := skippedKinds[op.Name()]; ok && !p.isDryRun() {
					fails = append(fails, p.sweepStale(kind, nil)) // [why] emptied op: sweep all prior dests of its kind
				}
				continue
			}
			log.Msg("all(run)", op.Name(), log.Off)
			if err := op.execOperation(p); err != nil {
				fails = append(fails, fmt.Errorf("%s: %w", op.Name(), err))
			}
		}
		return errors.Join(fails...)
	})
}

// ExecOperation executes one prepared operation (per-op subcommands): same
// debug level and Selected() gating.
func (p *ProfileReady) ExecOperation(op operationReady) error {
	return p.withDebugLevel(func() error {
		if !op.Selected() {
			log.Debug("all(skip)", op.Name()+" (nothing selected)", log.Off)
			return nil
		}
		return op.execOperation(p)
	})
}

// ExecOperationNamed executes the profile's operation named name (no-op if
// the profile prepared none).
func (p *ProfileReady) ExecOperationNamed(name string) error {
	for _, op := range p.OperationsReady {
		if op.Name() == name {
			return p.ExecOperation(op)
		}
	}
	return nil
}

// ExecRunScripts runs the profile's scripts filtered by name substrings,
// returning how many matched.
func (p *ProfileReady) ExecRunScripts(names []string) (int, error) {
	matched := 0
	err := p.withDebugLevel(func() error {
		for _, op := range p.OperationsReady {
			rs, ok := op.(*RunScriptsOperationReady)
			if !ok {
				continue
			}
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

// OperationReady is the shared resolved-operation parent: pure resolved data,
// NO context field — the owning *ProfileReady is passed in at execution time.
type OperationReady struct{}

// operationReady is the one interface the heterogeneous ordered run list
// forces: minimal, unexported. execOperation runs the op's logic against its
// owning profile (the execution context).
type operationReady interface {
	execOperation(p *ProfileReady) error
	Selected() bool
	Name() string
}

// prepareOperations resolves each operation recipe's subjects against the
// profile, returning the prepared operations in run order.
func (p *ProfileReady) prepareOperations(ops spec.OperationRecipes) ([]operationReady, error) {
	scripts, err := p.resolveScripts(ops.RunScripts.Scripts)
	if err != nil {
		return nil, err
	}
	services, err := p.resolveServices(ops.RunServices.Services)
	if err != nil {
		return nil, err
	}
	return []operationReady{
		&PruneLinksOperationReady{Dirs: ops.PruneLinks.Dirs},
		&MakeDirsOperationReady{Dirs: ops.MakeDirs.Dirs},
		&MakeLinksOperationReady{Links: ops.MakeLinks.Links, Dirs: ops.MakeLinks.Dirs},
		&MakeCopiesOperationReady{Copies: ops.MakeCopies.Copies, Dirs: ops.MakeCopies.Dirs},
		&RenderTemplatesOperationReady{Templates: ops.RenderTemplates.Templates, SkipSecrets: p.opts.RenderSkipSecrets},
		&RunScriptsOperationReady{Scripts: scripts},
		&BootoutOperationReady{Services: services},
		&BootinOperationReady{Services: services},
		&EnsureOperationReady{Services: services},
	}, nil
}

// [<] 🤖🤖

// [>] 🤖🤖 per-kind operations

// PruneLinksOperationReady removes ledger-recorded link dests whose source is
// gone. Selected when the profile manages links (declares prune-links dirs).
type PruneLinksOperationReady struct {
	OperationReady
	Dirs []string
}

func (o *PruneLinksOperationReady) Name() string   { return "prune-links" }
func (o *PruneLinksOperationReady) Selected() bool { return len(o.Dirs) > 0 }
func (o *PruneLinksOperationReady) execOperation(p *ProfileReady) error {
	return p.pruneBrokenLinks()
}

// MakeDirsOperationReady creates the profile's dirs (ancestors + makeDirs entries).
type MakeDirsOperationReady struct {
	OperationReady
	Dirs []spec.FileItem
}

func (o *MakeDirsOperationReady) Name() string                        { return "make-dirs" }
func (o *MakeDirsOperationReady) Selected() bool                      { return len(o.Dirs) > 0 }
func (o *MakeDirsOperationReady) execOperation(p *ProfileReady) error { return p.makeDirs(o.Dirs) }

// MakeLinksOperationReady symlinks configs into the system root.
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

// MakeCopiesOperationReady copies *.ontoHost.cp sources onto their dests.
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

// RenderTemplatesOperationReady renders *.tpl sources; each dest path decides
// the target (relative: repo, ~/ or absolute: host).
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

// RunScriptsOperationReady runs the profile's scripts, absolute paths in run order.
type RunScriptsOperationReady struct {
	OperationReady
	Scripts []string
}

func (o *RunScriptsOperationReady) Name() string   { return "run-scripts" }
func (o *RunScriptsOperationReady) Selected() bool { return len(o.Scripts) > 0 }
func (o *RunScriptsOperationReady) execOperation(p *ProfileReady) error {
	return p.runScripts(o.Scripts)
}

// BootoutOperationReady unloads each service (bootout if loaded, wait until gone).
type BootoutOperationReady struct {
	OperationReady
	Services []Service
}

func (o *BootoutOperationReady) Name() string                        { return "services bootout" }
func (o *BootoutOperationReady) Selected() bool                      { return len(o.Services) > 0 }
func (o *BootoutOperationReady) execOperation(p *ProfileReady) error { return p.bootout(o.Services) }

// BootinOperationReady loads each service (bootstrap from plist).
type BootinOperationReady struct {
	OperationReady
	Services []Service
}

func (o *BootinOperationReady) Name() string                        { return "services bootin" }
func (o *BootinOperationReady) Selected() bool                      { return len(o.Services) > 0 }
func (o *BootinOperationReady) execOperation(p *ProfileReady) error { return p.bootin(o.Services) }

// EnsureOperationReady settles then verifies each long-running service has a
// live pid.
type EnsureOperationReady struct {
	OperationReady
	Services []Service
}

func (o *EnsureOperationReady) Name() string                        { return "services ensure" }
func (o *EnsureOperationReady) Selected() bool                      { return len(o.Services) > 0 }
func (o *EnsureOperationReady) execOperation(p *ProfileReady) error { return p.ensure(o.Services) }

// [<] 🤖🤖
