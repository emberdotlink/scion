# Grove Shared Directories

## Status
**Proposed** | March 2026

## Problem Statement

Currently, each scion agent operates in full isolation — its own home directory, its own git worktree workspace, and its own mounted volumes. There is no built-in mechanism for agents within a grove to share persistent, mutable state via the filesystem.

Common use cases that require shared directory access between agents:

- **Shared build caches**: Multiple agents working on the same project could benefit from a shared compilation cache (e.g., `GOCACHE`, `node_modules/.cache`, Bazel output base).
- **Shared artifacts**: One agent produces build artifacts or data files that another agent needs to consume.
- **Shared context / knowledge base**: A directory containing reference materials, design docs, or generated context that all agents should read (and optionally write to).
- **Coordination files**: Lock files, status markers, or message-passing files for lightweight inter-agent coordination.

Users can manually configure volume mounts in `settings.yaml` to achieve this, but the approach requires:
1. Manually managing host-side directories
2. Coordinating mount targets across agent configurations
3. No grove-level abstraction — each agent's config must be updated individually

## Design Goals

1. **Grove-scoped**: Shared directories belong to a grove and are available to all agents within it.
2. **Named by slug**: Each shared directory is identified by a simple slug (e.g., `build-cache`, `artifacts`, `shared-context`).
3. **Leverage existing infrastructure**: Use the existing `VolumeMount` / `Volumes` config and runtime mount machinery.
4. **Deterministic mount paths**: Agents should find shared directories at a well-known, predictable location.
5. **Runtime-portable**: Work across Docker, Podman, Apple container, and Kubernetes (with appropriate adaptation).
6. **Minimal configuration**: Declaring a shared directory at the grove level should be sufficient — agents should not need per-agent volume config.

## Proposed Data Model

### Settings Extension

Add a `shared_dirs` field to grove-level `settings.yaml`:

```yaml
# settings.yaml
shared_dirs:
  - name: build-cache
    read_only: false
  - name: shared-context
    read_only: true    # agents get read-only access by default
  - name: artifacts
```

### Go Types

```go
// SharedDir defines a grove-level shared directory available to all agents.
type SharedDir struct {
    Name     string `json:"name" yaml:"name"`         // Slug identifier (e.g., "build-cache")
    ReadOnly bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"` // Default access mode
}
```

The `Name` field must be a valid slug: lowercase alphanumeric with hyphens, no spaces or special characters.

### Host-Side Storage

Shared directories would be stored alongside agent homes in the grove's external config directory:

```
~/.scion/grove-configs/<slug>__<uuid>/
├── agents/
│   ├── agent-1/
│   │   └── home/
│   └── agent-2/
│       └── home/
└── shared-dirs/           # NEW
    ├── build-cache/       # One directory per declared shared dir
    ├── shared-context/
    └── artifacts/
```

This location is:
- Outside the git repository (no git interaction concerns)
- Alongside agent homes (consistent with existing external storage pattern)
- Per-grove (naturally scoped)
- Persistent across agent restarts and reprovisioning

## Mount Target Options

### Option A: Dedicated Mount Root (Recommended)

Mount shared directories under a well-known root path:

```
/scion/shared/<name>
```

Examples:
- `/scion/shared/build-cache`
- `/scion/shared/shared-context`
- `/scion/shared/artifacts`

**Pros:**
- Clean namespace, no collision with workspace or home
- Obvious and discoverable — agents can `ls /scion/shared/` to see all available shared dirs
- No interaction with git (outside workspace and repo-root)
- No `.gitignore` concerns
- Consistent across all runtimes
- Extensible — `/scion/` prefix could host other scion-managed mounts in the future

**Cons:**
- Requires agents/tasks to reference a non-standard path
- Not in the workspace, so tools that operate on workspace files won't naturally see shared dir contents

### Option B: Under Workspace (e.g., `/workspace/.shared/<name>`)

Mount shared directories inside the workspace tree:

```
/workspace/.shared/build-cache
/workspace/.shared/shared-context
```

**Pros:**
- Visible to tools that operate on the workspace
- Feels "close" to the code being worked on

**Cons:**
- **Git interaction**: If the workspace is a git worktree, the `.shared` directory would appear as untracked content. This requires:
  - Adding `.shared/` to `.gitignore` (modifies repo state, potentially committed)
  - Or adding to worktree-specific excludes (`.git/info/exclude`) — but worktree `.git` is a reference file, not a directory
  - Or using `git update-index --assume-unchanged` — fragile
- **Bind mount over existing dir**: If `.shared/` already exists in the repo, the mount shadows it
- **Agent confusion**: LLM agents may try to commit, modify, or reference `.shared` contents as part of the codebase
- **Workspace sync (K8s)**: Kubernetes runtime syncs workspace via tar — shared dirs would need to be excluded from sync to avoid duplicating large caches

### Option C: Under Home Directory (e.g., `/home/<user>/shared/<name>`)

Mount shared directories inside the agent's home:

```
/home/scion/shared/build-cache
```

**Pros:**
- No git concerns
- Within the agent's "territory"

**Cons:**
- Home directory is per-agent — mounting a shared dir inside it creates a confusing ownership model
- Name collision risk with harness-specific directories
- Less discoverable (buried inside home)
- Different path per runtime user (`/home/scion/shared/` vs `/home/gemini/shared/`)

### Option D: Environment Variable + Configurable Target

Let users specify the mount target per shared dir, with a default:

```yaml
shared_dirs:
  - name: build-cache
    target: /scion/shared/build-cache    # default, can be overridden
  - name: node-cache
    target: /workspace/.cache/node       # custom target
```

**Pros:**
- Maximum flexibility
- Supports use cases where a specific path is required (e.g., tool-specific cache dirs)

**Cons:**
- More complex configuration
- Per-agent override of target path could lead to inconsistency
- Harder to reason about "where are the shared dirs?"

### Recommendation

**Option A (`/scion/shared/<name>`)** as the default, with **Option D's override capability** as an optional extension:

```yaml
shared_dirs:
  - name: build-cache
    # mounts at /scion/shared/build-cache by default
  - name: node-cache
    target: /tmp/node-cache    # optional override
```

Additionally, inject an environment variable `SCION_SHARED_DIR=/scion/shared` so agents and scripts can programmatically discover the shared directory root.

## Implementation Approach

### Phase 1: Core Implementation

#### 1. Config Changes

Add `SharedDir` type to `pkg/api/types.go` and `SharedDirs []SharedDir` to the `Settings` struct in `pkg/config/settings.go`.

Add validation:
- `Name` must be a valid slug (`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
- No duplicate names within a grove
- Optional `Target` override must be an absolute path

#### 2. Storage Provisioning

In `pkg/config/init.go` or a new `pkg/config/shared_dirs.go`:
- When shared dirs are defined in settings, ensure `~/.scion/grove-configs/<grove>/shared-dirs/<name>/` exists
- Create directories lazily on first agent start, or eagerly on `scion init` / settings change
- Implement `GetSharedDirPath(grovePath, name) string` helper

#### 3. Volume Injection

In `pkg/agent/run.go`, during `RunConfig` construction:
- Read `shared_dirs` from grove settings
- For each shared dir, synthesize a `VolumeMount`:
  ```go
  api.VolumeMount{
      Source: sharedDirHostPath,
      Target: fmt.Sprintf("/scion/shared/%s", dir.Name),  // or dir.Target if set
      ReadOnly: dir.ReadOnly,
  }
  ```
- Append to `RunConfig.Volumes` before passing to runtime

This leverages the existing `buildCommonRunArgs()` → volume deduplication → bind mount pipeline with **zero changes to runtime code**.

#### 4. Environment Variable

Add `SCION_SHARED_DIR=/scion/shared` to agent environment variables in the run config.

#### 5. CLI Commands

```bash
# List shared directories for current grove
scion shared list

# Create a new shared directory
scion shared create <name>

# Remove a shared directory (with confirmation)
scion shared remove <name>

# Inspect a shared directory (show path, size, agents using it)
scion shared info <name>
```

### Phase 2: Kubernetes Support

For Kubernetes, local bind mounts are not supported. Shared directories require a different backing mechanism.

**Approach: PersistentVolumeClaim (PVC)**

- When a grove has shared dirs and uses a Kubernetes runtime, create a PVC per shared dir (or one PVC with subdirectories)
- PVC access mode: `ReadWriteMany` (RWX) — requires a storage class that supports it (e.g., NFS, GCE Filestore, EFS)
- Mount the PVC at `/scion/shared/<name>` in the pod spec

```go
// In k8s_runtime.go buildPod():
for _, sd := range config.SharedDirs {
    // Add PVC volume to pod spec
    pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
        Name: fmt.Sprintf("shared-%s", sd.Name),
        VolumeSource: corev1.VolumeSource{
            PersistentVolumeClaim: &corev1.PersistentVolumeClaim{
                ClaimName: fmt.Sprintf("scion-shared-%s-%s", groveSlug, sd.Name),
            },
        },
    })
    // Add volume mount to container
    container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
        Name:      fmt.Sprintf("shared-%s", sd.Name),
        MountPath: fmt.Sprintf("/scion/shared/%s", sd.Name),
        ReadOnly:  sd.ReadOnly,
    })
}
```

**PVC Lifecycle:**
- Created when shared dir is declared and K8s runtime is active
- Storage class configurable via `settings.yaml`:
  ```yaml
  kubernetes:
    shared_dir_storage_class: "standard-rwx"
    shared_dir_size: "10Gi"    # default size per shared dir
  ```
- PVCs persist across agent restarts (they are grove-scoped, not agent-scoped)
- Deleted when shared dir is removed via CLI (with confirmation)

**Alternative: EmptyDir (Ephemeral)**

For cases where persistence across pod restarts is not required, an `EmptyDir` could be used. However, EmptyDir is per-pod and not shared across pods, making it unsuitable for multi-agent sharing in K8s. It would only work if all agents for a grove run in the same pod (not the current model).

### Phase 3: Hub Integration

For the hosted architecture:
- Hub API gains shared dir metadata as part of grove registration
- Runtime brokers provision shared dir storage based on hub grove config
- Broker-side storage at `~/.scion/groves/<hub-grove>/shared-dirs/<name>/`
- Cross-broker sharing would require a network filesystem or object storage — out of scope for initial implementation

## Per-Agent Access Control

The default `read_only` flag on `SharedDir` sets the grove-wide default. Per-agent overrides could be supported via agent config or profiles:

```yaml
# In profile or agent template
shared_dir_overrides:
  - name: artifacts
    read_only: false     # This agent can write to artifacts
  - name: build-cache
    exclude: true        # This agent doesn't get build-cache mounted
```

This is a Phase 2+ concern and can be deferred.

## Concurrency and Safety

Shared directories introduce the possibility of concurrent writes from multiple agents. The design intentionally does **not** provide file-level locking or transactional semantics:

- **Filesystem-level guarantees**: POSIX semantics on the host filesystem apply (atomic rename, O_EXCL create, etc.)
- **User responsibility**: Agents/tasks that write to shared dirs must coordinate at the application level (e.g., using lock files, unique filenames, or atomic write patterns)
- **Read-only default**: The `read_only: true` default for new shared dirs encourages a single-writer pattern

## Alternatives Considered

### Alternative: Extend Existing Volume Config

Instead of a dedicated `shared_dirs` concept, users could be told to configure volumes manually in settings:

```yaml
volumes:
  - source: ~/.scion/grove-configs/my-project__abc123/custom-shared/
    target: /scion/shared/my-data
```

**Why rejected:**
- Requires users to know internal grove paths
- No grove-level abstraction — each profile/template must repeat the config
- No lifecycle management (create/delete)
- Doesn't compose well with Kubernetes (local volumes not supported)

### Alternative: Symlink-Based Sharing

Create symlinks inside agent workspaces pointing to a shared host directory.

**Why rejected:**
- Symlinks don't cross container mount boundaries
- Would require the shared directory to be mounted anyway
- Adds complexity without solving the core problem

### Alternative: Named Docker Volumes

Use Docker named volumes instead of bind mounts for shared dirs.

**Why rejected:**
- Docker named volumes are managed by Docker, not scion — harder to inspect/backup
- Not portable to non-Docker runtimes without adaptation
- Bind mounts are more transparent and consistent with existing scion patterns

## Open Questions

| # | Question | Notes |
|---|----------|-------|
| 1 | **Should shared dirs be auto-mounted to all agents, or opt-in per agent?** | Auto-mount is simpler and matches the "grove-scoped" model. Opt-in (via template/profile) adds flexibility but complexity. Recommend auto-mount with `exclude` override. |
| 2 | **Should there be a size limit or quota for shared dirs?** | Useful for preventing runaway cache growth. Could be enforced via `du` checks or filesystem quotas. Likely a Phase 2 concern. |
| 3 | **How should shared dirs interact with `scion clone` / grove duplication?** | Options: copy shared dirs (expensive), reference the same dirs (surprising), or start fresh (clean). |
| 4 | **Should we support GCS-backed shared dirs?** | The existing `gcs` volume type could back shared dirs for cloud-native setups. Natural extension of the GCS volume support. |
| 5 | **Naming: `shared_dirs` vs `grove_dirs` vs `shared_volumes`?** | `shared_dirs` emphasizes the sharing aspect. `grove_dirs` emphasizes scope. Current preference: `shared_dirs`. |
| 6 | **Should the mount root be `/scion/shared/` or `/scion-mnts/`?** | `/scion/shared/` is more descriptive and leaves room for other scion-managed mounts under `/scion/`. `/scion-mnts/` is shorter but less clear. |
| 7 | **What permissions/ownership should shared dir contents have?** | Host UID/GID may differ from container UID/GID. Existing `SCION_HOST_UID`/`SCION_HOST_GID` pattern could be used. May need a chown step on container startup. |
| 8 | **Should shared dirs be included in `scion snapshot` / backup operations?** | If snapshot support is added, shared dirs may contain large caches that should be excluded by default. |

## References

- [Grove Mount Protection](.design/grove-mount-protection.md) — Related: agent isolation and mount security
- [GCS Volume Support](.design/initial-gcs-volume-support.md) — Prior art: volume type extension
- [Agent Config Flow](.design/agent-config-flow.md) — How agent configuration is resolved and merged
- `pkg/api/types.go` — `VolumeMount` struct definition
- `pkg/runtime/common.go` — `buildCommonRunArgs()` volume mounting logic
- `pkg/config/settings.go` — Settings struct and volume expansion
- `pkg/agent/run.go` — RunConfig construction and volume injection
