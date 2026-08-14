# Spice Views and projected workspaces

Spice Views are the normal way developers and coding agents navigate a Spice
application. They present a concise class-oriented tree while preserving the
physical checkout as fully valid Go.

## Standard View tree

```text
src/main/go
src/main/resources
src/test/go
src/test/resources
build/generated/spice
```

For example:

```text
src/main/go/users/domain/User.go
src/main/go/users/application/UserService.go
src/main/go/users/persistence/UserRepository.go
src/main/go/users/web/UserController.go
src/test/go/users/application/UserServiceTest.go
```

The physical files may remain:

```text
cmd/commerce/main.go
internal/users/user.go
internal/users/user_service.go
internal/users/user_repository.go
internal/users/user_controller.go
internal/users/user_service_test.go
internal/spicegen/commerce/...
```

The source bytes remain ordinary Go. A View changes presentation, not the
package clause, import path, symbol, generated call, breakpoint, or stack
frame.

## Identity and imports

Spice uses exactly three relevant identities:

```text
project: commerce
canonical Go symbol: github.com/acme/commerce/internal/users.User
View: src/main/go/users/domain/User.go
```

There is no semantic Spice namespace. Go imports remain Go imports. Spice may
complete and insert imports, choose aliases, navigate through View paths, and
visually conceal a qualifier without changing the source. `// @import`
continues to resolve annotation descriptors only; it is not an ordinary type
import.

Go packages remain the coarse architecture boundary. Views classify finer
roles inside a feature package: `domain`, `application`, `persistence`, `web`,
`configuration`, and `events`.

## Inference

The Toolchain maps `internal/users` to feature `users`; special physical
segments such as `internal`, `pkg`, and `cmd` do not appear in normal View
paths. It classifies primary declarations in this order:

- `@Application`: project root;
- `@Controller`: `web`;
- `@Service`: `application`;
- `@Repository`: `persistence`;
- configuration descriptors: `configuration`;
- listeners and topics: `events`;
- otherwise: `domain`.

Type suffixes, implemented interfaces, source filenames, architectural
annotations, and package-level roles may refine this deterministic inference.
Ambiguity fails closed. Tests mirror the production declaration View whenever
possible. Generated files appear under `build/generated/spice` and are
read-only.

A same-package View move changes classification or an exceptional override. A
cross-package move changes Go identity and must use a semantic Spice refactor
that updates packages, imports, references, module/View edges, and cycle
validation.

## `spice shell`

```text
spice shell
spice shell --cwd src/main/go/users/application
spice shell -- codex
spice shell -- claude
spice shell -- opencode
spice shell -- spice-agent
spice exec -- go test ./...
```

Spice creates or attaches to a projected workspace, prepends a command broker
to `PATH`, changes to the requested View directory, and launches the user's
real shell or command. Bash, Zsh, Fish, sh, PowerShell, and CMD keep their
normal syntax, history, aliases, pipes, redirects, job control, and scripts.
Spice does not implement a shell parser or terminal emulator.

The portable provider materializes real directories and files in a per-user
runtime location. Ordinary `ls`, `cat`, `rg`, `grep`, `sed`, `awk`, Python,
Node, and file APIs operate normally. Real session backing files and a
reconciliation daemon are the write authority; writable symlinks are not.

One writable session exists per physical checkout. Additional clients attach
or use read-only mode. The daemon uses a same-user Unix-domain socket or
Windows named pipe, a random inherited token, bounded messages, and no network
listener. It journals base/session/physical hashes, writes canonical files
atomically, detects external conflicts, and preserves dirty crashed sessions
for recovery.

Workspace commands are:

```text
spice workspace status
spice workspace conflicts
spice workspace resolve
spice workspace flush
spice workspace recover
spice workspace stop
```

## Command broker

View directories do not define canonical Go packages, so package-sensitive
tools pass through one shared broker:

- filesystem-oriented tools run unchanged in the projection;
- `make`, `just`, `task`, and `mage` flush and run at the canonical root;
- Go commands use immutable overlay snapshots where supported and translate
  package patterns from the Project Model;
- Git and diagnostic tools execute canonically and translate exact known path
  tokens;
- `spice` and the brokered gopls proxy attach directly to the session.

From `src/main/go/users/application`, `go test .` means the owning canonical Go
package. `go test ./...` means the distinct canonical packages represented
below that View subtree. From `src/main/go`, it means all main-source
application packages.

Build, test, run, list, vet, and `go tool` use overlays where supported. Module
edits, code generation, cgo-heavy workflows, and generators that inspect their
own source paths flush physical files first. Outside `spice shell`, ordinary Go
uses the synchronized canonical checkout.

Git runs against the physical `.git` and worktree. `status`, `diff`, `add`,
`restore`, `commit`, `checkout`, `apply`, and other supported operations accept
and display View paths. Repository-changing operations flush, suspend
reconciliation, execute canonically, reload the Project Model, and refresh the
projection.

## Editors and agents

GoLand's dedicated Spice Project View and the first VS Code Spice Explorer open
real physical files in the editor while presenting View nodes. Native Go,
gopls, and Delve therefore retain their normal behavior. Zed initially adds
View-aware LSP presentation and shell tasks within its extension API ceiling.

Coding agents work inside `spice shell -- <agent>`. They receive View roots,
generated workspace guidance, and the canonical-path-free agent Project Model.
Semantic commands provide stable create, move, rename, dependency, tree, and
verification operations. A later LSP proxy maps View URIs to canonical gopls
and compiler URIs and maps edits and diagnostics back.

The projection is a developer-experience boundary, not a security sandbox. A
process with the user's privileges may discover parent paths or process
metadata. `spice path --physical <view-path>` is the explicit diagnostic escape
hatch.
