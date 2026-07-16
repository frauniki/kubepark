---
title: Sandbox state machine
description: Phases, transitions and what survives suspension.
---

Pending → Provisioning → Running ⇄ (Suspending → Suspended → Resuming), plus Failed and Terminating. The home PVC, ServiceAccount, RBAC and host key survive suspension; only the Pod is deleted. Full transition documentation lands with milestone M5.
