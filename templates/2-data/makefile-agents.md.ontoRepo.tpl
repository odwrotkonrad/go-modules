{{- renderMakefileDoc "Makefile" }}
{{ renderMakefileDoc "che/Makefile" | strings.ReplaceAll "## `./Makefile`" "## `./che/Makefile`" }}
{{ renderMakefileDoc "get-os-open-files-with/Makefile" | strings.ReplaceAll "## `./Makefile`" "## `./get-os-open-files-with/Makefile`" }}
{{ renderMakefileDoc "get-term-open-files-with/Makefile" | strings.ReplaceAll "## `./Makefile`" "## `./get-term-open-files-with/Makefile`" }}
{{ renderMakefileDoc "lib/Makefile" | strings.ReplaceAll "## `./Makefile`" "## `./lib/Makefile`" -}}
