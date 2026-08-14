#!/usr/bin/env zsh
##[>] 🤖🤖
# Delegates a darwin host-mode e2e-install-methods run into a tart macOS VM so
# installs never touch the bare-metal host. Idempotent: clones the VM from
# $VM_BASE if absent, boots it if stopped, bootstraps ssh-key auth on first use
# (SSH_ASKPASS reading the runner vm password op ref, value never echoed),
# tar-pipes the git-tracked tree plus the host-built darwin dist binaries in,
# runs the prebuilt e2e.test with the caller's E2E_INSTALL_* selection,
# tar-pipes the cover dir back. VM stays running for fast reruns:
# `tart delete $VM` resets.
set -eu

VM="${VM:-che-e2e-install-methods}"
VM_BASE="${VM_BASE:-macos-tahoe-vanilla-base}"
ROOT="${0:a:h:h}"
REMOTE_REPO=go-modules
source "${0:a:h}/e2e-install-methods-vm-lib.zsh"

if ! { tart list | awk '{print $2}' | grep -qx "$VM" } {
  echo "## clone $VM_BASE -> $VM"
  tart clone "$VM_BASE" "$VM"
}
if [[ "$(tart list | awk -v vm="$VM" '$2 == vm {print $NF}')" != running ]] {
  echo "## boot $VM"
  tart run --no-graphics "$VM" > /dev/null 2>&1 &!
}

vm_wait_ip "$VM"
vm_wait_ssh_port
vm_bootstrap_key

echo "## sync repo + dist into $VM"
cd "$ROOT"
"${SSH[@]}" "$VM_USER@$IP" "rm -rf $REMOTE_REPO/che/cover-e2e-install-methods && mkdir -p $REMOTE_REPO"
{ git ls-files -z | while IFS= read -r -d '' f; do [[ -e $f ]] && printf '%s\0' "$f"; done; printf '%s\0' che/dist/che che/dist/e2e.test } \
  | tar -cf - --null -T - \
  | "${SSH[@]}" "$VM_USER@$IP" "tar -xf - -C $REMOTE_REPO"

echo "## run e2e.test in $VM"
RC=0
"${SSH[@]}" "$VM_USER@$IP" "
  cd $REMOTE_REPO/che && mkdir -p cover-e2e-install-methods && cd e2e &&
  E2E_BIN=\$HOME/$REMOTE_REPO/che/dist/che \
  E2E_GOCOVERDIR=\$HOME/$REMOTE_REPO/che/cover-e2e-install-methods \
  E2E_INSTALL_METHOD='${E2E_INSTALL_METHOD:-all}' \
  E2E_INSTALL_PACKAGE='${E2E_INSTALL_PACKAGE:-}' \
  E2E_INSTALL_MODE='${E2E_INSTALL_MODE:-}' \
  E2E_INSTALL_PLATFORM='${E2E_INSTALL_PLATFORM:-}' \
  ../dist/e2e.test -test.v -test.count=1 -test.timeout 45m -test.run TestE2EInstallMethods
" || RC=$?

echo "## sync cover dir back"
rm -rf che/cover-e2e-install-methods
"${SSH[@]}" "$VM_USER@$IP" "tar -cf - -C $REMOTE_REPO/che cover-e2e-install-methods" | tar -xf - -C che

exit $RC
##[<] 🤖🤖
