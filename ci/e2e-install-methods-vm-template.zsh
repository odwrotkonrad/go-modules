#!/usr/bin/env zsh
##[>] 🤖🤖
set -eu

TEMPLATE="${TEMPLATE:-che-e2e-install-methods-template}"
VM_BASE="${VM_BASE:-macos-tahoe-vanilla-base}"
source "${0:a:h}/e2e-install-methods-vm-lib.zsh"

if { tart list | awk '{print $2}' | grep -qx "$TEMPLATE" } {
  echo "## template $TEMPLATE present"
  exit 0
}

echo "## clone $VM_BASE -> $TEMPLATE"
tart clone "$VM_BASE" "$TEMPLATE"
tart run --no-graphics "$TEMPLATE" > /dev/null 2>&1 &!
vm_wait_ip "$TEMPLATE"
vm_wait_ssh_port
vm_bootstrap_key
echo "## stop $TEMPLATE"
tart stop "$TEMPLATE"
##[<] 🤖🤖
