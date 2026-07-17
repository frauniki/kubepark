---
title: Security model
description: Trust boundaries, the SSH-CA identity chain, the AccessProfile escalation boundary, and honest v1 limitations.
---

kubepark's security rests on a small number of explicit trust boundaries. The core operator provides only lifecycle, isolation, connectivity, permission injection and audit — every boundary below is enforced by that core, not by convention.

## Identity: certificate-based SSH

The gateway accepts **only certificate authentication** against a user CA. Password and raw public-key auth are rejected. A certificate carries a principal, and authorization is a single rule:

> the certificate principal must equal `sandbox.spec.owner.name`.

This check is enforced in **two** places — at the gateway and again inside the pod by the in-pod agent. That shared check is the keystone of the model: even if the gateway were bypassed, the agent independently refuses a mismatched principal.

Certificates are short-lived (default **8h TTL**) and are issued either through an OIDC login (`kubepark login`, auth-code + PKCE) or by an administrator signing offline (`kubepark admin sign-cert`).

## CA custody

The user CA **private** key lives only in the operator/gateway namespace, in the `kubepark-ca` Secret. It is **never** mounted into a sandbox pod. Sandbox pods receive only **public** key material: the user CA public key travels alongside the per-sandbox host-key Secret.

Each sandbox's host key is signed by an auto-bootstrapped **host CA**, so clients trust it via a single `@cert-authority` line in `known_hosts` — there is no trust-on-first-use prompt and no per-host key pinning.

## AccessProfile is the escalation boundary

The interesting attack surface is not *creating* a powerful `AccessProfile` — it is *referencing* one. A profile is only honored when its `allowedNamespaces` list includes the referencing Sandbox's namespace. Otherwise the Sandbox gets `RBACReady=False` with reason `ProfileNotPermitted` and **no credentials are minted at all** (default deny).

The operator ClusterRole necessarily holds the `escalate` verb on Roles (it must mint Roles with arbitrary rules). That is precisely why **AccessProfile authorship must be restricted to administrators** — it is the platform's real privilege boundary.

Every per-sandbox ServiceAccount is annotated with its owner and profile, so apiserver audit logs can join "who did what, via which sandbox."

## HTTP exposed ports

Exposed ports are routed by host: `<port>--<sandbox>--<namespace>.<baseDomain>`, parsed left-anchored with a round-trip check. This requires wildcard DNS and wildcard TLS one level deep.

- `auth: oidc` requires an authenticated browser session (OIDC cookie) whose identity is the owner, or an explicit `allowedUsers` / `allowedGroups` entry. Authentication alone is never sufficient — authorization is still checked.
- `auth: none` is proxied without auth, but it **never wakes a suspended sandbox** (it returns `503`) and **never creates a `SandboxSession`** record.

## Baseline and strong isolation

Baseline isolation applies to every sandbox: a per-user namespace, a default-deny `NetworkPolicy` (with only built-in kube-dns and API-server egress plus the template's declared egress), non-root execution, and `seccomp: RuntimeDefault`.

Strong isolation adds a sandboxed runtime (gVisor or Kata) selected through the template's `isolationLevel: strong`, which requires a `runtimeClassName`.

## Honest v1 limitations

1. **Owner spoofing.** Whoever can create a Sandbox in a namespace with a given `owner` effectively controls who may connect — Sandbox-create rights are roughly namespace ownership. A validating webhook that pins `owner` to the requester is a planned follow-up.
2. **Namespace-per-user isolation is not provisioned by the operator.** It is an admin/GitOps responsibility. Co-tenanting one namespace collapses the home/owner boundary.
3. **Operator ServiceAccount compromise equals cluster-admin.** This is inherent to the SA-injection method; a `SubjectAccessReview` admission webhook is the planned mitigation.
4. **Process immortality is not guaranteed.** There is no CRIU in v1. kubepark guarantees continuity of *environment and work state*, not of a running process. The in-pod agent provides tmux-style reconnect that survives disconnect — but not Pod death.

See the [state machine](/kubepark/design/state-machine/) for what survives a suspend, and [AccessProfiles](/kubepark/guides/access-profiles/) for the referencing guard in practice.
