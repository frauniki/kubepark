---
title: Installation
description: Prerequisites, Helm installation and the key values you will set.
---

kubepark installs as a single operator plus a single gateway Deployment (the gateway reuses the operator's ServiceAccount and RBAC) plus four CRDs. The CA and the gateway host key are auto-generated on first start, so there is **zero manual bootstrap**.

## Prerequisites

- **A CNI that enforces NetworkPolicy** — e.g. Calico or Cilium. Plain kindnet does **not** enforce NetworkPolicy, so isolation and egress rules silently no-op there. This is the single most important prerequisite for the security model to hold.
- **For HTTP exposed ports:** wildcard DNS and wildcard TLS for your base domain (routing is `<port>--<sandbox>--<namespace>.<baseDomain>`, one level deep).
- **Storage:** a StorageClass. RWO is fine; `WaitForFirstConsumer` binding mode is recommended, and `allowVolumeExpansion: true` is recommended.

## Install with Helm

```sh
helm install kubepark oci://ghcr.io/frauniki/charts/kubepark \
  -n kubepark-system --create-namespace
```

You can also install from the local chart directory:

```sh
helm install kubepark ./charts/kubepark \
  -n kubepark-system --create-namespace
```

This deploys the operator, the gateway, and the `Sandbox`, `SandboxTemplate`, `AccessProfile` and `SandboxSession` CRDs.

## Key Helm values

| Value | Purpose | Default |
| --- | --- | --- |
| `image.repository` / `image.tag` | Operator and gateway image | chart default |
| `gateway.service.type` | `LoadBalancer`, `NodePort` or `ClusterIP` | `LoadBalancer` |
| `gateway.sshPort` | SSH listener | `2222` |
| `gateway.httpPort` | HTTP listener | `8080` |
| `gateway.baseDomain` | Base domain for HTTP exposed ports | — |
| `oidc.issuer` | OIDC issuer URL | — |
| `oidc.clientID` | OIDC client ID | — |
| `oidc.principalClaim` | Claim used as the SSH principal | `email` |
| `crds.enabled` / `crds.keep` | Install / retain CRDs | — |

The operator also needs an `--agent-image` (the kubepark image itself): it is used by the init container that injects the in-pod agent into each sandbox pod.

## Verify

```sh
kubectl -n kubepark-system get deploy
kubectl get crds | grep kubepark.dev
```

Once the operator and gateway are `Available`, continue to the [quickstart](/kubepark/getting-started/quickstart/) to create your first sandbox.
