# Repositories

Repo index for "test-group". Directory tree:

```
che
tools/
  configs
  render-files
```

Maps 1:1 to gitlab group "test-group". Each repo's purpose follows.

## Repo: ./che

### Purpose

#### What It Is

Spec-driven dotfile configuration loader.

#### Why It Exists

Shell dotfile loading is fragile.

#### Goals

- One spec loads any host.

## Subgroup: ./tools

### Repo: ./tools/configs

#### Purpose

##### What It Is

Dotfiles extended into root OS space, loaded by che.

##### Why It Exists

Stateful system configuration as one tree.

##### Goals

- Every option explicit.

### Repo: ./tools/render-files

#### Purpose

##### What It Is

Shared render engine and doc-rendering CLIs.

##### Why It Exists

One render implementation for all repos.

##### Goals

- Generated docs stay fresh.
