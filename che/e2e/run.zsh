#!/usr/bin/env zsh
##[>] 🤖🤖
set -eu -o pipefail

e2e_dir=${0:a:h}
bin=$e2e_dir/../dist/che
work=${$(mktemp -d):A}

fail() { print -u2 "FAIL: $1 (workdir kept: $work)"; exit 1 }
step() { print "== $1" }

assert_file() { [[ -f $1 ]] || fail "missing file $1" }
assert_dir() { [[ -d $1 ]] || fail "missing dir $1" }
assert_link() {
  [[ -L $1 ]] || fail "not a symlink: $1"
  [[ $(readlink $1) == ${~2} ]] || fail "symlink $1 -> $(readlink $1), want $2"
}
assert_content() { [[ $(<$1) == $2 ]] || fail "content of $1 is '$(<$1)', want '$2'" }
assert_absent() { [[ ! -e $1 && ! -L $1 ]] || fail "present but must be absent: $1" }
assert_out() { grep -qF -- $2 <<<$3 || fail "$1: output misses '$2'" }
assert_not_out() { grep -qF -- $2 <<<$3 && fail "$1: output must not carry '$2'" || true }

typeset -a NS=(plain conditional remote)

assert_ns_absent() {
  local ns
  for ns in $NS; do
    assert_absent $HOME/$ns/link.txt
    assert_absent $HOME/$ns/copy
    assert_absent $HOME/$ns/tpl
    assert_absent $HOME/$ns/made-dir
  done
  assert_absent $HOME/dropped
}

assert_clean() {
  assert_ns_absent
  local ns
  for ns in $NS; do assert_absent $HOME/$ns/script-marker; done
}

step "setup: workdir $work"
cp -R $e2e_dir/local $work/local
cp -R $e2e_dir/remote $work/remote
sed -i.bak "s|__REMOTE_DIR__|$work/remote|" $work/local/che.yml
rm $work/local/che.yml.bak

export HOME=$work/home
export XDG_STATE_HOME=$HOME/.local/state
export XDG_CACHE_HOME=$HOME/.cache
export XDG_CONFIG_HOME=$HOME/.config
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
unset -m 'CHE_*' 2>/dev/null || true
export CHE_E2E=1
export CHE_VALIDATE_SPEC=error
mkdir -p $HOME

for repo in local remote; do
  git -C $work/$repo init -q -b main
  git -C $work/$repo add -A
  git -C $work/$repo -c user.email=e2e@invalid -c user.name=e2e commit -qm init
done

che() { print -r -- "\$ che $*"; $bin -C $work/local "$@" }

step "discover-profiles"
out=$(che discover-profiles 2>&1)
print -r -- $out
assert_out discover '(match): plain:' $out
assert_out discover '(match): conditional:' $out
assert_out discover 'ops-remote' $out
assert_out discover 'noMatchDue[env:CHE_E2E_DROPPED]): dropped' $out
assert_not_out discover '(match): dropped:' $out

step "run --dry-run=all"
che run --dry-run=all
assert_clean

step "run"
che run
for ns in $NS; do
  assert_link $HOME/$ns/link.txt "*/root/$ns/link.txt"
  assert_content $HOME/$ns/copy "copy-$ns"
  assert_content $HOME/$ns/tpl "RENDERED-${(U)ns}"
  assert_dir $HOME/$ns/made-dir
  assert_content $HOME/$ns/script-marker "marker-$ns"
done
assert_link $HOME/plain/link.txt $work/local/root/plain/link.txt
assert_absent $HOME/dropped

step "uninstall --dry-run"
che uninstall --dry-run
for ns in $NS; do
  assert_file $HOME/$ns/copy
  assert_link $HOME/$ns/link.txt "*/root/$ns/link.txt"
done

step "uninstall"
for ns in $NS; do rm $HOME/$ns/script-marker; done
che uninstall
assert_clean
for ns in $NS; do assert_absent $HOME/$ns; done

step "make-dirs cycle"
che make-dirs --dry-run=all
assert_ns_absent
che make-dirs
for ns in $NS; do assert_dir $HOME/$ns/made-dir; done
che uninstall
assert_ns_absent

step "make-links cycle"
che make-links --dry-run=all
assert_ns_absent
che make-links
for ns in $NS; do assert_link $HOME/$ns/link.txt "*/root/$ns/link.txt"; done
che uninstall
assert_ns_absent

step "make-copies cycle"
che make-copies --dry-run=all
assert_ns_absent
che make-copies
for ns in $NS; do assert_content $HOME/$ns/copy "copy-$ns"; done
che uninstall
assert_ns_absent

step "render-templates cycle"
che render-templates --dry-run=all
assert_ns_absent
che make-dirs
che render-templates
for ns in $NS; do assert_content $HOME/$ns/tpl "RENDERED-${(U)ns}"; done
che uninstall
assert_ns_absent

step "run-scripts cycle"
che run-scripts --dry-run=all
for ns in $NS; do assert_absent $HOME/$ns/script-marker; done
che run-scripts
for ns in $NS; do assert_content $HOME/$ns/script-marker "marker-$ns"; done
che uninstall
for ns in $NS; do rm $HOME/$ns/script-marker; rmdir $HOME/$ns 2>/dev/null || true; done
assert_clean

step "prune-broken-links cycle"
che make-links
for ns in $NS; do assert_link $HOME/$ns/link.txt "*/root/$ns/link.txt"; done
rm $work/local/root/plain/link.txt
che prune-broken-links --dry-run=all
[[ -L $HOME/plain/link.txt ]] || fail "dry-run pruned $HOME/plain/link.txt"
che prune-broken-links
assert_absent $HOME/plain/link.txt
assert_link $HOME/conditional/link.txt "*/root/conditional/link.txt"
assert_link $HOME/remote/link.txt "*/root/remote/link.txt"
git -C $work/local checkout -q -- root/plain/link.txt
che uninstall
assert_ns_absent

step "PASS"
rm -rf $work
##[<] 🤖🤖
