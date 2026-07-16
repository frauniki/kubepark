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



AccessProfile is the Schema for the accessprofiles API



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



AccessProfileSpec defines the desired state of AccessProfile



_Appears in:_
- [AccessProfile](#accessprofile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `foo` _string_ | foo is an example field of AccessProfile. Edit accessprofile_types.go to remove/update |  | Optional: \{\} <br /> |


#### AccessProfileStatus



AccessProfileStatus defines the observed state of AccessProfile.



_Appears in:_
- [AccessProfile](#accessprofile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the AccessProfile resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### Sandbox



Sandbox is the Schema for the sandboxes API



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


#### SandboxSession



SandboxSession is the Schema for the sandboxsessions API



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



SandboxSessionSpec defines the desired state of SandboxSession



_Appears in:_
- [SandboxSession](#sandboxsession)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `foo` _string_ | foo is an example field of SandboxSession. Edit sandboxsession_types.go to remove/update |  | Optional: \{\} <br /> |


#### SandboxSessionStatus



SandboxSessionStatus defines the observed state of SandboxSession.



_Appears in:_
- [SandboxSession](#sandboxsession)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the SandboxSession resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### SandboxSpec



SandboxSpec defines the desired state of Sandbox



_Appears in:_
- [Sandbox](#sandbox)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `foo` _string_ | foo is an example field of Sandbox. Edit sandbox_types.go to remove/update |  | Optional: \{\} <br /> |


#### SandboxStatus



SandboxStatus defines the observed state of Sandbox.



_Appears in:_
- [Sandbox](#sandbox)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the Sandbox resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### SandboxTemplate



SandboxTemplate is the Schema for the sandboxtemplates API



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



SandboxTemplateSpec defines the desired state of SandboxTemplate



_Appears in:_
- [SandboxTemplate](#sandboxtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `foo` _string_ | foo is an example field of SandboxTemplate. Edit sandboxtemplate_types.go to remove/update |  | Optional: \{\} <br /> |


#### SandboxTemplateStatus



SandboxTemplateStatus defines the observed state of SandboxTemplate.



_Appears in:_
- [SandboxTemplate](#sandboxtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | conditions represent the current state of the SandboxTemplate resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


