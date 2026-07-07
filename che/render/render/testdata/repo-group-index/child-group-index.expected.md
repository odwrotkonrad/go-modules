# Repositories

This is "tools" directory with dirs structure containing repositories:

```
configs
render-files
```

This structure maps 1:1 to gitlab group "tools"

Beneath are descriptions and purpose of each repository:

## Repo: ./configs

### Purpose

#### What It Is

Dotfiles extended into root OS space, loaded by che.

#### Why It Exists

Stateful system configuration as one tree.

#### Goals

- Every option explicit.

## Repo: ./render-files

### Purpose

#### What It Is

Shared render engine and doc-rendering CLIs.

#### Why It Exists

One render implementation for all repos.

#### Goals

- Generated docs stay fresh.
