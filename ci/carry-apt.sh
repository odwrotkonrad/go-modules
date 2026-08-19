#!/bin/sh
##[>] 🤖🤖
#[what] copy the live site's apt tree into $1, so a Pages deploy that rebuilds no apt repo keeps
#   serving the one already published. --mirror cannot do this: Pages serves no directory index,
#   so there is nothing to crawl. metadata paths are fixed, .deb paths are named by the Packages
#   indexes, so both are fetched explicitly. plain sh and busybox wget: the mkdocs image is alpine
set -eu

site="${APT_SITE_URL:?}"
out="${1:?}"

for f in dists/stable/Release dists/stable/InRelease dists/stable/Release.gpg \
         dists/stable/main/binary-amd64/Packages dists/stable/main/binary-amd64/Packages.gz \
         dists/stable/main/binary-arm64/Packages dists/stable/main/binary-arm64/Packages.gz; do
  mkdir -p "$out/$(dirname "$f")"
  wget -q -O "$out/$f" "$site/$f" || rm -f "$out/$f"
done

#[why] no Release means the live site serves no apt repo: nothing to carry, and the caller decides
#   whether that is expected
if [ ! -s "$out/dists/stable/Release" ]; then
  rm -rf "$out"
  exit 0
fi

for arch in amd64 arm64; do
  idx="$out/dists/stable/main/binary-$arch/Packages"
  [ -s "$idx" ] || continue
  sed -n 's|^Filename: *||p' "$idx" | while read -r pkg; do
    [ -n "$pkg" ] || continue
    [ -s "$out/$pkg" ] && continue
    mkdir -p "$out/$(dirname "$pkg")"
    wget -q -O "$out/$pkg" "$site/$pkg" || rm -f "$out/$pkg"
  done
done
##[<] 🤖🤖
