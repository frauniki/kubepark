---
title: インストール
description: 前提条件、Helm でのインストール、そして設定する主要な values。
---

kubepark は単一のオペレータと単一のゲートウェイ Deployment(ゲートウェイはオペレータの ServiceAccount と RBAC を再利用します)、加えて 4 つの CRD としてインストールされます。CA とゲートウェイのホスト鍵は初回起動時に自動生成されるため、**手動ブートストラップは不要**です。

## 前提条件

- **NetworkPolicy を強制する CNI** — 例えば Calico や Cilium。素の kindnet は NetworkPolicy を**強制しません**。そのため分離ルールと egress ルールが黙って no-op になります。これはセキュリティモデルが成立するための最重要の前提条件です。
- **HTTP 公開ポートを使う場合:** ベースドメインの wildcard DNS と wildcard TLS(ルーティングは `<port>--<sandbox>--<namespace>.<baseDomain>` の 1 段分)。
- **ストレージ:** StorageClass。RWO で十分です。binding mode は `WaitForFirstConsumer` を推奨し、`allowVolumeExpansion: true` を推奨します。

## Helm でインストール

```sh
helm install kubepark oci://ghcr.io/frauniki/charts/kubepark \
  -n kubepark-system --create-namespace
```

ローカルのチャートディレクトリからもインストールできます。

```sh
helm install kubepark ./charts/kubepark \
  -n kubepark-system --create-namespace
```

これによりオペレータ、ゲートウェイ、そして `Sandbox`・`SandboxTemplate`・`AccessProfile`・`SandboxSession` の CRD がデプロイされます。

## 主要な Helm values

| Value | 目的 | デフォルト |
| --- | --- | --- |
| `image.repository` / `image.tag` | オペレータとゲートウェイのイメージ | チャート既定 |
| `gateway.service.type` | `LoadBalancer` / `NodePort` / `ClusterIP` | `LoadBalancer` |
| `gateway.sshPort` | SSH リスナー | `2222` |
| `gateway.httpPort` | HTTP リスナー | `8080` |
| `gateway.baseDomain` | HTTP 公開ポート用のベースドメイン | — |
| `oidc.issuer` | OIDC issuer URL | — |
| `oidc.clientID` | OIDC クライアント ID | — |
| `oidc.principalClaim` | SSH principal として使う claim | `email` |
| `crds.enabled` / `crds.keep` | CRD のインストール/保持 | — |

オペレータには `--agent-image`(kubepark イメージそのもの)も必要です。各 sandbox Pod に in-pod agent を注入する init コンテナで使われます。

## 確認

```sh
kubectl -n kubepark-system get deploy
kubectl get crds | grep kubepark.dev
```

オペレータとゲートウェイが `Available` になったら、[クイックスタート](/kubepark/ja/getting-started/quickstart/)に進んで最初の sandbox を作成してください。
