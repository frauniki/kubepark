---
title: クイックスタート
description: helm install から、数枚のマニフェストで最初の sandbox シェルまで。
---

新規インストールから、自分の sandbox 内の SSH シェルまでの流れを説明します。[インストール](/kubepark/ja/getting-started/installation/)を完了し、NetworkPolicy を強制する CNI がある前提です。

## 1. テンプレートを作成

`SandboxTemplate` は環境を記述します。`command` は長時間動作するメインワークロードです。対話的な SSH ログインは `command` に関わらず常にログインシェルを得ます。

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: ops}
spec:
  image: ghcr.io/example/ops-tools:latest   # 例: kubectl, k9s, stern
  command: ["sleep", "infinity"]
  isolationLevel: standard
  homeSize: 5Gi
  egress:
    - to: [{ipBlock: {cidr: 10.0.0.0/8}}]
      ports: [{protocol: TCP, port: 443}]
```

## 2.(任意)クラスタアクセスを付与

`AccessProfile` は sandbox に許可する操作を宣言します。`allowedNamespaces` は、このプロファイルを*参照*してよい Sandbox の namespace を列挙します — これが参照を安全にするガードです。

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

## 3. sandbox を作成

```yaml
apiVersion: kubepark.dev/v1alpha1
kind: Sandbox
metadata: {name: demo, namespace: team-alice}
spec:
  template: ops
  accessProfile: ml-debug
  owner: {name: alice@example.com}
  idleTimeout: 30m
  exposedPorts:
    - {name: jupyter, port: 8888, auth: oidc}
```

apply して `Running` になるまで watch します。

```sh
kubectl apply -f sandbox.yaml
kubectl -n team-alice get sandbox demo -w
```

`owner.name` は証明書の principal と一致していなければなりません。これが接続を認可します。

## 4. ログインして証明書を取得

```sh
kubepark login --gateway-url https://gateway:8080
```

OIDC の auth-code + PKCE フローを実行し、短命の証明書(デフォルト 8h)を `~/.kubepark/` に書き込みます。

## 5. sandbox に SSH

```sh
kubepark ssh demo -n team-alice --gateway gateway:2222
```

これは `~/.kubepark/ssh_config`(ProxyJump + 証明書 + CA 検証済み `known_hosts`)を書き込み、`ssh` を exec します。以降は素のツールも動作します。

```sh
ssh -F ~/.kubepark/ssh_config demo.team-alice
```

`scp`・`rsync`・VS Code Remote-SSH も同じ設定で動作します。

## 切断したときに起きること

離席して Active なセッションが残らない場合、sandbox は `idleTimeout`(ここでは 30m)後にサスペンドします。Pod は削除されますが home PVC・ServiceAccount・RBAC は残ります。再接続すると Pod が再作成され、同じ home に戻ります。ライフサイクル全体は[状態機械](/kubepark/ja/design/state-machine/)を参照してください。
