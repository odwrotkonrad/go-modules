##[>] 🤖🤖
.test-install-package-list: &packages
{{- range $name, $_ := (file.Read "che/internal/packages/packages.yml" | data.YAML).packages }}
  - {{ $name }}
{{- end }}

.test-install-package:
  variables:
    METHOD: all
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event" || $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
      when: manual
      allow_failure: true
  artifacts:
    paths:
      - che/cover-e2e-install-methods/

.test-install-package-linux:
  extends: .test-install-package
  services:
    - docker:dind
  variables:
    DOCKER_HOST: tcp://docker:2376
    DOCKER_TLS_CERTDIR: "/certs"
    DOCKER_TLS_VERIFY: "1"
    DOCKER_CERT_PATH: "$DOCKER_TLS_CERTDIR/client"
    E2E_INSTALL_CACHE_DIR: $CI_PROJECT_DIR/.cache/che-e2e-install
  before_script:
    - apt-get update && apt-get install -y --no-install-recommends docker.io

test-install-package-darwin-arm64:
  extends: .test-install-package
  stage: test-e2e-package-installs-darwin-arm64
  tags:
    - saas-macos-medium-m1
  image: macos-26-xcode-26
  needs:
    - job: build-che-cover-darwin
      optional: true
  before_script:
    - curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --retry-all-errors -o /tmp/go.tar.gz "https://go.dev/dl/go1.26.4.darwin-arm64.tar.gz"
    - sudo tar -xzf /tmp/go.tar.gz -C /usr/local
    - export PATH=/usr/local/go/bin:$PATH
  script:
    - make -C che e2e-install-methods PACKAGE="$PACKAGE" METHOD="$METHOD" PLATFORM=darwin-arm64 MODE=with_deps $(test -x che/dist/e2e.test && echo "BUILD_DEP= E2E_USE_TEST_BIN=1")
  parallel:
    matrix:
      - PACKAGE: *packages

test-install-package-linux-amd64:
  extends: .test-install-package-linux
  stage: test-e2e-package-installs-linux-amd64
  image: $CI_IMAGE
  needs:
    - job: build-che-cover
      optional: true
  script:
    - make -C che e2e-install-methods PACKAGE="$PACKAGE" METHOD="$METHOD" PLATFORM=linux-amd64 MODE=with_no_deps $(test -x che/dist/e2e.test && echo "BUILD_DEP= E2E_USE_TEST_BIN=1")
  parallel:
    matrix:
      - PACKAGE: *packages

test-install-package-linux-arm64:
  extends: .test-install-package-linux
  stage: test-e2e-package-installs-linux-arm64
  tags:
    - saas-linux-small-arm64
  image: $CI_IMAGE_ARM64
  needs:
    - job: build-che-cover-linux-arm64
      optional: true
  script:
    - make -C che e2e-install-methods PACKAGE="$PACKAGE" METHOD="$METHOD" PLATFORM=linux-arm64 MODE=with_no_deps $(test -x che/dist/e2e.test && echo "BUILD_DEP= E2E_USE_TEST_BIN=1") $(test -x che/dist/che-linux-arm64 && echo "E2E_INSTALL_METHODS_DEPS=")
  parallel:
    matrix:
      - PACKAGE: *packages
##[<] 🤖🤖
