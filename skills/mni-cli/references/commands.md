# mNi CLI command reference

## Install the CLI

Choose one supported installation method:

```sh
nix run github:mNi-Cloud/cli
brew tap mNi-Cloud/tap && brew install mni
```

Binary archives are also published on the repository's GitHub releases page.

## Authentication and targeting

```sh
mni login
mni login --context staging --server https://api.example.com --issuer https://auth.example.com
mni whoami
mni logout

mni config get-contexts
mni config current-context
mni config use-context <name>
mni config use-tenant <name>
mni config delete-context <name>
```

All applicable commands accept `--context <name>` and `--tenant <name>` (`-t`). The `MNI_CONTEXT` and `MNI_TENANT` environment variables provide the same overrides.

## Discovery and resources

```sh
mni api-resources
mni explain <resource>[.<field>...] [--status] [--recursive]
mni get <resource> [name] [-o table|json|yaml|jsonpath=<path>]
mni describe <resource> <name>
mni dependencies <resource> <name> [-o ...]
mni dependents <resource> <name> [--direct] [-o ...]
mni apply -f <manifest.yaml>
mni edit <resource> <name>
mni delete <resource> <name> [--yes]
```

A manifest file may contain multiple YAML documents. Each document requires `apiVersion`, `kind`, and `metadata.name`.

## Tenants

```sh
mni tenants list [-o ...]
mni tenants create <name> [--display-name <text>] [--description <text>]
mni tenants delete <name> [--yes]
mni tenants members <tenant> [-o ...]
mni tenants add-member <tenant> <username> [--role <role> ...]
mni tenants remove-member <tenant> <user-id>
```

Use the user ID returned by `mni tenants members` for `remove-member`.

## Virtual machines and SSH keys

```sh
mni ssh-key add <name> <public-key-file>
mni vm start <name>
mni vm stop <name>
mni vm restart <name>
mni vm serial <name>
mni vm vnc <name> [--port <port>]
```

Port `0`, the default for VNC, selects a free local port. The listener binds to `127.0.0.1`.

## Containers

```sh
mni ctr logs <name> [--follow] [--tail <lines>] [--timestamps] [--previous] [--since <duration>]
mni ctr exec <name> [--stdin] [--tty] -- <command> [args...]
```

`--since` accepts Go-style durations such as `30s`, `5m`, or `2h`. `--tty` requires `--stdin`.

## Shell completion

```sh
source <(mni completion bash)
source <(mni completion zsh)
mni completion fish > ~/.config/fish/completions/mni.fish
```
