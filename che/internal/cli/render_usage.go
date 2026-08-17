package cli

// [>] 🤖🤖

const renderTplUsage = `usage: che render tpl -f <template>

Render <template> with the shared engine (gomplate built-ins + op:// (1Password)
and gcp:// (GCP Secret Manager) secrets + frontmatter/readBody + native
generators), env vars visible via env.Getenv, to
stdout. Drop-in for 'gomplate -f'. Paths in frontmatter/readBody/renderDirsTree
resolve against the cwd.
`

const renderDirsTreeUsage = `usage: che render dirs-tree
       che render dirs-tree --check <file>

Print the plain directory tree of the cwd repo's tracked files (stdout):
read tracked paths from the git index, drop each file leaf, nest and sort
the remaining dirs, 2-space indented, one dir per line.
--check regenerates and diffs against <file>:
exit 0 match, 22 differ (unified diff on stderr).
`

const renderMakefileDocUsage = `usage: che render makefile-doc <makefile-path>
       che render makefile-doc --check <doc-file>

Emit makefile.agents.md from a Makefile's [genai-include] sections (stdout).
--check regenerates from ./Makefile and diffs against <doc-file>:
exit 0 match, 22 differ (unified diff on stderr).
`

const renderRepoGroupIndexUsage = `usage: che render repo-group-index <subgroup-dir>
       che render repo-group-index --check <file>

Print the repo-group index for <subgroup-dir> (stdout): a # Repositories
section (where you are, the group's directory structure tree, then each direct
child repo (a dir with .git) as ## Repo: ./<rel-path> with its
assets/docs-agents/purpose.md body inlined, or a placeholder when missing),
then each child subgroup as ## Subgroup: ./<rel-path> with its repos inlined
recursively (purposes only, no repeated tree or section headings).
--check regenerates for <file>'s dir and diffs against <file>:
exit 0 match, 22 differ (unified diff on stderr).
`

// [<] 🤖🤖
