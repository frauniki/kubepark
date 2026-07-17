---
title: SandboxTemplate を書く
description: 用途をテンプレートの語彙で表現する — 踏み台・ML クライアント・DB オペレーション。
---

`SandboxTemplate`(クラスタスコープ、shortName `sbt`)は、あらゆる用途の多様性が宿る場所です。コアオペレータが知るのはライフサイクル・分離・接続性・権限・監査のみで、*sandbox が何のためのものか*は、イメージ・command・egress、そして別立ての [AccessProfile](/kubepark/ja/guides/access-profiles/) による RBAC で完全に決まります。

## 語彙

| フィールド | 意味 |
| --- | --- |
| `image` | 環境のコンテナイメージ |
| `command` | 長時間動作する**メインワークロード**。対話的 SSH は常にログインシェルを得る。 |
| `env`, `resources` | 標準の環境変数とリソース requests/limits |
| `isolationLevel` | `standard` または `strong`(strong は `runtimeClassName` が必要) |
| `homeSize`, `storageClassName` | home PVC のデフォルト |
| `egress` | sandbox `NetworkPolicy` に描画。組み込み DNS + API-server egress に**加算的** |
| `defaultIdleTimeout` | Sandbox が自身で設定しない場合のフォールバック |
| `runAsUser` | デフォルト `1000`。非 root を強制 |

sandbox は GPU/ジョブ基盤に対する**クライアント**であり、それ自体が GPU を持つことはありません。

## レシピ: 運用踏み台

`kubectl`・`k9s`・`stern` を備えた踏み台。運用者が必要とする namespace への read 権限を付与する AccessProfile と組み合わせます。

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: ops}
spec:
  image: ghcr.io/example/ops-tools:latest
  command: ["sleep", "infinity"]
  isolationLevel: standard
  homeSize: 5Gi
  egress:
    - to: [{ipBlock: {cidr: 10.0.0.0/8}}]
      ports: [{protocol: TCP, port: 443}]
```

## レシピ: MLOps クライアント

外部基盤上のジョブを投入・監視する Slurm / CUDA の**クライアント**ツール。sandbox は GPU を持たず、egress はジョブスケジューラとオブジェクトストアに到達します。

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: ml-client}
spec:
  image: ghcr.io/example/ml-client:latest   # slurm client, cuda toolkit, aws/gcloud
  command: ["sleep", "infinity"]
  isolationLevel: standard
  homeSize: 20Gi
  egress:
    - to: [{ipBlock: {cidr: 10.20.0.0/16}}]   # Slurm / ジョブ基盤
      ports: [{protocol: TCP, port: 6817}]
    - to: [{ipBlock: {cidr: 10.30.0.0/16}}]   # オブジェクトストア
      ports: [{protocol: TCP, port: 443}]
```

## レシピ: DB オペレーション

DB クライアントに、データベースサブネットへの egress と `env` 経由の認証情報を与えます。

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: db-ops}
spec:
  image: ghcr.io/example/db-clients:latest   # psql, mysql, redis-cli
  command: ["sleep", "infinity"]
  env:
    - name: PGHOST
      value: db.internal
  homeSize: 5Gi
  egress:
    - to: [{ipBlock: {cidr: 10.40.0.0/16}}]
      ports: [{protocol: TCP, port: 5432}]
```

## standard と strong の分離

`isolationLevel: standard` はすべての sandbox に per-user namespace、デフォルト拒否の NetworkPolicy、非 root 実行、`seccomp: RuntimeDefault` を与えます。信頼できない、あるいはリスクの高いワークロードには `isolationLevel: strong` を選び、サンドボックス化された RuntimeClass(gVisor または Kata)を使います。これには `runtimeClassName` が**必要**です。各レベルが何をもたらすかは[セキュリティモデル](/kubepark/ja/design/security-model/)を参照してください。

## egress とユーザーに関する注意

- `egress` は**加算的**です。組み込みの kube-dns と API-server egress は常に存在し、ルールはそれを置き換えるのではなく拡張します。それ以外はデフォルトで拒否されます。
- egress ルールは CNI が NetworkPolicy を強制する場合にのみ有効です。[インストール](/kubepark/ja/getting-started/installation/)時に確認してください。
- `runAsUser` はデフォルト `1000` で非 root を強制します。非特権ユーザーで動くイメージを作成してください。
