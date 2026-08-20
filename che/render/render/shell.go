package render

// [>] 🤖🤖

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

var shellCallPattern = regexp.MustCompile(`\{\{-?\s*shell\b`)

var nonInteractiveShells = []string{"nologin", "false"}

// IsShellCallPresent reports whether a template body invokes the shell function.
func IsShellCallPresent(body []byte) bool {
	return shellCallPattern.Match(body)
}

func newShellFunc(ctx context.Context, cwd string) func(string) (string, error) {
	return func(command string) (string, error) {
		cmd := exec.CommandContext(ctx, userShell(), "-c", command)
		cmd.Dir = cwd
		cmd.Env = os.Environ()
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("shell %q: %w: %s", command, err, strings.TrimSpace(stderr.String()))
		}
		return strings.TrimRight(stdout.String(), "\n"), nil
	}
}

func userShell() string {
	if s := os.Getenv("SHELL"); isUsableShell(s) {
		return s
	}
	if s := loginShell(); isUsableShell(s) {
		return s
	}
	return "sh"
}

func isUsableShell(path string) bool {
	if path == "" || slices.Contains(nonInteractiveShells, filepath.Base(path)) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode()&0o111 != 0
}

func loginShell() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return darwinLoginShell(u.Username)
	default:
		return passwdLoginShell(u.Uid)
	}
}

func darwinLoginShell(username string) string {
	out, err := exec.Command("dscl", ".", "-read", "/Users/"+username, "UserShell").Output()
	if err != nil {
		return ""
	}
	_, shell, _ := strings.Cut(strings.TrimSpace(string(out)), ": ")
	return shell
}

func passwdLoginShell(uid string) string {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) >= 7 && fields[2] == uid {
			return fields[6]
		}
	}
	if scanner.Err() != nil {
		return ""
	}
	return ""
}

// [<] 🤖🤖
