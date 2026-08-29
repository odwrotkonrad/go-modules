package cli

// [>] 🤖🤖

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/render/checkcmd"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func makeRenderTools() []checkcmd.Tool {
	return []checkcmd.Tool{
		{
			Name:    "tpl",
			Short:   "render a template with the shared engine, to stdout",
			Usage:   renderTplUsage,
			FlagArg: "-f",
			Generate: func(path string) (string, error) {
				src, err := os.ReadFile(path)
				if err != nil {
					return "", &yamlcfg.CodedError{Code: yamlcfg.CodeFileNotFound, Msg: "file not found: " + path}
				}
				cwd, err := os.Getwd()
				if err != nil {
					return "", err
				}
				out, err := render.Exec(path, src, cwd)
				return string(out), err
			},
		},
		{
			Name:     "dirs-tree",
			Short:    "print the cwd repo's tracked-file directory tree",
			Usage:    renderDirsTreeUsage,
			Label:    "dirs-tree",
			CheckArg: ".",
			Generate: func(string) (string, error) { return render.DirsTree(".") },
		},
		{
			Name:     "makefile-doc",
			Short:    "emit makefile.agents.md from a Makefile's [genai-include] sections",
			Usage:    renderMakefileDocUsage,
			Label:    "generated",
			NeedsArg: true,
			CheckArg: "Makefile",
			Generate: lib.Generate,
		},
		{
			Name:     "repo-group-index",
			Short:    "print the repo-group index for a subgroup dir",
			Usage:    renderRepoGroupIndexUsage,
			Label:    "repo-group-index",
			NeedsArg: true,
			CheckArg: ".",
			Generate: render.RepoGroupIndexDir,
		},
	}
}

func (a *app) makeRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "render templates and generated docs with the shared engine",
		Long: `render drives che's shared render engine: gomplate built-ins, shell command
output, remoteFile cross-repo inclusion, frontmatter and markdown transforms.
Under a mergeUpsert dest a missing key is written and an existing key kept unless
the dest's mergeUpdate (none | dependencies | shell | all) or a mark says otherwise:
"| alwaysUpdate" overwrites, "| keepIfExisting" keeps, "| dependency" tags the key
for mergeUpdate dependencies, a shell value runs only when its key is written.
Piping a whole multi-line block through one mark marks every KEY=VALUE line in it.

Every subcommand writes to stdout and reads paths relative to the cwd. The
generating subcommands take --check <file> instead, regenerating and diffing
against <file>: exit 0 match, 22 differ (unified diff on stderr).`,
	}
	for _, tool := range makeRenderTools() {
		cmd.AddCommand(makeRenderToolCmd(tool))
	}
	return cmd
}

// [why] render is a pure filter: no che repo, spec resolution or telemetry, so it
//
//	overrides the root's PersistentPreRunE instead of running app.init
func makeRenderToolCmd(tool checkcmd.Tool) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   tool.Name,
		Short:                 tool.Short,
		Long:                  tool.Usage,
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		PersistentPreRunE:     func(*cobra.Command, []string) error { return nil },
		PersistentPostRunE:    func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			if out, done := renderHelp(cmd, args); done {
				_, err := cmd.OutOrStdout().Write([]byte(out))
				return err
			}
			out, err := tool.Run(args)
			if err != nil {
				return &renderError{err}
			}
			_, err = cmd.OutOrStdout().Write([]byte(out))
			return err
		},
	}
	return cmd
}

// [why] render errors carry the standalone binaries' exit codes and bare messages:
//
//	main prints them unprefixed so --check diffs stay pipeable
type renderError struct{ err error }

func (e *renderError) Error() string { return e.err.Error() }
func (e *renderError) Unwrap() error { return e.err }

func CodedExit(err error) (int, bool) {
	if re, ok := errors.AsType[*renderError](err); ok {
		return yamlcfg.ExitCode(re.err), true
	}
	return 0, false
}

func renderHelp(cmd *cobra.Command, args []string) (string, bool) {
	if len(args) != 1 {
		return "", false
	}
	switch args[0] {
	case "-h", "--help":
		return cmd.Long, true
	}
	return "", false
}

// [<] 🤖🤖
