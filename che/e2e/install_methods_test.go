package e2e

// [>] 🤖🤖

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

// [why] installs real packages into a throwaway HOME (with_deps mode) or a fresh
//
//	docker container (with_no_deps mode), then runs each one: the only coverage
//	proving a method works end to end against live sources
//	E2E_INSTALL_PACKAGE=all (default)|<pkg>|<pkg1>,<pkg2> picks packages, one
//	subtest per entry method; E2E_INSTALL_METHOD narrows methods (exact or
//	<method>/ prefix); E2E_INSTALL_PACKAGES_PER_METHOD caps packages per method
type installJob struct {
	packages    []string
	onlyMethods []string
	verify      map[string]verifyCmd
	warnMissing bool
}

type verifyCmd struct {
	cmd     string
	wantOut bool
}

type runCfg struct {
	mode      string
	linuxArch string
	darwinVM  bool
}

func resolveRunCfg(platform, mode, hostOS, hostArch string, virt, allowHost bool) (runCfg, error) {
	hostRun := func() (runCfg, error) {
		if !virt && !allowHost {
			return runCfg{}, fmt.Errorf("installs never run on the bare-metal host: invoke via make e2e-install-methods (auto-delegates darwin runs into a tart VM), or set E2E_ALLOW_HOST=1")
		}
		return runCfg{mode: "with_deps"}, nil
	}
	m := cmp.Or(mode, "with_no_deps")
	if m != "with_deps" && m != "with_no_deps" {
		return runCfg{}, fmt.Errorf("E2E_INSTALL_MODE %q: want with_deps or with_no_deps", mode)
	}
	switch platform {
	case "":
		if m == "with_deps" {
			return hostRun()
		}
		if hostOS == "darwin" {
			return runCfg{mode: m, darwinVM: true}, nil
		}
		return runCfg{mode: m}, nil
	case "darwin-arm64":
		if hostOS != "darwin" || hostArch != "arm64" {
			return runCfg{}, fmt.Errorf("platform darwin-arm64 needs a darwin-arm64 host, this host is %s-%s", hostOS, hostArch)
		}
		if m == "with_deps" {
			return hostRun()
		}
		return runCfg{mode: m, darwinVM: true}, nil
	case "linux-arm64", "linux-amd64":
		if mode != "" && mode != "with_no_deps" {
			return runCfg{}, fmt.Errorf("E2E_INSTALL_MODE %q conflicts with platform %s (implies with_no_deps)", mode, platform)
		}
		arch := strings.TrimPrefix(platform, "linux-")
		if hostOS == "linux" && arch != hostArch {
			return runCfg{}, fmt.Errorf("platform %s: only the host platform (linux-%s) is testable on linux hosts", platform, hostArch)
		}
		return runCfg{mode: "with_no_deps", linuxArch: arch}, nil
	}
	return runCfg{}, fmt.Errorf("E2E_INSTALL_PLATFORM %q: want darwin-arm64, linux-arm64 or linux-amd64", platform)
}

const noDepsSkipMarker = "CHE_E2E_SKIP_NO_APPLICABLE_MANAGER"

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func noApplicableManager(out string, pkgs []string) bool {
	stripped := ansiRe.ReplaceAllString(out, "")
	for _, pkg := range pkgs {
		if !strings.Contains(stripped, "will not install "+pkg+": no applicable installation method") {
			return false
		}
	}
	return true
}

func TestE2EInstallMethods(t *testing.T) {
	if os.Getenv("E2E_BIN") == "" {
		t.Skip("E2E_BIN not set; run via make e2e-install-methods")
	}
	cfg, err := resolveRunCfg(os.Getenv("E2E_INSTALL_PLATFORM"), os.Getenv("E2E_INSTALL_MODE"),
		runtime.GOOS, runtime.GOARCH, fsutil.IsVirtualized(), os.Getenv("E2E_ALLOW_HOST") == "1")
	require.NoError(t, err)
	runPackageMode(t, cmp.Or(os.Getenv("E2E_INSTALL_PACKAGE"), "all"), os.Getenv("E2E_INSTALL_METHOD"), cfg)
}

func methodKey(mgr string) string {
	switch mgr {
	case "brew":
		return "brew/formulae"
	case "cask":
		return "brew/cask"
	}
	return mgr
}

func installerKey(mgr string) string {
	if mgr == "cask" {
		return "brew/cask"
	}
	return mgr
}

func targetHost(cfg runCfg) packages.Host {
	if cfg.mode == "with_no_deps" && !cfg.darwinVM {
		return packages.Host{OS: "linux", Arch: cmp.Or(cfg.linuxArch, runtime.GOARCH), Distro: "debian"}
	}
	if cfg.darwinVM || runtime.GOOS == "darwin" {
		return packages.Host{OS: "darwin", Arch: "arm64"}
	}
	return packages.Host{OS: "linux", Arch: runtime.GOARCH, Distro: "debian"}
}

func eligibleMethods(entry packages.Entry, method string, eligible []string) []string {
	var methods []string
	for _, it := range entry.Items {
		if slices.Contains(methods, it.Mgr) || !slices.Contains(eligible, installerKey(it.Mgr)) || !methodMatches(methodKey(it.Mgr), method) {
			continue
		}
		methods = append(methods, it.Mgr)
	}
	return methods
}

func logLevel() string {
	return cmp.Or(os.Getenv("CHE_LOG_LEVEL"), "info")
}

func methodMatches(key, selected string) bool {
	if selected == "" || selected == "all" {
		return true
	}
	return key == selected || strings.HasPrefix(key, selected+"/")
}

func splitPackages(sel string) []string {
	names := strings.Split(sel, ",")
	for i, name := range names {
		names[i] = strings.TrimSpace(name)
	}
	return slices.DeleteFunc(names, func(name string) bool { return name == "" })
}

func perMethodLimit(t *testing.T) int {
	t.Helper()
	v := os.Getenv("E2E_INSTALL_PACKAGES_PER_METHOD")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	require.NoError(t, err, "E2E_INSTALL_PACKAGES_PER_METHOD %q", v)
	return n
}

func runPackageMode(t *testing.T, sel, method string, cfg runCfg) {
	file, err := packages.LoadBuiltin()
	require.NoError(t, err)
	lenient := sel == "all"
	names := slices.Sorted(maps.Keys(file.Packages))
	if !lenient {
		names = splitPackages(sel)
		require.NotEmpty(t, names, "E2E_INSTALL_PACKAGE %q selects no package", sel)
		for _, name := range names {
			_, ok := file.Packages[name]
			require.True(t, ok, "package %s not in the builtin packages.yml", name)
		}
	}
	limit := perMethodLimit(t)
	used := map[string]int{}
	eligible := file.EligibleInstallers(targetHost(cfg))
	if len(names) == 1 {
		runPackageMethods(t, file.Packages[names[0]], names[0], method, cfg, eligible, lenient, limit, used)
		return
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			runPackageMethods(t, file.Packages[name], name, method, cfg, eligible, lenient, limit, used)
		})
	}
}

func runPackageMethods(t *testing.T, entry packages.Entry, pkg, method string, cfg runCfg, eligible []string, lenient bool, limit int, used map[string]int) {
	methods := eligibleMethods(entry, method, eligible)
	if len(methods) == 0 {
		t.Skipf("package %s has no %s method for the target platform", pkg, cmp.Or(method, "install"))
	}
	for _, m := range methods {
		key := methodKey(m)
		if limit > 0 && used[key] >= limit {
			t.Logf("skip %s for %s: %d packages already ran for the method", key, pkg, limit)
			continue
		}
		used[key]++
		t.Run(key, func(t *testing.T) {
			runJob(t, installJob{
				packages:    []string{pkg},
				onlyMethods: []string{m},
				verify:      map[string]verifyCmd{pkg: resolveVerify(t, entry, pkg, m)},
				warnMissing: lenient || os.Getenv("E2E_INSTALL_MISSING_METHOD") == "warn",
			}, cfg)
		})
	}
}

func resolveVerify(t *testing.T, entry packages.Entry, pkg, mgr string) verifyCmd {
	t.Helper()
	spec := entry.Verify
	for _, it := range entry.Items {
		if it.Mgr == mgr && it.Verify != nil {
			spec = it.Verify
		}
	}
	switch {
	case spec != nil && spec.Cmd != "":
		return verifyCmd{cmd: spec.Cmd}
	case spec != nil && spec.Strategy == packages.VerifyPkgVersionCmd:
		cmd, err := pkgVersionCmd(entry, pkg, mgr)
		require.NoError(t, err)
		return verifyCmd{cmd: cmd, wantOut: true}
	case spec == nil && entry.VersionCommand != "":
		return verifyCmd{cmd: entry.VersionCommand, wantOut: true}
	}
	cmd := pkg
	if entry.Command != "" {
		cmd = entry.Command
	}
	return verifyCmd{cmd: fmt.Sprintf("%s --version 2>/dev/null || %s version", cmd, cmd), wantOut: true}
}

func pkgVersionCmd(entry packages.Entry, pkg, mgr string) (string, error) {
	name := pkg
	for _, it := range entry.Items {
		if it.Mgr != mgr {
			continue
		}
		if it.Apt != nil && it.Apt.PackageName != "" {
			name = it.Apt.PackageName
		} else if it.Name != "" {
			name = it.Name
		}
	}
	switch mgr {
	case "apt":
		return fmt.Sprintf("dpkg-query -W -f '${Version}\\n' %s", name), nil
	case "brew":
		if entry.Version != "" {
			return fmt.Sprintf("brew list --versions %s@%s || brew list --versions %s", name, entry.Version, name), nil
		}
		return "brew list --versions " + name, nil
	case "cask":
		return "brew list --cask --versions " + name, nil
	case "npm":
		return "npm ls --global " + name, nil
	case "vscode":
		return fmt.Sprintf(`code $([ "$(id -u)" = 0 ] && echo "--no-sandbox --user-data-dir $HOME/.vscode-root") --list-extensions --show-versions | grep -i '^%s@'`, name), nil
	}
	return "", fmt.Errorf("verify pkgVersionCmd: no version query for method %s", mgr)
}

func runJob(t *testing.T, job installJob, cfg runCfg) {
	t.Helper()
	var out string
	var skipped bool
	switch {
	case cfg.darwinVM:
		out, skipped = runJobVM(t, job)
	case cfg.mode == "with_no_deps":
		out, skipped = runJobNoDeps(t, job, cfg)
	default:
		out, skipped = runJobHost(t, job)
	}
	if skipped {
		t.Skipf("no applicable installation method for %s here", strings.Join(job.packages, ", "))
	}
	// [why] a host copy already on PATH makes che skip: the run would then verify the system
	//   binary and prove nothing about the method under test; in all-packages mode a
	//   preinstalled package is an environment outcome, not a failure
	stripped := ansiRe.ReplaceAllString(out, "")
	for _, pkg := range job.packages {
		if regexp.MustCompile(`(?m)installed ` + regexp.QuoteMeta(pkg) + `( \S+)? via `).MatchString(stripped) {
			continue
		}
		if !strings.Contains(stripped, "will not install "+pkg+":") &&
			!strings.Contains(stripped, pkg+": ✅ already") {
			continue
		}
		t.Skipf("%s was already present here, the method never ran", pkg)
	}
}

func installArgv(job installJob) []string {
	argv := []string{"packages", "install", "--packages-file", packages.BuiltinSentinel}
	if job.warnMissing {
		argv = append(argv, "--missing-method-warn")
	}
	if len(job.onlyMethods) > 0 {
		argv = append(argv, "--only-methods", strings.Join(job.onlyMethods, ","))
	}
	return append(argv, job.packages...)
}

func hostInstallEnv(t *testing.T, home string) []string {
	t.Helper()
	bin := filepath.Join(home, ".local", "bin")
	pyenvRoot := filepath.Join(home, ".pyenv")
	// [why] go install writes to GOBIN, nvm/pyenv to their own roots: every destination a
	//   method may use has to be visible to the verify step
	goBin := filepath.Join(home, "go", "bin")
	dirs := []string{
		bin, goBin,
		filepath.Join(home, ".gem", "bin"),
		filepath.Join(pyenvRoot, "bin"), filepath.Join(pyenvRoot, "shims"),
	}
	if runtime.GOOS == "darwin" {
		dirs = append(dirs, "/opt/homebrew/bin", "/opt/homebrew/sbin")
	}
	path := strings.Join(append(dirs, os.Getenv("PATH")), ":")
	for _, d := range []string{".local/bin", ".config", ".cache", ".local/state"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, d), 0o755))
	}
	env := []string{
		"HOME=" + home,
		"PATH=" + path,
		"PYENV_ROOT=" + pyenvRoot,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"XDG_BIN_HOME=" + bin,
		"XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
		"GEM_HOME=" + filepath.Join(home, ".gem"),
		"CHE_E2E=1",
		"CHE_LOG_LEVEL=" + logLevel(),
		"CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH=0",
		"NVM_DIR=" + filepath.Join(home, ".config", "nvm"),
		"GOBIN=" + goBin,
		//[why] GOPATH defaults into the throwaway HOME: without this the module cache's
		//  read-only dirs make t.TempDir cleanup fail the passing test
		"GOFLAGS=-modcacherw",
		"CHE_PACKAGES_DOWNLOAD_CACHE_DIR=" + filepath.Join(devCacheDir(t), "downloads"),
	}
	if v := os.Getenv("SSL_CERT_FILE"); v != "" {
		env = append(env, "SSL_CERT_FILE="+v)
	}
	if dir := os.Getenv("E2E_GOCOVERDIR"); dir != "" {
		env = append(env, "GOCOVERDIR="+dir)
	}
	return env
}

func runJobHost(t *testing.T, job installJob) (string, bool) {
	t.Helper()
	home := t.TempDir()
	env := hostInstallEnv(t, home)

	work := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(work, "che.yml"),
		[]byte("e2e:\n  options: {autoDiscover: true}\n"), 0o644))
	install := exec.Command(binPath(t), installArgv(job)...)
	install.Env = env
	install.Dir = work
	var buf bytes.Buffer
	install.Stdout = io.MultiWriter(&buf, os.Stderr)
	install.Stderr = install.Stdout
	t.Logf("install %s", strings.Join(job.packages, " "))
	err := install.Run()
	out := buf.String()
	require.NoError(t, err, "install %s:\n%s", strings.Join(job.packages, " "), out)
	if noApplicableManager(out, job.packages) {
		return out, true
	}

	for pkg, v := range job.verify {
		verify := exec.Command("/bin/bash", "-ec", v.cmd)
		verify.Env = env
		verify.Dir = home
		vout, verr := verify.CombinedOutput()
		t.Logf("verify %s: %s\n%s", pkg, v.cmd, vout)
		require.NoError(t, verr, "verify %s via %q", pkg, v.cmd)
		if v.wantOut {
			require.NotEmpty(t, strings.TrimSpace(string(vout)), "verify %s produced no output", pkg)
		}
	}
	return out, false
}

func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// [why] apt/download caches persist across runs while each environment stays isolated:
//
//	with_no_deps mounts /var/cache/apt, /var/lib/apt/lists, /root/.cache and the che
//	download cache from here; with_deps points CHE_PACKAGES_DOWNLOAD_CACHE_DIR at downloads/
func devCacheDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("E2E_INSTALL_CACHE_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		base = filepath.Join(home, ".cache", "che", "dev")
	}
	for _, d := range []string{"apt-cache", "apt-lists", "xdg", "downloads"} {
		require.NoError(t, os.MkdirAll(filepath.Join(base, d), 0o755))
	}
	return base
}

func noDepsScript(job installJob) string {
	var b strings.Builder
	b.WriteString(`export HOME=/root
export PATH=/root/.local/bin:/root/go/bin:/root/.pyenv/bin:/root/.pyenv/shims:$PATH
export XDG_CONFIG_HOME=/root/.config XDG_DATA_HOME=/root/.local/share XDG_BIN_HOME=/root/.local/bin
export XDG_STATE_HOME=/root/.local/state XDG_CACHE_HOME=/root/.cache
export PYENV_ROOT=/root/.pyenv NVM_DIR=/root/.config/nvm GOBIN=/root/go/bin GOFLAGS=-modcacherw
`)
	fmt.Fprintf(&b, "export CHE_E2E=1 CHE_LOG_LEVEL=%s CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH=0\n", logLevel())
	b.WriteString(`mkdir -p /root/.local/bin /root/.config /root/.local/state /tmp/work
cd /tmp/work
printf 'e2e:\n  options: {autoDiscover: true}\n' > che.yml
`)
	appendRunVerify(&b, "che", job)
	return b.String()
}

func appendRunVerify(b *strings.Builder, che string, job installJob) {
	b.WriteString("olog=$(mktemp) rcf=$(mktemp)\n")
	fmt.Fprintf(b, "{ %s %s 2>&1; echo $? > \"$rcf\"; } | tee \"$olog\"\n", che, strings.Join(installArgv(job), " "))
	b.WriteString("[ \"$(cat \"$rcf\")\" = 0 ] || exit 1\n")
	b.WriteString("out=$(cat \"$olog\")\n")
	b.WriteString("skip=1\n")
	for _, pkg := range job.packages {
		fmt.Fprintf(b, "case \"$out\" in (*'%s: no applicable installation method'*) ;; (*) skip=0;; esac\n", pkg)
	}
	fmt.Fprintf(b, "if [ \"$skip\" = 1 ]; then echo %s; exit 0; fi\n", noDepsSkipMarker)
	for pkg, v := range job.verify {
		cmd := `if [ -s "$NVM_DIR/nvm.sh" ]; then . "$NVM_DIR/nvm.sh"; fi; ` + v.cmd
		fmt.Fprintf(b, "vout=$(bash -c %s 2>&1) || { printf '%%s\\n' \"$vout\"; exit 1; }\n", shq(cmd))
		fmt.Fprintf(b, "printf 'verify %s: %%s\\n' \"$vout\"\n", pkg)
		if v.wantOut {
			b.WriteString("test -n \"$vout\"\n")
		}
	}
}

func runJobNoDeps(t *testing.T, job installJob, cfg runCfg) (string, bool) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker absent; with_no_deps mode needs it")
	}
	binLinux := os.Getenv("E2E_BIN_LINUX")
	if binLinux == "" {
		t.Skip("E2E_BIN_LINUX not set; run via make e2e-install-methods MODE=with_no_deps")
	}
	require.FileExists(t, binLinux)
	arch := cmp.Or(cfg.linuxArch, runtime.GOARCH)
	image := noDepsImage(t, arch)
	cache := devCacheDir(t)
	argv := []string{
		"run", "--rm", "--quiet", "--platform", "linux/" + arch,
		"-v", binLinux + ":/usr/local/bin/che:ro",
		"-v", filepath.Join(cache, "apt-cache") + ":/var/cache/apt",
		"-v", filepath.Join(cache, "apt-lists") + ":/var/lib/apt/lists",
		"-v", filepath.Join(cache, "xdg") + ":/root/.cache",
		"-v", filepath.Join(cache, "downloads") + ":/che-cache",
		"-e", "CHE_PACKAGES_DOWNLOAD_CACHE_DIR=/che-cache",
		image, "sh", "-ec", noDepsScript(job),
	}
	run := exec.Command("docker", argv...)
	out, err := runStreamed(t, run, "with_no_deps install "+strings.Join(job.packages, " "))
	require.NoError(t, err, "with_no_deps install %s", strings.Join(job.packages, " "))
	return out, strings.Contains(out, noDepsSkipMarker)
}

var noDepsImages sync.Map

func noDepsImage(t *testing.T, arch string) string {
	t.Helper()
	tag := "che-e2e-debian:" + arch
	once, _ := noDepsImages.LoadOrStore(arch, new(sync.Once))
	once.(*sync.Once).Do(func() {
		if exec.Command("docker", "image", "inspect", tag).Run() == nil {
			return
		}
		build := exec.Command("docker", "build", "--quiet", "--platform", "linux/"+arch,
			"--file", "install-methods.Dockerfile", "--tag", tag, ".")
		out, err := build.CombinedOutput()
		require.NoError(t, err, "build %s: %s", tag, out)
	})
	return tag
}

func runStreamed(t *testing.T, cmd *exec.Cmd, label string) (string, error) {
	t.Helper()
	t.Logf("%s", label)
	var buf bytes.Buffer
	w := io.MultiWriter(&buf, os.Stderr)
	cmd.Stdout, cmd.Stderr = w, w
	err := cmd.Run()
	return buf.String(), err
}

const vmTemplate = "che-e2e-install-methods-template"

const vmUser = "user"

var vmSeq atomic.Int64

func sshArgs(ip string, cmdline string) []string {
	return []string{
		"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR",
		vmUser + "@" + ip, cmdline,
	}
}

func tartHasVM(t *testing.T, name string) bool {
	t.Helper()
	out, err := exec.Command("tart", "list").Output()
	require.NoError(t, err, "tart list")
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[1] == name {
			return true
		}
	}
	return false
}

func waitVMSSH(t *testing.T, vm string) string {
	t.Helper()
	var ip string
	for range 90 {
		out, err := exec.Command("tart", "ip", vm).Output()
		if ip = strings.TrimSpace(string(out)); err == nil && ip != "" {
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.NotEmpty(t, ip, "no ip for %s after 180s", vm)
	for range 90 {
		if exec.Command("ssh", sshArgs(ip, "true")...).Run() == nil {
			return ip
		}
		time.Sleep(2 * time.Second)
	}
	require.Fail(t, fmt.Sprintf("no ssh to %s (%s) after 180s", vm, ip))
	return ip
}

func vmScript(job installJob) string {
	var b strings.Builder
	b.WriteString(`export PATH=$HOME/.local/bin:$HOME/go/bin:$HOME/.pyenv/bin:$HOME/.pyenv/shims:/opt/homebrew/bin:/opt/homebrew/sbin:$PATH
export XDG_CONFIG_HOME=$HOME/.config XDG_DATA_HOME=$HOME/.local/share XDG_BIN_HOME=$HOME/.local/bin
export XDG_STATE_HOME=$HOME/.local/state XDG_CACHE_HOME=$HOME/.cache
export PYENV_ROOT=$HOME/.pyenv NVM_DIR=$HOME/.config/nvm GOBIN=$HOME/go/bin GOFLAGS=-modcacherw
`)
	fmt.Fprintf(&b, "export CHE_E2E=1 CHE_LOG_LEVEL=%s CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH=0\n", logLevel())
	b.WriteString(`if [ -d '/Volumes/My Shared Files/che-cache' ]; then export CHE_PACKAGES_DOWNLOAD_CACHE_DIR='/Volumes/My Shared Files/che-cache'; fi
export GOCOVERDIR=$HOME/cover
mkdir -p $HOME/.local/bin $HOME/.config $HOME/.local/state $HOME/cover $HOME/work
cd $HOME/work
printf 'e2e:\n  options: {autoDiscover: true}\n' > che.yml
`)
	appendRunVerify(&b, "$HOME/che-e2e/che", job)
	return b.String()
}

func vmPush(t *testing.T, ip string) {
	t.Helper()
	bin := binPath(t)
	pack := exec.Command("tar", "-cf", "-", "-C", filepath.Dir(bin), filepath.Base(bin))
	unpack := exec.Command("ssh", sshArgs(ip, "mkdir -p che-e2e && tar -xf - -C che-e2e && mv -f che-e2e/"+filepath.Base(bin)+" che-e2e/che")...)
	pipe, err := pack.StdoutPipe()
	require.NoError(t, err)
	unpack.Stdin = pipe
	require.NoError(t, pack.Start())
	out, err := unpack.CombinedOutput()
	require.NoError(t, err, "push che into VM: %s", out)
	require.NoError(t, pack.Wait())
}

func vmPullCover(t *testing.T, ip string) {
	t.Helper()
	dir := os.Getenv("E2E_GOCOVERDIR")
	if dir == "" {
		return
	}
	pack := exec.Command("ssh", sshArgs(ip, "tar -cf - -C cover .")...)
	unpack := exec.Command("tar", "-xf", "-", "-C", dir)
	pipe, err := pack.StdoutPipe()
	require.NoError(t, err)
	unpack.Stdin = pipe
	require.NoError(t, pack.Start())
	out, err := unpack.CombinedOutput()
	require.NoError(t, err, "pull cover dir from VM: %s", out)
	require.NoError(t, pack.Wait())
}

func runJobVM(t *testing.T, job installJob) (string, bool) {
	t.Helper()
	if _, err := exec.LookPath("tart"); err != nil {
		t.Skip("tart absent; darwin with_no_deps mode needs it")
	}
	if !tartHasVM(t, vmTemplate) {
		t.Skipf("tart template %s absent; run ci/e2e-install-methods-vm-template.zsh (make e2e-install-methods prepares it)", vmTemplate)
	}
	vm := fmt.Sprintf("che-e2e-%d-%d", os.Getpid(), vmSeq.Add(1))
	t.Logf("tart clone %s -> %s", vmTemplate, vm)
	cloneOut, err := exec.Command("tart", "clone", vmTemplate, vm).CombinedOutput()
	require.NoError(t, err, "tart clone %s %s: %s", vmTemplate, vm, cloneOut)
	boot := exec.Command("tart", "run", "--no-graphics",
		"--dir=che-cache:"+filepath.Join(devCacheDir(t), "downloads"), vm)
	require.NoError(t, boot.Start())
	t.Cleanup(func() {
		t.Logf("stop + delete %s", vm)
		_ = exec.Command("tart", "stop", vm).Run()
		_ = boot.Wait()
		_ = exec.Command("tart", "delete", vm).Run()
	})
	t.Logf("boot %s, waiting for ssh (up to ~3 min)", vm)
	ip := waitVMSSH(t, vm)
	t.Logf("%s at %s, pushing che", vm, ip)
	vmPush(t, ip)
	run := exec.Command("ssh", sshArgs(ip, "sh -ec "+shq(vmScript(job)))...)
	out, err := runStreamed(t, run, "darwin VM ("+vm+") install "+strings.Join(job.packages, " "))
	vmPullCover(t, ip)
	require.NoError(t, err, "darwin VM install %s", strings.Join(job.packages, " "))
	return out, strings.Contains(out, noDepsSkipMarker)
}

// [<] 🤖🤖
