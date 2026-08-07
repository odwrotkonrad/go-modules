#!/usr/bin/env zsh
##[>] 🤖🤖
# Builds the apt repo tree into ./public-apt from every che .deb in the
# generic package registry (source of truth: Pages deploys replace the whole
# site, so the tree is regenerated from the registry every time). Imports the
# signing key from $APT_GPG_PRIVATE_KEY (file var, armored private key,
# optional $APT_GPG_PASSPHRASE), aptly-publishes distribution "stable"
# component "main" for amd64+arm64, exports the armored public key to
# public-apt/gpg.key. No debs in the registry yet -> only gpg.key, exit 0.
set -eu

API="${CI_API_V4_URL:?}"
PROJECT="${CI_PROJECT_ID:?}"
OUT="$PWD/public-apt"
WORK="$(mktemp -d)"

curl_gl() curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN:?}" "$@"

mkdir -p "$OUT" "$WORK/debs"

export GNUPGHOME="$WORK/gnupg"
mkdir -m 700 "$GNUPGHOME"
gpg --batch --import "${APT_GPG_PRIVATE_KEY:?}"
KEYID="$(gpg --batch --list-secret-keys --with-colons | awk -F: '$1 == "sec" {print $5; exit}')"
gpg --armor --export "$KEYID" > "$OUT/gpg.key"

typeset -A SEEN
integer page=1
while (( 1 )) {
  curl_gl -o "$WORK/packages.json" "${API}/projects/${PROJECT}/packages?package_type=generic&package_name=che&per_page=100&page=${page}"
  if [[ "$(yq -p=json 'length' "$WORK/packages.json")" == 0 ]] break
  yq -p=json -o=tsv '[.[] | select(.name == "che") | [.id, .version]]' "$WORK/packages.json" |
    while IFS=$'\t' read -r id version; do
      if [[ -z "$id" || "$version" == latest ]] continue
      curl_gl -o "$WORK/files.json" "${API}/projects/${PROJECT}/packages/${id}/package_files?per_page=100"
      yq -p=json '.[].file_name' "$WORK/files.json" | grep '\.deb$' |
        while read -r f; do
          if (( ${+SEEN[$f]} )) continue
          SEEN[$f]=1
          echo "downloading ${version}/${f}"
          curl_gl -o "$WORK/debs/$f" "${API}/projects/${PROJECT}/packages/generic/che/${version}/${f}"
        done || true
    done
  (( page++ ))
}

typeset -a DEBS
DEBS=("$WORK"/debs/*.deb(N))
if (( ${#DEBS} == 0 )) {
  echo "no debs in the registry yet, publishing key only"
  exit 0
}

print -r -- "{\"rootDir\": \"$WORK/aptly\"}" > "$WORK/aptly.conf"
aptly -config="$WORK/aptly.conf" repo create -distribution=stable -component=main che
aptly -config="$WORK/aptly.conf" repo add che "$WORK/debs"
typeset -a SIGN
SIGN=(-batch -gpg-key="$KEYID")
if [[ -n "${APT_GPG_PASSPHRASE:-}" ]] SIGN+=(-passphrase="$APT_GPG_PASSPHRASE")
aptly -config="$WORK/aptly.conf" publish repo $SIGN -architectures=amd64,arm64 che
cp -R "$WORK/aptly/public/." "$OUT/"
echo "apt tree ready: $OUT"
##[<] 🤖🤖
