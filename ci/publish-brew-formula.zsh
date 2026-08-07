#!/usr/bin/env zsh
##[>] 🤖🤖
set -eu

TAG="${CI_COMMIT_TAG:?}"
MODULE_VERSION="${TAG#che/v}"
PKG="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/che/${MODULE_VERSION}"

typeset -A SHA
for plat in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
  typeset -a f
  f=(che/dist/che_${MODULE_VERSION}_${plat}.tar.gz(N) che/darwin-dist/che_${MODULE_VERSION}_${plat}.tar.gz(N))
  if [[ ${#f} -ne 1 ]] { echo "expected one archive for ${plat}, got: ${f:-none}" >&2; exit 1 }
  SHA[$plat]="${$(sha256sum "${f[1]}")%% *}"
done

render_formula() {
  cat > "$2" <<EOF
class $1 < Formula
  desc "Spec-driven dotfile configuration loader"
  homepage "https://gitlab.com/konradodwrot/go-modules"
  version "${MODULE_VERSION}"
  license "MIT"

  on_macos do
    on_arm do
      url "${PKG}/che_${MODULE_VERSION}_darwin_arm64.tar.gz"
      sha256 "${SHA[darwin_arm64]}"
    end
    on_intel do
      url "${PKG}/che_${MODULE_VERSION}_darwin_amd64.tar.gz"
      sha256 "${SHA[darwin_amd64]}"
    end
  end
  on_linux do
    on_arm do
      url "${PKG}/che_${MODULE_VERSION}_linux_arm64.tar.gz"
      sha256 "${SHA[linux_arm64]}"
    end
    on_intel do
      url "${PKG}/che_${MODULE_VERSION}_linux_amd64.tar.gz"
      sha256 "${SHA[linux_amd64]}"
    end
  end

  def install
    bin.install "che", "render-tpl", "render-dirs-tree", "render-makefile-doc", "render-repo-group-index"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/che --version")
  end
end
EOF
}

FORMULA_FILE="${FORMULA_OUT:-$(mktemp)}"
render_formula Che "$FORMULA_FILE"
VERSIONED_FILE="${VERSIONED_FORMULA_OUT:-$(mktemp)}"
render_formula "CheAT${MODULE_VERSION//[.-]/}" "$VERSIONED_FILE"

if [[ "${RENDER_ONLY:-0}" == 1 ]] { echo "rendered ${FORMULA_FILE} ${VERSIONED_FILE}"; exit 0 }

commit_formula() {
  curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --request POST --header "PRIVATE-TOKEN: ${HOMEBREW_TAP_TOKEN:?}" \
    --data-urlencode "branch=main" \
    --data-urlencode "commit_message=chore: che ${MODULE_VERSION}" \
    --data-urlencode "actions[][action]=$1" \
    --data-urlencode "actions[][file_path]=$2" \
    --data-urlencode "actions[][content]=$(<"$3")" \
    "${CI_API_V4_URL}/projects/${HOMEBREW_TAP_PROJECT_ID:?}/repository/commits"
}

publish_formula() {
  echo "pushing $1 (${MODULE_VERSION}) to tap project ${HOMEBREW_TAP_PROJECT_ID:?}"
  if ! commit_formula update "$1" "$2"; then
    echo "update failed, trying create"
    commit_formula create "$1" "$2"
  fi
  echo
}

publish_formula "Formula/che.rb" "$FORMULA_FILE"
publish_formula "Formula/che@${MODULE_VERSION}.rb" "$VERSIONED_FILE"
##[<] 🤖🤖
