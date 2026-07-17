---
title: セキュリティモデル
description: 信頼境界、SSH-CA による identity チェーン、権限昇格境界としての AccessProfile、そして v1 の正直な制限。
---

kubepark のセキュリティは、少数の明示的な信頼境界の上に成り立っています。コアオペレータが提供するのはライフサイクル・分離・接続性・権限注入・監査のみで、以下の各境界は慣習ではなくそのコアによって強制されます。

## Identity: 証明書ベースの SSH

ゲートウェイは user CA に対する**証明書認証のみ**を受け付けます。パスワードや素の公開鍵認証は拒否されます。証明書は principal を持ち、認可ルールは 1 つだけです。

> 証明書の principal は `sandbox.spec.owner.name` と一致しなければならない。

このチェックは**2 箇所**で強制されます。ゲートウェイと、Pod 内の in-pod agent です。この共有されたチェックがモデルの要石であり、仮にゲートウェイを回避されても、agent が独立して principal 不一致を拒否します。

証明書は短命(デフォルト **TTL 8h**)で、OIDC ログイン(`kubepark login`、auth-code + PKCE)または管理者によるオフライン署名(`kubepark admin sign-cert`)で発行されます。

## CA の保管

user CA の**秘密**鍵はオペレータ/ゲートウェイの namespace にある `kubepark-ca` Secret にのみ存在し、sandbox Pod には**決してマウントされません**。sandbox Pod が受け取るのは**公開**鍵のみで、user CA 公開鍵は per-sandbox のホスト鍵 Secret とともに配布されます。

各 sandbox のホスト鍵は自動ブートストラップされた**ホスト CA** で署名されるため、クライアントは `known_hosts` の `@cert-authority` 行 1 行で信頼できます。TOFU(trust-on-first-use)のプロンプトも、ホストごとの鍵ピン留めも不要です。

## AccessProfile が権限昇格境界

本質的な攻撃面は、強力な `AccessProfile` を*作成する*ことではなく、それを*参照する*ことです。プロファイルはその `allowedNamespaces` リストに参照元 Sandbox の namespace が含まれる場合にのみ有効になります。そうでなければ Sandbox は `RBACReady=False`、reason は `ProfileNotPermitted` となり、**認証情報は一切発行されません**(デフォルト拒否)。

オペレータ ClusterRole は Role に対する `escalate` verb を必然的に持ちます(任意のルールを持つ Role を発行する必要があるため)。だからこそ **AccessProfile の作成権は管理者に限定しなければなりません** — これがプラットフォームの真の権限境界です。

per-sandbox の ServiceAccount には owner とプロファイルが annotation として付与されるため、apiserver の監査ログで「誰が、どの sandbox 経由で、何をしたか」を結合できます。

## HTTP 公開ポート

公開ポートはホストでルーティングされます: `<port>--<sandbox>--<namespace>.<baseDomain>`。左詰めで解析し round-trip チェックを行うため、1 段分の wildcard DNS と wildcard TLS が必要です。

- `auth: oidc` は、identity が owner(または明示的な `allowedUsers` / `allowedGroups` エントリ)である認証済みブラウザセッション(OIDC cookie)を要求します。認証だけでは決して十分ではなく、認可も必ずチェックされます。
- `auth: none` は認証なしでプロキシされますが、**サスペンド中の sandbox を起こすことはなく**(`503` を返す)、**`SandboxSession` レコードを作成することもありません**。

## ベースライン分離と強分離

ベースライン分離はすべての sandbox に適用されます: per-user namespace、デフォルト拒否の `NetworkPolicy`(組み込みの kube-dns と API-server egress、テンプレートで宣言した egress のみ許可)、非 root 実行、`seccomp: RuntimeDefault`。

強分離は、テンプレートの `isolationLevel: strong` で選択されるサンドボックスランタイム(gVisor または Kata)を追加します。これには `runtimeClassName` が必要です。

## v1 の正直な制限

1. **owner なりすまし。** ある `owner` で namespace 内に Sandbox を作成できる者は、実質的に誰が接続できるかを制御できます — Sandbox 作成権はおおむね namespace の所有権に等しいです。`owner` を要求者に固定する validating webhook は将来対応予定です。
2. **per-user namespace 分離はオペレータがプロビジョニングしません。** これは管理者/GitOps の責務です。1 つの namespace を共有すると home/owner の境界が崩壊します。
3. **オペレータ ServiceAccount の侵害は cluster-admin 相当。** これは SA 注入方式に内在するものです。`SubjectAccessReview` の admission webhook が緩和策として計画されています。
4. **プロセスの不死は保証しません。** v1 に CRIU はありません。kubepark が保証するのは*環境と作業状態*の継続性であり、実行中プロセスの継続性ではありません。in-pod agent は tmux 風の再接続を提供し、切断は乗り越えますが Pod の死は乗り越えません。

サスペンドで何が残るかは[状態機械](/kubepark/ja/design/state-machine/)を、参照ガードの実際の運用は [AccessProfile](/kubepark/ja/guides/access-profiles/) を参照してください。
