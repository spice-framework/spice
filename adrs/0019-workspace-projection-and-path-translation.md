# ADR 0019: Projected workspaces use the real shell and one path authority

## Status

Accepted.

## Context

Developers and coding agents need ordinary filesystem tools over Spice View
paths, while Go and Git must continue to operate on the canonical checkout.
Symlinks are not a safe writable authority because atomic-save workflows can
replace a link itself. A custom shell language would discard the user's normal
shell behavior.

## Decision

`spice shell` materializes or attaches to a projected workspace, changes to its
View root, prepends command-broker shims to `PATH`, exports opaque session
metadata, and launches the user's existing Bash, Zsh, Fish, sh, PowerShell, or
CMD. `spice shell -- command` launches one process and `spice exec -- command`
runs one projected command. Spice does not parse shell syntax or provide a
terminal emulator.

The first production provider uses real session backing files and a
reconciliation daemon. One writable lease exists per physical checkout;
additional clients attach or use read-only mode. The daemon owns reversible
mapping, base/session/physical hashes, atomic canonical writes, create/delete,
same-package View moves, external-change conflicts, generated refresh, a
journal, and crash recovery. It listens only on a same-user local Unix socket
or Windows named pipe, requires a random inherited token, bounds messages, and
opens no network listener.

One `spice-tool-proxy` executable dispatches by shim name. Ordinary
filesystem-oriented tools run in the projection. Package-sensitive Go tools
use immutable overlay snapshots from the canonical root where supported and
flush for commands whose behavior reads physical sources directly. Git runs
against the physical worktree with exact path-token translation. Repository-
changing Git operations flush, suspend reconciliation, execute canonically,
reload the model, and refresh the projection.

Cross-package moves are semantic refactors and never raw filesystem renames.
Conflicting external changes are retained and never silently overwritten.

## Consequences

- Shell syntax, aliases, history, pipes, redirects, scripts, and ordinary file
  APIs continue to work.
- The materialized provider is portable and remains the fallback if later
  ProjFS or FUSE providers are added.
- The broker remains necessary even with native providers because View
  directories are not canonical Go package directories.
