---
title: Writing SandboxTemplates
description: Expressing real use cases in template vocabulary — bastions, ML clients and DB ops.
---

A `SandboxTemplate` (cluster-scoped, shortName `sbt`) is where all use-case diversity lives. The core operator only knows lifecycle, isolation, connectivity, permissions and audit; *what a sandbox is for* is entirely a matter of image, command, egress and — through a separate [AccessProfile](/kubepark/guides/access-profiles/) — RBAC.

## The vocabulary

| Field | Meaning |
| --- | --- |
| `image` | Container image for the environment |
| `command` | The long-running **main workload**. Interactive SSH always gets a login shell regardless. |
| `env`, `resources` | Standard environment and resource requests/limits |
| `isolationLevel` | `standard` or `strong` (strong requires `runtimeClassName`) |
| `homeSize`, `storageClassName` | Home PVC defaults |
| `egress` | Rendered into the sandbox `NetworkPolicy`, **additive** on top of built-in DNS + API-server egress |
| `defaultIdleTimeout` | Fallback idle timeout when a Sandbox does not set its own |
| `runAsUser` | Default `1000`; non-root is enforced |

Sandboxes are **clients** to GPU/job infrastructure — they never have GPUs themselves.

## Recipe: ops bastion

A bastion with `kubectl`, `k9s` and `stern`. Pair it with an AccessProfile that grants read access to the namespaces operators need.

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: ops}
spec:
  image: ghcr.io/example/ops-tools:latest
  command: ["sleep", "infinity"]
  isolationLevel: standard
  homeSize: 5Gi
  egress:
    - to: [{ipBlock: {cidr: 10.0.0.0/8}}]
      ports: [{protocol: TCP, port: 443}]
```

## Recipe: MLOps client

Slurm and CUDA **client** tools that submit and monitor jobs on external infrastructure. The sandbox holds no GPUs; egress reaches the job scheduler and object store.

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: ml-client}
spec:
  image: ghcr.io/example/ml-client:latest   # slurm client, cuda toolkit, aws/gcloud
  command: ["sleep", "infinity"]
  isolationLevel: standard
  homeSize: 20Gi
  egress:
    - to: [{ipBlock: {cidr: 10.20.0.0/16}}]   # Slurm / job infra
      ports: [{protocol: TCP, port: 6817}]
    - to: [{ipBlock: {cidr: 10.30.0.0/16}}]   # object store
      ports: [{protocol: TCP, port: 443}]
```

## Recipe: DB ops

Database clients with egress to the database subnet and credentials supplied through `env`.

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: db-ops}
spec:
  image: ghcr.io/example/db-clients:latest   # psql, mysql, redis-cli
  command: ["sleep", "infinity"]
  env:
    - name: PGHOST
      value: db.internal
  homeSize: 5Gi
  egress:
    - to: [{ipBlock: {cidr: 10.40.0.0/16}}]
      ports: [{protocol: TCP, port: 5432}]
```

## Standard vs strong isolation

`isolationLevel: standard` gives every sandbox a per-user namespace, a default-deny NetworkPolicy, non-root execution and `seccomp: RuntimeDefault`. For untrusted or higher-risk workloads, `isolationLevel: strong` selects a sandboxed RuntimeClass (gVisor or Kata) and **requires** a `runtimeClassName`. See the [security model](/kubepark/design/security-model/) for what each level buys you.

## Notes on egress and users

- `egress` is **additive**: built-in kube-dns and API-server egress are always present; your rules extend, not replace, them. Everything else is denied by default.
- Egress rules only take effect if your CNI enforces NetworkPolicy — confirm this during [installation](/kubepark/getting-started/installation/).
- `runAsUser` defaults to `1000` and non-root is enforced; build images that work as an unprivileged user.
