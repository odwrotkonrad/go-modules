##[>] 🤖🤖
PROSE_ASSETS_REF={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_PROSE_ASSETS_REF" }}
PROSE_SPEC_REF={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_PROSE_SPEC_REF" }}
ARTIFACT_REGISTRY={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_ARTIFACT_REGISTRY" }}
ARTIFACT_REGISTRY_PROXY_DOCKERHUB={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_ARTIFACT_REGISTRY_PROXY_DOCKERHUB" }}
CI_IMAGES_REF={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_CI_IMAGES_REF" }}
CHE_PACKAGES_REF={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_CHE_PACKAGES_REF" }}
ENABLE_DARWIN_CI={{ shell "glab variable get -g konradodwrot GRP_KO_VAR_ENABLE_DARWIN_CI" }}
APT_GPG_PRIVATE_KEY=
APT_GPG_PASSPHRASE={{ shell "glab variable get -R konradodwrot/go-modules REPO_VAR_APT_GPG_PASSPHRASE" }}
HOMEBREW_TAP_TOKEN={{ shell "glab variable get -R konradodwrot/go-modules REPO_VAR_HOMEBREW_TAP_TOKEN" }}
HOMEBREW_TAP_PROJECT_ID={{ shell "glab variable get -R konradodwrot/go-modules REPO_VAR_HOMEBREW_TAP_PROJECT_ID" }}
##[<] 🤖🤖
