##[>] 🤖
consumes:
  - uri: gitlab.com/konradodwrot/ai-harness/configs
    type: gitRepository
    version: {{ env.Getenv "AI_CONFIGS_REF" }}
  - uri: gitlab.com/konradodwrot/che-packages
    type: gitRepository
    version: {{ env.Getenv "CHE_PACKAGES_REF" }}
  - uri: gitlab.com/konradodwrot/cross-repo/prose/assets
    type: gitRepository
    version: {{ env.Getenv "PROSE_ASSETS_REF" }}
  - uri: gitlab.com/konradodwrot/cross-repo/prose/spec
    type: gitRepository
    version: {{ env.Getenv "PROSE_SPEC_REF" }}
  - uri: us-central1-docker.pkg.dev/staging-499418/ci/ci-linux
    type: ociImage
    version: {{ env.Getenv "OCI_IMAGES_CI_LINUX_REF" }}
  - uri: us-central1-docker.pkg.dev/staging-499418/ci/ci-linux-dind
    type: ociImage
    version: {{ env.Getenv "OCI_IMAGES_CI_LINUX_DIND_REF" }}
##[<] 🤖
