---
title: AccessProfiles
description: Declaring what a sandbox may do in the cluster — and why referencing a profile is the real attack surface.
---

An `AccessProfile` (cluster-scoped, shortName `ap`) declares the cluster permissions a sandbox may hold. When a Sandbox references a permitted profile, the operator mints a per-sandbox ServiceAccount plus the Roles and RoleBindings the profile describes.

## Grants

`grants` is a list of `{namespaces, rules}`. Each entry binds a set of `rbacv1.PolicyRule`s into an explicit list of namespaces:

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

**v1 supports namespaced grants only** — no cluster-wide rules, and no wildcards in `namespaces`. The profile's status carries a `Valid` condition.

## allowedNamespaces: the referencing guard

The subtle point of the whole design: the attack surface is **referencing** a powerful profile, not *creating* one. `allowedNamespaces` is an explicit list of the namespaces whose Sandboxes MAY reference this profile — and it is **default deny**.

A profile is honored only when its `allowedNamespaces` includes the referencing Sandbox's namespace. Otherwise the Sandbox gets `RBACReady=False` with reason `ProfileNotPermitted`, and **no credentials are minted at all**. If a referenced profile is later removed, the Sandbox surfaces `ProfileDeleted` and loses its minted credentials.

## Why authorship is admin-only

The operator ClusterRole necessarily holds the `escalate` verb on Roles — it has to, in order to mint Roles with arbitrary rules on your behalf. That means anyone who can author an `AccessProfile` can describe *any* set of permissions and have the operator grant them. **AccessProfile authorship is the platform's trust boundary and must be restricted to administrators.**

`allowedNamespaces` is what lets an admin author a powerful profile safely: the profile is inert until an admin also opts a specific namespace into referencing it.

## Audit

Every per-sandbox ServiceAccount is annotated with its owner identity and the profile it was minted from. Apiserver audit logs can therefore join "who did what, via which sandbox" — you can attribute a cluster action back to a human, not just to an anonymous ServiceAccount.

## See also

The referencing guard and the `escalate` verb are covered as trust boundaries in the [security model](/kubepark/design/security-model/); use profiles alongside the templates in [Writing SandboxTemplates](/kubepark/guides/templates/).
