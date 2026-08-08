package climain

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func Run(name, version, usage string, run func(args []string) (string, error)) {
	RunNamed(name, version, usage, func(args []string) (string, error) {
		out, err := run(args)
		return out + "\n", err
	})
}

func RunNamed(name, version, usage string, run func(args []string) (string, error)) {
	usage = strings.TrimSuffix(usage, "\n")
	if out, done := HelpVersion(os.Args[1:], usage, name, resolveVersion(version)); done {
		Exit(out, nil)
	}
	RunRaw(run)
}

func RunRaw(run func(args []string) (string, error)) {
	out, err := run(os.Args[1:])
	if err == nil {
		fmt.Print(out)
		os.Exit(0)
	}
	Exit("", err)
}

func resolveVersion(version string) string {
	if version != "" && version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return version
}

func HelpVersion(args []string, usage, name, version string) (out string, done bool) {
	if len(args) != 1 {
		return "", false
	}
	switch args[0] {
	case "-h", "--help":
		return usage, true
	case "-v", "--version":
		return name + " version " + version, true
	}
	return "", false
}

func Exit(out string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	} else {
		fmt.Println(out)
	}
	os.Exit(yamlcfg.ExitCode(err))
}

//[<] 🤖🤖
