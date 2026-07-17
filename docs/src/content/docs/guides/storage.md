---
title: Storage
description: The per-sandbox home PVC — its independent lifecycle, retain policies, claim reuse and multi-AZ caveats.
---

Each sandbox gets a single **RWO** PersistentVolumeClaim named `kubepark-home-<name>`. This is one of the three things that outlive the Pod (alongside RBAC and the gateway route): the home PVC holds the sandbox's state, and its lifecycle is **independent of the Pod**. Suspending a sandbox deletes the Pod but keeps the PVC, so reconnecting drops you back into the same home.

## RWO is enough

Because a sandbox runs one Pod at a time, ReadWriteOnce is sufficient — **RWX is not required**. Configure the size and class through the template's `homeSize` / `storageClassName`, or per-Sandbox through `home`:

```yaml
spec:
  home:
    size: 10Gi
    storageClassName: fast-ssd
    retainPolicy: Retain      # default
```

## retainPolicy: Retain vs Delete

`home.retainPolicy` governs what happens to the PVC when the **Sandbox** is deleted:

- **Retain** (default): the PVC is kept. kubepark strips its owner linkage and labels it `kubepark.dev/orphaned-home=true`, so orphaned homes are easy to list and reclaim (or clean up) later.
- **Delete**: the PVC is deleted — but **only if kubepark created it**. kubepark never deletes a PVC it did not create.

## Reusing an existing claim

Point a Sandbox at a pre-existing PVC with `home.existingClaim`:

```yaml
spec:
  home:
    existingClaim: my-preseeded-home
```

Two guards apply:

- Because kubepark did not create it, `existingClaim` together with `retainPolicy: Delete` is **rejected by validation** — it would ask kubepark to delete a volume it does not own.
- A claim can only back one live sandbox at a time; a second Sandbox referencing the same claim is held with a `ClaimInUse` condition rather than corrupting shared state.

## Scheduling and multi-AZ caveats

An RWO volume is bound to one zone, which pins the sandbox Pod to that zone. In a multi-AZ cluster this matters:

- Prefer a StorageClass with `volumeBindingMode: WaitForFirstConsumer`, so the volume is provisioned in the zone where the Pod is actually scheduled. With immediate binding the PVC may land in a zone with no capacity for the Pod, and it will not schedule.
- `allowVolumeExpansion: true` is recommended so a home can grow without recreation.

These are the same storage recommendations called out during [installation](/kubepark/getting-started/installation/). For how suspension and deletion interact with the PVC, see the [state machine](/kubepark/design/state-machine/).
