##[>] 🤖
downstream:
  - uri: gitlab.com/konradodwrot/go-modules/che
    type: goModule
    versionEnvVar: GO_MODULES_CHE_REF
    version: {{ env.Getenv "GO_MODULES_CHE_REF" }}
  - uri: gitlab.com/konradodwrot/go-modules/che-schema
    type: file
    versionEnvVar: CHE_SCHEMA_REF
    version: {{ env.Getenv "CHE_SCHEMA_REF" }}
  - uri: gitlab.com/konradodwrot/go-modules/get-os-open-files-with
    type: goModule
    versionEnvVar: GO_MODULES_GET_OS_OPEN_FILES_WITH_REF
    version: {{ env.Getenv "GO_MODULES_GET_OS_OPEN_FILES_WITH_REF" }}
  - uri: gitlab.com/konradodwrot/go-modules/get-term-open-files-with
    type: goModule
    versionEnvVar: GO_MODULES_GET_TERM_OPEN_FILES_WITH_REF
    version: {{ env.Getenv "GO_MODULES_GET_TERM_OPEN_FILES_WITH_REF" }}
  - uri: gitlab.com/konradodwrot/go-modules/lib
    type: goModule
    versionEnvVar: GO_MODULES_LIB_REF
    version: {{ env.Getenv "GO_MODULES_LIB_REF" }}
  - uri: gitlab.com/konradodwrot/go-modules/che-packages-schema
    type: file
    versionEnvVar: CHE_PACKAGES_SCHEMA_REF
    version: {{ env.Getenv "CHE_PACKAGES_SCHEMA_REF" }}
##[<] 🤖
