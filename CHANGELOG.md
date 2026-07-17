# Changelog

## 1.0.0 (2026-07-17)


### Features

* add the SSH gateway, in-pod agent and client CLI (M3) ([c05a355](https://github.com/frauniki/kubepark/commit/c05a355a394ae4b2bc58333ac86f558fe1921e41))
* HTTP exposed ports, docs and a helm-based e2e (M5) ([206d25f](https://github.com/frauniki/kubepark/commit/206d25f4b2597a876e01c212fea3464bfcb073f7))
* idle suspension, wake-on-connect and OIDC login (M4) ([4dbf8d2](https://github.com/frauniki/kubepark/commit/4dbf8d2b75ddfa3bd50a5badc57a113b750eb1c7))
* implement Sandbox CRDs and the core provisioning controller ([194418f](https://github.com/frauniki/kubepark/commit/194418fcb411b8fccf87e6c7c184bd62634e5306))
* scaffold kubepark operator, gateway and CLI skeletons ([c21b764](https://github.com/frauniki/kubepark/commit/c21b7641f59dbaf4d386d521d32333a44a7c8932))
* translate AccessProfiles into per-sandbox ServiceAccounts and RBAC ([e2633c0](https://github.com/frauniki/kubepark/commit/e2633c087a0112f74b601d6a340c3b457083da9e))


### Bug Fixes

* use id -u instead of whoami in e2e ssh check ([24566a9](https://github.com/frauniki/kubepark/commit/24566a9ed66bddc4295121bb520033b95ae9dfd1))
