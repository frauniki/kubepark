---
title: Architecture
description: Components and how a connection reaches a sandbox.
---

kubepark has exactly four moving parts: the **Sandbox**, **SandboxTemplate** and **AccessProfile** CRDs plus one **gateway**. A Sandbox is a persistent declarative resource; its Pod is a disposable executor. Detailed component documentation lands with milestone M5.
