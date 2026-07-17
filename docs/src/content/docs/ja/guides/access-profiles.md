---
title: AccessProfile
description: sandbox に許可するクラスタ操作の宣言 — そしてなぜプロファイルの参照こそが本当の攻撃面なのか。
---

`AccessProfile`(クラスタスコープ、shortName `ap`)は、sandbox が持ちうるクラスタ権限を宣言します。Sandbox が許可されたプロファイルを参照すると、オペレータは per-sandbox の ServiceAccount と、プロファイルが記述する Role/RoleBinding を発行します。

## grants

`grants` は `{namespaces, rules}` のリストです。各エントリは `rbacv1.PolicyRule` の集合を、明示的な namespace のリストにバインドします。

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

**v1 は namespaced な grant のみ**をサポートします — クラスタ全体のルールは無く、`namespaces` にワイルドカードも使えません。プロファイルの status は `Valid` condition を持ちます。

## allowedNamespaces: 参照ガード

設計全体の核心はここです。攻撃面は強力なプロファイルを*作成する*ことではなく、それを**参照する**ことです。`allowedNamespaces` は、このプロファイルを参照してよい Sandbox の namespace を明示的に列挙したもので、**デフォルト拒否**です。

プロファイルは、その `allowedNamespaces` に参照元 Sandbox の namespace が含まれる場合にのみ有効になります。そうでなければ Sandbox は `RBACReady=False`、reason は `ProfileNotPermitted` となり、**認証情報は一切発行されません**。参照していたプロファイルが後で削除された場合、Sandbox は `ProfileDeleted` を表面化し、発行済みの認証情報を失います。

## なぜ作成権は管理者限定なのか

オペレータ ClusterRole は Role に対する `escalate` verb を必然的に持ちます — あなたの代わりに任意のルールを持つ Role を発行するために、そうでなければなりません。つまり `AccessProfile` を作成できる者は、*任意の*権限集合を記述し、それをオペレータに付与させられます。**AccessProfile の作成権はプラットフォームの信頼境界であり、管理者に限定しなければなりません。**

`allowedNamespaces` こそが、管理者が強力なプロファイルを安全に作成できる仕組みです。プロファイルは、管理者が特定の namespace を参照対象として明示的にオプトインするまで不活性です。

## 監査

per-sandbox の ServiceAccount には、その owner identity と発行元プロファイルが annotation として付与されます。したがって apiserver の監査ログで「誰が、どの sandbox 経由で、何をしたか」を結合できます。クラスタ操作を匿名の ServiceAccount ではなく人間まで遡って帰属させられます。

## 関連

参照ガードと `escalate` verb は[セキュリティモデル](/kubepark/ja/design/security-model/)で信頼境界として扱っています。プロファイルは [SandboxTemplate を書く](/kubepark/ja/guides/templates/)のテンプレートと組み合わせて使ってください。
