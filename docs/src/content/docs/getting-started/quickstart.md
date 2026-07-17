---
title: Quickstart
description: From helm install to your first sandbox shell in a handful of manifests.
---

This walks from a fresh install to an SSH shell inside your own sandbox. It assumes you have already completed [installation](/kubepark/getting-started/installation/) and have a NetworkPolicy-enforcing CNI.

## 1. Create a template

The `SandboxTemplate` describes the environment. `command` is the long-running main workload; an interactive SSH login always gets a login shell regardless of `command`.

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: ops}
spec:
  image: ghcr.io/example/ops-tools:latest   # e.g. kubectl, k9s, stern
  command: ["sleep", "infinity"]
  isolationLevel: standard
  homeSize: 5Gi
  egress:
    - to: [{ipBlock: {cidr: 10.0.0.0/8}}]
      ports: [{protocol: TCP, port: 443}]
```

## 2. (Optional) Grant cluster access

An `AccessProfile` declares what a sandbox may do. `allowedNamespaces` lists which namespaces' Sandboxes may *reference* this profile — this is the guard that makes referencing safe.

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: AccessProfile
metadata: {name: ml-debug}
spec:
  allowedNamespaces: [team-alice]
  grants:
    - namespaces: [ml-training]
      rules:
        - apiGroups: [""]
          resources: [pods, pods/log]
          verbs: [get, list]
        - apiGroups: [batch]
          resources: [cronjobs]
          verbs: [get, list, patch]
```

## 3. Create your sandbox

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: Sandbox
metadata: {name: demo, namespace: team-alice}
spec:
  template: ops
  accessProfile: ml-debug
  owner: {name: alice@example.com}
  idleTimeout: 30m
  exposedPorts:
    - {name: jupyter, port: 8888, auth: oidc}
```

Apply and watch it reach `Running`:

```sh
kubectl apply -f sandbox.yaml
kubectl -n team-alice get sandbox demo -w
```

The `owner.name` must match the principal on your certificate — it is what authorizes you to connect.

## 4. Log in and get a cert

```sh
kubepark login --gateway-url https://gateway:8080
```

This runs an OIDC auth-code + PKCE flow and writes a short-lived certificate (default 8h) to `~/.kubepark/`.

## 5. SSH into your sandbox

```sh
kubepark ssh demo -n team-alice --gateway gateway:2222
```

This writes `~/.kubepark/ssh_config` (ProxyJump + certificate + CA-verified `known_hosts`) and execs `ssh`. Afterwards plain tooling works too:

```sh
ssh -F ~/.kubepark/ssh_config demo.team-alice
```

`scp`, `rsync` and VS Code Remote-SSH all work against the same config.

## What happens when you disconnect

If you leave and no session stays active, the sandbox suspends after its `idleTimeout` (here 30m): the Pod is deleted but your home PVC, ServiceAccount and RBAC are kept. Reconnecting recreates the Pod and drops you back into the same home. See the [state machine](/kubepark/design/state-machine/) for the full lifecycle.
