##[>] 🤖🤖
VM_USER=user
VM_PASSWORD_REF="op://ProgrammaticAccess/gitlab_runner/vm_password"
SSH=(ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)

vm_wait_ip() {
  local vm=$1
  IP=""
  echo "## wait for ip"
  for i in {1..90}; do
    IP="$(tart ip "$vm" 2>/dev/null || true)"
    if [[ -n "$IP" ]] break
    sleep 2
  done
  [[ -n "$IP" ]] || { echo "no ip for $vm after 180s" >&2; exit 1 }
  echo "## $vm at $IP"
}

vm_wait_ssh_port() {
  echo "## wait for ssh"
  for i in {1..90}; do
    if { nc -z -G 2 "$IP" 22 2>/dev/null } break
    sleep 2
  done
}

vm_bootstrap_key() {
  if ! { "${SSH[@]}" -o BatchMode=yes "$VM_USER@$IP" true 2>/dev/null } {
    echo "## bootstrap ssh key"
    PUB="$(< ~/.ssh/id_access.pub)"
    ASKPASS="$(mktemp)"
    print -rl -- '#!/bin/sh' "exec '$(command -v op)' read '$VM_PASSWORD_REF'" > "$ASKPASS"
    chmod 700 "$ASKPASS"
    SSH_ASKPASS="$ASKPASS" SSH_ASKPASS_REQUIRE=force \
      "${SSH[@]}" -o PasswordAuthentication=yes -o KbdInteractiveAuthentication=yes -o PubkeyAuthentication=no \
      "$VM_USER@$IP" "mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '$PUB' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
    rm -f "$ASKPASS"
    "${SSH[@]}" -o BatchMode=yes "$VM_USER@$IP" true
  }
}
##[<] 🤖🤖
