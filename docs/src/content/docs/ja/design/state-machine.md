---
title: Sandbox 状態機械
description: フェーズ・遷移・サスペンドで何が残るか。
---

Pending → Provisioning → Running ⇄ (Suspending → Suspended → Resuming)、加えて Failed / Terminating。ホーム PVC・ServiceAccount・RBAC・ホスト鍵はサスペンドを越えて維持され、削除されるのは Pod のみ。完全な遷移ドキュメントは M5 で。
