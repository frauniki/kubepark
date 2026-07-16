---
title: API Reference
editUrl: false
tableOfContents:
  maxHeadingLevel: 4
---

## Packages
- [kubepark.dev/v1alpha1](#kubeparkdevv1alpha1)


## kubepark.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the  v1alpha1 API group.

### Resource Types
- [AccessProfile](#accessprofile)
- [AccessProfileList](#accessprofilelist)
- [Sandbox](#sandbox)
- [SandboxList](#sandboxlist)
- [SandboxSession](#sandboxsession)
- [SandboxSessionList](#sandboxsessionlist)
- [SandboxTemplate](#sandboxtemplate)
- [SandboxTemplateList](#sandboxtemplatelist)



#### AccessProfile



AccessProfile declares which Kubernetes operations a sandbox may perform.
The controller translates it into a per-namespace Role plus a per-sandbox
RoleBinding for the sandbox's ServiceAccount.



_Appears in:_
- [AccessProfileList](#accessprofilelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `AccessProfile` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[AccessProfileSpec](#accessprofilespec)_ | spec defines the desired state of AccessProfile |  | Required: \{\} <br /> |
| `status` _[AccessProfileStatus](#accessprofilestatus)_ | status defines the observed state of AccessProfile |  | Optional: \{\} <br /> |


#### AccessProfileList



AccessProfileList contains a list of AccessProfile





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `AccessProfileList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[AccessProfile](#accessprofile) array_ |  |  |  |


#### AccessProfileSpec



AccessProfileSpec defines the desired state of AccessProfile.

AccessProfiles are the trust boundary of kubepark: whoever can create or
modify them controls what sandboxes may do in the cluster. Their creation
must be restricted to administrators.



_Appears in:_
- [AccessProfile](#accessprofile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `grants` _[NamespacedGrant](#namespacedgrant) array_ | Grants are the permissions this profile bestows on a sandbox's<br />ServiceAccount. |  | MinItems: 1 <br /> |
| `allowedNamespaces` _string array_ | AllowedNamespaces is the explicit list of namespaces whose Sandboxes<br />may reference this profile. A Sandbox in any other namespace is<br />refused (RBACReady=False, reason ProfileNotPermitted). Empty means no<br />namespace may use the profile — referencing, not creation, is the<br />escalation surface, so the default is deny. |  | Optional: \{\} <br /> |


#### AccessProfileStatus



AccessProfileStatus defines the observed state of AccessProfile.



_Appears in:_
- [AccessProfile](#accessprofile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the AccessProfile resource. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ |  |  | Optional: \{\} <br /> |


#### AuthMode

_Underlying type:_ _string_

AuthMode selects how an exposed HTTP port is authenticated at the gateway.

_Validation:_
- Enum: [oidc none]

_Appears in:_
- [ExposedPort](#exposedport)

| Field | Description |
| --- | --- |
| `oidc` | AuthModeOIDC requires an OIDC login at the gateway; only the sandbox<br />owner (and any allowedUsers/allowedGroups) may access the port.<br /> |
| `none` | AuthModeNone proxies without authentication. Unauthenticated traffic<br />never wakes a suspended sandbox and never creates SandboxSessions.<br /> |


#### DesiredState

_Underlying type:_ _string_

DesiredState is the state the owner wants the sandbox to be in.

_Validation:_
- Enum: [Running Stopped]

_Appears in:_
- [SandboxSpec](#sandboxspec)

| Field | Description |
| --- | --- |
| `Running` | DesiredStateRunning requests a schedulable, connectable sandbox pod.<br /> |
| `Stopped` | DesiredStateStopped requests suspension: the pod is deleted while the<br />home volume, permissions and host key are kept for a fast resume.<br /> |


#### EgressRule



EgressRule is a small vocabulary mapping 1:1 onto a
NetworkPolicyEgressRule. Template egress is additive on top of the
built-in allowances (DNS and the Kubernetes API server).



_Appears in:_
- [SandboxTemplateSpec](#sandboxtemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `to` _[NetworkPolicyPeer](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#networkpolicypeer-v1-networking) array_ | To lists the destinations this rule allows. |  | Optional: \{\} <br /> |
| `ports` _[NetworkPolicyPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#networkpolicyport-v1-networking) array_ | Ports restricts the rule to specific ports. |  | Optional: \{\} <br /> |


#### ExposedPort



ExposedPort declares an HTTP port on the sandbox that the gateway routes
to via host-based routing (<port>--<sandbox>--<namespace>.<baseDomain>).



_Appears in:_
- [SandboxSpec](#sandboxspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the routing key; it becomes the first label segment of the<br />hostname. Must be a DNS label without consecutive hyphens so the<br />hostname parse stays unambiguous. |  | MaxLength: 15 <br />Pattern: `^[a-z]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `port` _integer_ | Port is the container port to proxy to. |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `auth` _[AuthMode](#authmode)_ | Auth selects gateway authentication for this port. |  | Enum: [oidc none] <br /> |
| `allowedUsers` _string array_ | AllowedUsers optionally grants access to OIDC identities besides the<br />owner. Only meaningful with auth: oidc. |  | Optional: \{\} <br /> |
| `allowedGroups` _string array_ | AllowedGroups optionally grants access to OIDC groups besides the<br />owner. Only meaningful with auth: oidc. |  | Optional: \{\} <br /> |


#### HomeSpec



HomeSpec configures the sandbox home volume.



_Appears in:_
- [SandboxSpec](#sandboxspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | Size overrides the template's homeSize for the created PVC. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName overrides the template's storage class. |  | Optional: \{\} <br /> |
| `existingClaim` _string_ | ExistingClaim mounts an existing PVC as the home instead of creating<br />one. The claim must not be in use by another non-suspended Sandbox. |  | Optional: \{\} <br /> |
| `retainPolicy` _[RetainPolicy](#retainpolicy)_ | RetainPolicy controls the PVC's fate on Sandbox deletion.<br />Defaults to Retain. | Retain | Enum: [Retain Delete] <br />Optional: \{\} <br /> |


#### IsolationLevel

_Underlying type:_ _string_

IsolationLevel selects how strongly the sandbox pod is isolated from the
node.

_Validation:_
- Enum: [standard strong]

_Appears in:_
- [SandboxTemplateSpec](#sandboxtemplatespec)

| Field | Description |
| --- | --- |
| `standard` | IsolationStandard uses the baseline: non-root, seccomp RuntimeDefault,<br />per-sandbox NetworkPolicy.<br /> |
| `strong` | IsolationStrong additionally runs the pod under the template's<br />RuntimeClass (e.g. gVisor or Kata).<br /> |


#### NamespacedGrant



NamespacedGrant grants RBAC rules in an explicit list of namespaces.
v1 deliberately supports namespaced grants only (no cluster-wide rules).



_Appears in:_
- [AccessProfileSpec](#accessprofilespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `namespaces` _string array_ | Namespaces is the explicit list of namespaces the rules apply in.<br />Wildcards are not supported. |  | MinItems: 1 <br /> |
| `rules` _[PolicyRule](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#policyrule-v1-rbac) array_ | Rules are standard RBAC policy rules, applied verbatim as a Role in<br />each listed namespace. |  | MinItems: 1 <br /> |


#### OwnerSpec



OwnerSpec identifies the human owner of the sandbox. The name is the OIDC
claim value (default: email) that must appear as the principal of the SSH
certificate presented at the gateway.



_Appears in:_
- [SandboxSpec](#sandboxspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the OIDC identity (certificate principal) of the owner. |  | MinLength: 1 <br /> |
| `groups` _string array_ | Groups optionally records the owner's OIDC groups. |  | Optional: \{\} <br /> |


#### RetainPolicy

_Underlying type:_ _string_

RetainPolicy controls what happens to the home volume when the Sandbox is
deleted.

_Validation:_
- Enum: [Retain Delete]

_Appears in:_
- [HomeSpec](#homespec)

| Field | Description |
| --- | --- |
| `Retain` | RetainPolicyRetain keeps the PVC after Sandbox deletion (default). The<br />controller strips its owner linkage and labels it as an orphaned home.<br /> |
| `Delete` | RetainPolicyDelete deletes the PVC together with the Sandbox. Only<br />valid for PVCs created by kubepark (not with home.existingClaim).<br /> |


#### Sandbox



Sandbox is a persistent, declarative workspace. Its pod is a disposable
executor: the home volume, permissions and gateway route outlive it.



_Appears in:_
- [SandboxList](#sandboxlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `Sandbox` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[SandboxSpec](#sandboxspec)_ | spec defines the desired state of Sandbox |  | Required: \{\} <br /> |
| `status` _[SandboxStatus](#sandboxstatus)_ | status defines the observed state of Sandbox |  | Optional: \{\} <br /> |


#### SandboxList



SandboxList contains a list of Sandbox





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `SandboxList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Sandbox](#sandbox) array_ |  |  |  |


#### SandboxPhase

_Underlying type:_ _string_

SandboxPhase is a coarse, derived summary of the sandbox state. The
conditions are the source of truth.

_Validation:_
- Enum: [Pending Provisioning Running Suspending Suspended Resuming Failed Terminating]

_Appears in:_
- [SandboxStatus](#sandboxstatus)

| Field | Description |
| --- | --- |
| `Pending` |  |
| `Provisioning` |  |
| `Running` |  |
| `Suspending` |  |
| `Suspended` |  |
| `Resuming` |  |
| `Failed` |  |
| `Terminating` |  |


#### SandboxSession



SandboxSession is the short-lived audit record of one connection to a
sandbox through the gateway.



_Appears in:_
- [SandboxSessionList](#sandboxsessionlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `SandboxSession` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[SandboxSessionSpec](#sandboxsessionspec)_ | spec defines the desired state of SandboxSession |  | Required: \{\} <br /> |
| `status` _[SandboxSessionStatus](#sandboxsessionstatus)_ | status defines the observed state of SandboxSession |  | Optional: \{\} <br /> |


#### SandboxSessionList



SandboxSessionList contains a list of SandboxSession





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `SandboxSessionList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[SandboxSession](#sandboxsession) array_ |  |  |  |


#### SandboxSessionSpec



SandboxSessionSpec defines the desired state of SandboxSession.
Sessions are created by the gateway, one per authenticated connection
(ssh) or per (sandbox, user) sliding window (http). They are the audit
record of who reached which sandbox from where.



_Appears in:_
- [SandboxSession](#sandboxsession)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sandboxName` _string_ | SandboxName is the sandbox this session connects to (same namespace). |  | MinLength: 1 <br /> |
| `user` _string_ | User is the authenticated identity (SSH certificate principal or OIDC<br />claim). |  | MinLength: 1 <br /> |
| `clientAddr` _string_ | ClientAddr is the remote address the connection came from. |  | Optional: \{\} <br /> |
| `kind` _[SessionKind](#sessionkind)_ | Kind is ssh or http. |  | Enum: [ssh http] <br /> |
| `certSerial` _string_ | CertSerial is the serial of the SSH certificate used, for joining<br />with signing audit logs. |  | Optional: \{\} <br /> |
| `heartbeatInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | HeartbeatInterval is stamped by the gateway at creation so the stale<br />reaper can compute its threshold without re-deriving the sandbox's<br />idle timeout. |  | Optional: \{\} <br /> |


#### SandboxSessionStatus



SandboxSessionStatus defines the observed state of SandboxSession.



_Appears in:_
- [SandboxSession](#sandboxsession)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[SessionState](#sessionstate)_ | State is Active while the connection lives, then Closed. |  | Enum: [Active Closed] <br />Optional: \{\} <br /> |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `lastActivityTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastActivityTime is refreshed by gateway heartbeats. |  | Optional: \{\} <br /> |
| `endTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `exitReason` _string_ | ExitReason records why the session closed (Disconnected,<br />StaleHeartbeat, SandboxDeleted). |  | Optional: \{\} <br /> |


#### SandboxSpec



SandboxSpec defines the desired state of Sandbox.



_Appears in:_
- [Sandbox](#sandbox)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _string_ | Template names the cluster-scoped SandboxTemplate this sandbox is<br />built from. |  | MinLength: 1 <br /> |
| `accessProfile` _string_ | AccessProfile optionally names a cluster-scoped AccessProfile whose<br />grants are translated into RBAC for this sandbox's ServiceAccount.<br />Empty means the sandbox gets no Kubernetes API credentials. |  | Optional: \{\} <br /> |
| `owner` _[OwnerSpec](#ownerspec)_ | Owner is the identity allowed to connect to this sandbox. |  |  |
| `desiredState` _[DesiredState](#desiredstate)_ | DesiredState is Running (default) or Stopped (suspended: pod deleted,<br />home and permissions kept). | Running | Enum: [Running Stopped] <br />Optional: \{\} <br /> |
| `idleTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | IdleTimeout suspends the sandbox after this duration without active<br />sessions. Unset inherits the template default; 0 disables idle<br />suspension. |  | Optional: \{\} <br /> |
| `exposedPorts` _[ExposedPort](#exposedport) array_ | ExposedPorts are HTTP ports routed by the gateway. |  | Optional: \{\} <br /> |
| `home` _[HomeSpec](#homespec)_ | Home configures the home volume. |  | Optional: \{\} <br /> |


#### SandboxStatus



SandboxStatus defines the observed state of Sandbox.



_Appears in:_
- [Sandbox](#sandbox)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[SandboxPhase](#sandboxphase)_ | Phase is a derived one-word summary; conditions are authoritative. |  | Enum: [Pending Provisioning Running Suspending Suspended Resuming Failed Terminating] <br />Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the Sandbox resource. |  | Optional: \{\} <br /> |
| `podName` _string_ | PodName is the current executor pod, if any. |  | Optional: \{\} <br /> |
| `pvcName` _string_ | PVCName is the home volume claim in use. |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | ServiceAccountName is the per-sandbox SA carrying AccessProfile<br />grants. |  | Optional: \{\} <br /> |
| `podIP` _string_ | PodIP is the routing target for the gateway. Cleared while the<br />sandbox is suspending or suspended. |  | Optional: \{\} <br /> |
| `templateHash` _string_ | TemplateHash pins the hash of the template spec the current pod was<br />built from. Template changes never restart a running pod; they apply<br />on the next resume. |  | Optional: \{\} <br /> |
| `lastActivityTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastActivityTime is initialized when the sandbox becomes Running and<br />updated when sessions close. It drives idle suspension. |  | Optional: \{\} <br /> |
| `activeSessions` _integer_ | ActiveSessions is display-only; the suspend decision is always<br />computed from the live SandboxSession list. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ |  |  | Optional: \{\} <br /> |


#### SandboxTemplate



SandboxTemplate is an admin-defined sandbox class. All use-case diversity
(ops bastion, MLOps client, DB operations) is expressed here; the
controller has no use-case-specific logic.



_Appears in:_
- [SandboxTemplateList](#sandboxtemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `SandboxTemplate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[SandboxTemplateSpec](#sandboxtemplatespec)_ | spec defines the desired state of SandboxTemplate |  | Required: \{\} <br /> |
| `status` _[SandboxTemplateStatus](#sandboxtemplatestatus)_ | status defines the observed state of SandboxTemplate |  | Optional: \{\} <br /> |


#### SandboxTemplateList



SandboxTemplateList contains a list of SandboxTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubepark.dev/v1alpha1` | | |
| `kind` _string_ | `SandboxTemplateList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[SandboxTemplate](#sandboxtemplate) array_ |  |  |  |


#### SandboxTemplateSpec



SandboxTemplateSpec defines the desired state of SandboxTemplate.



_Appears in:_
- [SandboxTemplate](#sandboxtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the sandbox container image. Its ENTRYPOINT is not used: the<br />operator wraps Command with the kubepark agent (see Command). |  | MinLength: 1 <br /> |
| `command` _string array_ | Command is the long-running main process, executed as a child of the<br />kubepark agent (which is PID 1). If empty, the agent runs alone and<br />spawns a login shell per connection. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | Env is added to the sandbox container. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the container resource requirements. |  | Optional: \{\} <br /> |
| `isolationLevel` _[IsolationLevel](#isolationlevel)_ | IsolationLevel defaults to standard. | standard | Enum: [standard strong] <br />Optional: \{\} <br /> |
| `runtimeClassName` _string_ | RuntimeClassName is required when isolationLevel is strong. |  | Optional: \{\} <br /> |
| `homeSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | HomeSize is the default size of the per-sandbox home PVC. |  |  |
| `storageClassName` _string_ | StorageClassName is the default storage class for home PVCs. |  | Optional: \{\} <br /> |
| `egress` _[EgressRule](#egressrule) array_ | Egress is rendered into the sandbox NetworkPolicy in addition to the<br />built-in DNS and API-server allowances. Everything else is denied. |  | Optional: \{\} <br /> |
| `defaultIdleTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DefaultIdleTimeout applies to sandboxes that do not set idleTimeout.<br />Zero or unset disables idle suspension by default. |  | Optional: \{\} <br /> |
| `runAsUser` _integer_ | RunAsUser is the UID of the sandbox user. Defaults to 1000; root is<br />not allowed. | 1000 | Minimum: 1 <br />Optional: \{\} <br /> |


#### SandboxTemplateStatus



SandboxTemplateStatus defines the observed state of SandboxTemplate.



_Appears in:_
- [SandboxTemplate](#sandboxtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the SandboxTemplate resource. |  | Optional: \{\} <br /> |


#### SessionKind

_Underlying type:_ _string_

SessionKind is the transport of a session.

_Validation:_
- Enum: [ssh http]

_Appears in:_
- [SandboxSessionSpec](#sandboxsessionspec)

| Field | Description |
| --- | --- |
| `ssh` |  |
| `http` |  |


#### SessionState

_Underlying type:_ _string_

SessionState is the lifecycle state of a session.

_Validation:_
- Enum: [Active Closed]

_Appears in:_
- [SandboxSessionStatus](#sandboxsessionstatus)

| Field | Description |
| --- | --- |
| `Active` |  |
| `Closed` |  |


