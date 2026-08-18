---
name: mni-cli
description: Operate mNi Cloud with the `mni` command-line client. Use when inspecting or managing mNi Cloud contexts, tenants, API resources, manifests, dependency graphs, virtual machines, containers, or SSH keys, and when diagnosing `mni` CLI commands or output.
---

# mNi CLI

Use `mni` to inspect and manage mNi Cloud. Prefer discovery from the connected server over assumptions because available resource kinds and schemas can vary by deployment.

## Start safely

1. Verify the binary with `mni --version` and inspect command-specific help when syntax is uncertain.
2. Check the active identity and target with `mni whoami`. For multiple environments or tenants, pass `--context <name>` and `--tenant <name>` explicitly.
3. Discover available kinds with `mni api-resources`.
4. Inspect the relevant schema with `mni explain <resource> --recursive` before authoring or changing a manifest.

Do not run `login`, `apply`, `edit`, `delete`, tenant mutations, VM power operations, `ctr exec`, or `ssh-key add` unless the user's request authorizes that state change. Treat `mni login` as interactive because it starts a browser OAuth flow.

## Inspect resources

- List objects with `mni get <resource>`.
- Read structured output with `mni get <resource> [name] -o json` or `-o yaml`.
- Extract one field with `-o 'jsonpath=<path>'`.
- Summarize an object's state and dependency graph with `mni describe <resource> <name>`.
- Inspect dependencies with `mni dependencies <resource> <name>`.
- Inspect deletion impact with `mni dependents <resource> <name>`; add `--direct` only when transitive dependents are not wanted.

Use structured output for automation and table output for humans. Preserve command output and exit status when diagnosing failures.

## Change resources

Before applying a manifest:

1. Resolve the intended context and tenant.
2. Run `mni explain` for the resource and important nested fields.
3. Inspect an existing peer resource in YAML when one exists.
4. Write the manifest to a file and run `mni apply -f <file>`.
5. Verify the result with `mni get` or `mni describe`.

`mni apply` creates missing resources and patches existing resources. It does not accept stdin; provide a file path. Use `mni edit <resource> <name>` only for an explicitly requested interactive edit.

Before deletion, run `mni dependents <resource> <name>` and report the cascade. Keep the interactive confirmation. Use `--yes` only when the user explicitly authorizes unattended deletion. Tenant deletion removes everything inside the tenant and requires the same care.

## Operate workloads

- Read container logs with `mni ctr logs <name>` and narrow them with `--tail`, `--since`, `--timestamps`, or `--previous`.
- Use `--follow` only when ongoing streaming is useful and can be stopped cleanly.
- Run a container command as `mni ctr exec <name> -- <command> [args...]`. Add `--stdin` only when input is needed; `--tty` also requires `--stdin`.
- Run VM power actions with `mni vm start|stop|restart <name>` only after confirming the requested operation and target.
- Open `mni vm serial <name>` only in an interactive terminal. `mni vm vnc <name>` exposes the console only on loopback and prints the selected port.

## Consult the command reference

Read [references/commands.md](references/commands.md) for installation, context selection, tenant administration, output formats, and the complete command map. Prefer `mni <command> --help` if the installed version differs from the reference.
