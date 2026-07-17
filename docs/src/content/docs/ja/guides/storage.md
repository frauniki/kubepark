---
title: ストレージ
description: per-sandbox の home PVC — 独立したライフサイクル、retain ポリシー、claim の再利用、multi-AZ の注意点。
---

各 sandbox は `kubepark-home-<name>` という名前の単一の **RWO** PersistentVolumeClaim を持ちます。これは Pod より長生きする 3 つのもの(RBAC とゲートウェイルートと並ぶ)の 1 つです。home PVC は sandbox の状態を保持し、そのライフサイクルは **Pod から独立**しています。sandbox のサスペンドは Pod を削除しますが PVC は残すため、再接続すると同じ home に戻ります。

## RWO で十分

sandbox は一度に 1 つの Pod を実行するため、ReadWriteOnce で十分です — **RWX は不要**です。サイズとクラスはテンプレートの `homeSize` / `storageClassName`、または Sandbox ごとに `home` で設定します。

```yaml
spec:
  home:
    size: 10Gi
    storageClassName: fast-ssd
    retainPolicy: Retain      # デフォルト
```

## retainPolicy: Retain と Delete

`home.retainPolicy` は、**Sandbox** が削除されたときに PVC がどうなるかを制御します。

- **Retain**(デフォルト): PVC は残されます。kubepark は owner との紐付けを剥がし、`kubepark.dev/orphaned-home=true` のラベルを付けます。孤児化した home を後で列挙・回収(あるいは掃除)しやすくなります。
- **Delete**: PVC は削除されます — ただし **kubepark が作成した場合のみ**です。kubepark は自身が作成していない PVC を決して削除しません。

## 既存の claim を再利用

`home.existingClaim` で Sandbox を既存の PVC に向けられます。

```yaml
spec:
  home:
    existingClaim: my-preseeded-home
```

2 つのガードが適用されます。

- kubepark が作成したものではないため、`existingClaim` と `retainPolicy: Delete` の併用は**バリデーションで拒否**されます — kubepark に、所有していないボリュームの削除を要求することになるためです。
- 1 つの claim は同時に 1 つの稼働 sandbox しか裏付けられません。同じ claim を参照する 2 つ目の Sandbox は、共有状態を壊す代わりに `ClaimInUse` condition で保留されます。

## スケジューリングと multi-AZ の注意点

RWO ボリュームは 1 つのゾーンに束縛され、sandbox Pod をそのゾーンに固定します。multi-AZ クラスタではこれが問題になります。

- `volumeBindingMode: WaitForFirstConsumer` の StorageClass を推奨します。Pod が実際にスケジュールされたゾーンでボリュームがプロビジョニングされます。即時バインドでは PVC が Pod の空き容量の無いゾーンに配置され、スケジュールできなくなることがあります。
- home を再作成せず拡張できるよう `allowVolumeExpansion: true` を推奨します。

これらは[インストール](/kubepark/ja/getting-started/installation/)で挙げたストレージ推奨と同じです。サスペンドと削除が PVC とどう相互作用するかは[状態機械](/kubepark/ja/design/state-machine/)を参照してください。
