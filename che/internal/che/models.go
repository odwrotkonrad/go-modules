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

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
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
//	            UNRESOLVED subjects, its Ready the RESOLVED (host-bound) ones;
//	            Host is built at call time and passed in, never stored
//
// cli holds opts (options.Options) and the root *SpecReady as separately
// initialized values: PrepareOptions, then PrepareSpecs.

// NewHost builds each profile's Host; tests override it to inject a mock fs.
var NewHost = host.New

// [<] 🤖🤖

// [>] 🤖🤖 package-level funcs

// PrepareOptions finalizes the runtime options: chdir (-C), locate the repo,
// then resolve with precedence flags > env vars > local che.yml options: >
// defaults.
func PrepareOptions(flags options.Options) (options.Options, error) {
	c := flags
	c.Dir = cmp.Or(c.Dir, os.Getenv("CHE_DIR"))
	if c.Dir != "" {
		if err := os.Chdir(c.Dir); err != nil {
			return c, fmt.Errorf("-C: %w", err)
		}
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return c, err
	}
	if err := c.Resolve(specLayer(filepath.Join(repoRoot, "che.yml"))); err != nil {
		return c, err
	}
	log.SetDebug(c.Debug)
	return c, nil
}

// specLayer leniently reads the local spec's options: block ([why] parse
// errors surface later, at PrepareSpecs).
func specLayer(path string) options.SpecLayer {
	b, err := os.ReadFile(path)
	if err != nil {
		return options.SpecLayer{}
	}
	var d struct {
		Options spec.Options `yaml:"options"`
	}
	if err := yaml.Unmarshal(b, &d); err != nil {
		return options.SpecLayer{}
	}
	return options.SpecLayer{ValidateSpec: d.Options.ValidateSpec, Debug: d.Options.Debug}
}

// PrepareSpecs resolves the root spec and its whole Include tree (top-level
// include.sources + sourced include.profiles refs), fully recursive,
// cycle-guarded, deduped by source URI + profile name.
func PrepareSpecs(opts options.Options, src spec.SpecSourceRecipe) (*SpecReady, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	home, err := invokingHome()
	if err != nil {
		return nil, err
	}
	p := &specsPrep{
		opts:      opts,
		home:      home,
		eval:      spec.NewEvaluator().EvalExecIf,
		seenSpecs: map[string]bool{},
		seenRefs:  map[string]bool{},
	}
	return p.prepare(src, repoRoot, nil, true)
}

// specsPrep threads the shared composition state through the recursion.
type specsPrep struct {
	opts      options.Options
	home      string
	eval      func(expr string) (bool, error)
	seenSpecs map[string]bool // resolved spec dirs (include.sources cycle/dup guard)
	seenRefs  map[string]bool // <uri>::<profile> (sourced-ref dedup)
}

// prepare runs one spec through the full pipeline. forced pins the spec to a
// single referenced profile (sourced include.profiles ref); root marks the
// user-invoked spec (zero eligible profiles is fatal there only). Returns nil
// for an include.sources duplicate.
func (p *specsPrep) prepare(src spec.SpecSourceRecipe, anchor string, forced *spec.ProfileSourceRecipe, root bool) (*SpecReady, error) {
	rcp := &SpecRecipe{Source: src}
	if err := rcp.PrepareSpec(anchor, p.home); err != nil {
		return nil, err
	}
	if forced != nil {
		// [why] keyed on the resolved dir, not the raw URI: the same spec is
		// referenced under different relative URIs across hops.
		key := rcp.sourceReady.DirectoryPath + "::" + forced.ProfileName
		if p.seenRefs[key] {
			log.Debug("source("+forced.ProfileName+")", "skip duplicate "+forced.String(), log.Off)
			return nil, nil
		}
		p.seenRefs[key] = true
	} else if !root && p.seenSpecs[rcp.sourceReady.DirectoryPath] {
		log.Debug("source(spec)", "skip duplicate "+rcp.sourceReady.DirectoryPath, log.Off)
		return nil, nil
	}
	p.seenSpecs[rcp.sourceReady.DirectoryPath] = true
	if err := rcp.PrepareProfileRecipes(p.opts); err != nil {
		return nil, err
	}
	return rcp.PrepareProfiles(p, forced, root)
}

// findRepoRoot: git toplevel of cwd, che.yml must live there (che's defining marker).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.RepoRoot(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "che.yml")); err != nil {
		return "", fmt.Errorf("che.yml not found at repo root %s", root)
	}
	return root, nil
}

// invokingHome resolves the invoking user's home. Under sudo (EUID 0,
// SUDO_USER set), looks up that user's home from passwd so dest paths derive
// from the real user, not /var/root. Otherwise uses $HOME.
func invokingHome() (string, error) {
	if os.Geteuid() == 0 {
		if name := os.Getenv("SUDO_USER"); name != "" {
			h, err := fsutil.UserHome(name)
			if err != nil {
				return "", fmt.Errorf("lookup SUDO_USER %q: %w", name, err)
			}
			return h, nil
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME must be set")
	}
	return home, nil
}

// withEnv runs fn with env exported (host values shadowed), restoring after.
func withEnv(env map[string]string, fn func() error) error {
	if len(env) == 0 {
		return fn()
	}
	for _, k := range slices.Sorted(maps.Keys(env)) {
		prev, had := os.LookupEnv(k)
		if err := os.Setenv(k, env[k]); err != nil {
			return err
		}
		defer func() {
			if had {
				os.Setenv(k, prev)
			} else {
				os.Unsetenv(k)
			}
		}()
	}
	return fn()
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
	if err := r.validateSchema(opts.ValidateSpec); err != nil {
		return err
	}
	doc, err := spec.Load(r.sourceReady.DefinitionURI)
	if err != nil {
		return err
	}
	r.Options, r.Env, r.Include = doc.Options, doc.Env, doc.Include
	// [why] che-level (flag/env) seeds the spec default: profile > spec > che.
	if r.Options.WorkingDirectory == "" {
		r.Options.WorkingDirectory = opts.WorkingDirectory
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

// workingDir resolves a profile's effective options.workingDirectory onto the
// checkout anchor: empty -> the checkout itself, else env-expanded (~/, $VAR,
// env vars), relative -> under the checkout. Must be an existing dir.
func workingDir(anchor, directory string) (string, error) {
	if directory == "" {
		directory = spec.DefaultWorkingDir
	}
	dir := fsutil.ExpandHome(os.ExpandEnv(directory), os.Getenv("HOME"))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(anchor, dir)
	}
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("options.workingDirectory not found: %s (from %q)", dir, directory)
	}
	return dir, nil
}

func (r *SpecRecipe) validateSchema(mode options.ValidateSpecMode) error {
	b, err := os.ReadFile(r.sourceReady.DefinitionURI)
	if err != nil {
		return fmt.Errorf("spec not found: %s", r.sourceReady.DefinitionURI)
	}
	finds := spec.ValidateSchema(b)
	if len(finds) == 0 {
		return nil
	}
	if mode == options.ValidateSpec.Error {
		return fmt.Errorf("schema violations in %s:\n%s", r.sourceReady.DefinitionURI, strings.Join(finds, "\n"))
	}
	for _, f := range finds {
		log.Msg("validate(che.yml)", f, log.Off)
	}
	return nil
}

// PrepareProfiles assembles the SpecReady: spec-level execIf gate, top-level
// include.sources composed (their recipes joining the local-ref lookup),
// per-recipe eligibility, one MakeProfile per eligible recipe, sourced refs
// spawning nested SpecReadys (recursive).
func (r *SpecRecipe) PrepareProfiles(p *specsPrep, forced *spec.ProfileSourceRecipe, root bool) (*SpecReady, error) {
	ready := &SpecReady{Source: r.sourceReady, Options: r.Options, Env: r.Env, recipes: r.ProfileRecipes}
	err := withEnv(r.Env, func() error {
		pass, err := spec.AllPass("spec "+r.sourceReady.DefinitionURI, r.Options.ExecIf, p.opts.SkipExecIf, p.eval)
		if err != nil {
			return err
		}
		if !pass {
			log.Msg("spec(skip)", r.sourceReady.DefinitionURI+" (execIf failed)", log.Off)
			return nil
		}
		lookup, err := r.composeIncludes(p, ready)
		if err != nil {
			return err
		}
		names, err := r.eligibleNames(p, forced, root)
		if err != nil || len(names) == 0 {
			return err
		}
		return r.assembleProfiles(p, ready, lookup, names, forced)
	})
	if err != nil {
		return nil, err
	}
	return ready, nil
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

// eligibleNames picks this spec's own profiles to assemble: the forced ref's
// one profile (execIf gated inside its env overlay, skip on fail), or
// EligibleRecipes (zero eligible fatal only at the root spec).
func (r *SpecRecipe) eligibleNames(p *specsPrep, forced *spec.ProfileSourceRecipe, root bool) ([]string, error) {
	if forced != nil {
		rec, err := spec.FindRecipe(r.ProfileRecipes, forced.ProfileName)
		if err != nil {
			return nil, fmt.Errorf("ref %s: %w", forced, err)
		}
		var pass bool
		err = withEnv(forced.Env, func() error {
			var e error
			pass, e = spec.AllPass(forced.ProfileName, rec.Options.OverRef(forced.Options).ExecIf, p.opts.SkipExecIf, p.eval)
			return e
		})
		if err != nil {
			return nil, fmt.Errorf("ref %s: %w", forced, err)
		}
		if !pass {
			log.Msg("source("+forced.ProfileName+")", fmt.Sprintf("skip %s (execIf failed)", forced), log.Off)
			return nil, nil
		}
		return []string{forced.ProfileName}, nil
	}
	forceOne := ""
	if root {
		forceOne = p.opts.Profile
	}
	names, err := spec.EligibleRecipes(r.ProfileRecipes, forceOne, p.opts.SkipExecIf, p.eval)
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
// emits the operation recipes, a Host anchored at the recipe's directory
// prepares them (subjects resolved), the env overlay wraps preparation.
func (r *SpecRecipe) makeProfileReady(p *specsPrep, rec spec.ProfileRecipe, lookup []spec.ProfileRecipe, env map[string]string) (*ProfileReady, []spec.ProfileSourceRecipe, error) {
	name := rec.Source.GetProfileName()
	var pr *ProfileReady
	var refs []spec.ProfileSourceRecipe
	err := withEnv(env, func() error {
		wd, err := workingDir(rec.Source.DirectoryPath, rec.Options.WorkingDirectory)
		if err != nil {
			return err
		}
		ops, sourced, err := rec.MakeProfile(lookup, wd)
		if err != nil {
			return err
		}
		refs = sourced
		pr = &ProfileReady{
			Source: spec.ProfileSourceReady{
				SourceReady: spec.SourceReady{DefinitionURI: r.sourceReady.DefinitionURI, DirectoryPath: rec.Source.DirectoryPath},
				ProfileName: name,
			},
			Options:    rec.Options,
			Env:        env,
			Profiles:   sourced,
			ref:        rec.Source.String(),
			workingDir: wd,
			opts:       p.opts,
			home:       p.home,
		}
		pr.OperationsReady, err = prepareOperations(ops, pr.newHost(), p.opts)
		return err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q: %w", name, err)
	}
	return pr, refs, nil
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

// ProfileReady is one resolved profile, ready to install onto the OS. Host is
// built at execution time, anchored at Source.DirectoryPath, and passed per
// call — never stored.
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
}

// Ref is the profile's display ref (report lines, detect).
func (p *ProfileReady) Ref() string { return p.ref }

// newHost builds the op executor anchored at the profile's checkout, its
// load-ops tree at the resolved working dir. Sourced profiles carry a profile=
// log subtype ([why] disambiguates interleaved runs).
func (p *ProfileReady) newHost() host.Host {
	h := NewHost(p.Source.DirectoryPath, p.workingDir, p.home, p.Source.GetProfileName(), p.opts)
	if p.ref != p.Source.GetProfileName() {
		h = h.WithLogSub("profile=" + p.Source.GetProfileName())
	}
	return h
}

// withEnv runs fn inside the profile's env overlay and effective debug level.
func (p *ProfileReady) withEnv(fn func() error) error {
	debug := p.opts.Debug
	if p.Options.Debug != nil {
		debug = *p.Options.Debug
	}
	log.SetDebug(debug)
	defer log.SetDebug(p.opts.Debug)
	return withEnv(p.Env, fn)
}

// ExecOperations executes ALL of the profile's operations, in run order:
// Selected() gated (all(skip) debug line), errors join, a failing op does not
// stop the rest.
func (p *ProfileReady) ExecOperations() error {
	return p.withEnv(func() error {
		h := p.newHost()
		var fails []error
		for _, op := range p.OperationsReady {
			if !op.Selected() {
				log.Debug("all(skip)", op.Name()+" (nothing selected)", log.Off)
				continue
			}
			log.Msg("all(run)", op.Name(), log.Off)
			if err := op.execOperation(h); err != nil {
				fails = append(fails, fmt.Errorf("%s: %w", op.Name(), err))
			}
		}
		return errors.Join(fails...)
	})
}

// ExecOperation executes one prepared operation (per-op subcommands): same
// env overlay and Selected() gating.
func (p *ProfileReady) ExecOperation(op operationReady) error {
	return p.withEnv(func() error {
		if !op.Selected() {
			log.Debug("all(skip)", op.Name()+" (nothing selected)", log.Off)
			return nil
		}
		return op.execOperation(p.newHost())
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
	err := p.withEnv(func() error {
		for _, op := range p.OperationsReady {
			rs, ok := op.(*RunScriptsOperationReady)
			if !ok {
				continue
			}
			scripts := filterScripts(rs.Scripts, names)
			matched = len(scripts)
			return p.newHost().RunScripts(scripts)
		}
		return nil
	})
	return matched, err
}

func filterScripts(scripts, names []string) []string {
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
// NO Host field — the Host is passed in at execution time.
type OperationReady struct{}

// operationReady is the one interface the heterogeneous ordered run list
// forces: minimal, unexported.
type operationReady interface {
	execOperation(h host.Host) error
	Selected() bool
	Name() string
}

// prepareOperations resolves each operation recipe's subjects against h,
// returning the prepared operations in run order.
func prepareOperations(ops spec.OperationRecipes, h host.Host, opts options.Options) ([]operationReady, error) {
	scripts, err := h.ResolveScripts(ops.RunScripts.Scripts)
	if err != nil {
		return nil, err
	}
	services, err := h.ResolveServices(ops.Services.Services)
	if err != nil {
		return nil, err
	}
	return []operationReady{
		&PruneLinksOperationReady{Dirs: ops.PruneLinks.Dirs},
		&MkDirsOperationReady{Dirs: ops.MkDirs.Dirs},
		&LinkOperationReady{Links: ops.Link.Links, Dirs: ops.Link.Dirs},
		&CopyOperationReady{Copies: ops.Copy.Copies, Dirs: ops.Copy.Dirs},
		&RenderTemplatesOperationReady{Templates: ops.RenderTemplates.Templates, SkipSecrets: opts.RenderSkipSecrets},
		&RunScriptsOperationReady{Scripts: scripts},
		&BootoutOperationReady{Services: services},
		&BootinOperationReady{Services: services},
		&EnsureOperationReady{Services: services},
	}, nil
}

// [<] 🤖🤖

// [>] 🤖🤖 per-kind operations

// PruneLinksOperationReady deletes broken symlinks under the profile's dirs.
type PruneLinksOperationReady struct {
	OperationReady
	Dirs []string
}

func (o *PruneLinksOperationReady) Name() string   { return "prune-links" }
func (o *PruneLinksOperationReady) Selected() bool { return len(o.Dirs) > 0 }
func (o *PruneLinksOperationReady) execOperation(h host.Host) error {
	return h.PruneBrokenLinks(o.Dirs)
}

// MkDirsOperationReady creates the profile's dirs (ancestors + mkdirs entries).
type MkDirsOperationReady struct {
	OperationReady
	Dirs []spec.FileItem
}

func (o *MkDirsOperationReady) Name() string                    { return "mk-dirs" }
func (o *MkDirsOperationReady) Selected() bool                  { return len(o.Dirs) > 0 }
func (o *MkDirsOperationReady) execOperation(h host.Host) error { return h.MkDirs(o.Dirs) }

// LinkOperationReady symlinks configs into the system root.
type LinkOperationReady struct {
	OperationReady
	Links []spec.FileItem
	Dirs  []string
}

func (o *LinkOperationReady) Name() string                    { return "link" }
func (o *LinkOperationReady) Selected() bool                  { return len(o.Links) > 0 }
func (o *LinkOperationReady) execOperation(h host.Host) error { return h.MkLinks(o.Links, o.Dirs) }

// CopyOperationReady copies *.ontoHost.cp sources onto their dests.
type CopyOperationReady struct {
	OperationReady
	Copies []spec.FileItem
	Dirs   []string
}

func (o *CopyOperationReady) Name() string                    { return "copy" }
func (o *CopyOperationReady) Selected() bool                  { return len(o.Copies) > 0 }
func (o *CopyOperationReady) execOperation(h host.Host) error { return h.MkCopies(o.Copies, o.Dirs) }

// RenderTemplatesOperationReady renders *.tpl sources; each dest path decides
// the target (relative: repo, ~/ or absolute: host).
type RenderTemplatesOperationReady struct {
	OperationReady
	Templates   []spec.FileItem
	SkipSecrets bool
}

func (o *RenderTemplatesOperationReady) Name() string   { return "render-templates" }
func (o *RenderTemplatesOperationReady) Selected() bool { return len(o.Templates) > 0 }
func (o *RenderTemplatesOperationReady) execOperation(h host.Host) error {
	return h.RenderTemplates(o.Templates, o.SkipSecrets)
}

// RunScriptsOperationReady runs the profile's scripts, absolute paths in run order.
type RunScriptsOperationReady struct {
	OperationReady
	Scripts []string
}

func (o *RunScriptsOperationReady) Name() string                    { return "run-scripts" }
func (o *RunScriptsOperationReady) Selected() bool                  { return len(o.Scripts) > 0 }
func (o *RunScriptsOperationReady) execOperation(h host.Host) error { return h.RunScripts(o.Scripts) }

// BootoutOperationReady unloads each service (bootout if loaded, wait until gone).
type BootoutOperationReady struct {
	OperationReady
	Services []host.Service
}

func (o *BootoutOperationReady) Name() string                    { return "services bootout" }
func (o *BootoutOperationReady) Selected() bool                  { return len(o.Services) > 0 }
func (o *BootoutOperationReady) execOperation(h host.Host) error { return h.Bootout(o.Services) }

// BootinOperationReady loads each service (bootstrap from plist).
type BootinOperationReady struct {
	OperationReady
	Services []host.Service
}

func (o *BootinOperationReady) Name() string                    { return "services bootin" }
func (o *BootinOperationReady) Selected() bool                  { return len(o.Services) > 0 }
func (o *BootinOperationReady) execOperation(h host.Host) error { return h.Bootin(o.Services) }

// EnsureOperationReady settles then verifies each long-running service has a
// live pid.
type EnsureOperationReady struct {
	OperationReady
	Services []host.Service
}

func (o *EnsureOperationReady) Name() string                    { return "services ensure" }
func (o *EnsureOperationReady) Selected() bool                  { return len(o.Services) > 0 }
func (o *EnsureOperationReady) execOperation(h host.Host) error { return h.Ensure(o.Services) }

// [<] 🤖🤖
