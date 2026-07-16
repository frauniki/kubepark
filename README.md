# kubepark

**Persistent, declarative sandboxes for Kubernetes.**

kubepark manages "reachability into a cluster" — execution environment,
permissions and audit — as a declarative `Sandbox` resource. A Sandbox is not
a Pod: Pods are disposable executors, while the substance of the environment
(the home PVC, the RBAC grants, the gateway route) outlives them.

> Coder builds environments for IDEs. Teleport builds doors into existing
> environments. kubepark builds both — operations environments *and* their
> doors, inside Kubernetes, as CRDs.

## Status

Pre-release, under active development. Not ready for production use.

## Components

kubepark is deliberately small — one operator, one gateway, four CRDs:

| Component | Role |
|---|---|
| `Sandbox` | The only user-facing resource: desired state, idle timeout, exposed ports. Survives Pod death and suspension. |
| `SandboxTemplate` | Admin-defined classes (image, resources, isolation level, allowed egress). All use-case diversity lives here. |
| `AccessProfile` | Declarative cluster permissions for a sandbox, translated to ServiceAccounts and RBAC, bound to the owner's identity for audit. |
| Gateway | A single SSH/HTTP entry point. Authenticates short-lived, OIDC-issued SSH certificates and routes to sandbox pods — terminals, scp/rsync, VS Code Remote-SSH, JetBrains Gateway and port-forwarding all work through one `ProxyJump` line. |

Idle sandboxes are automatically suspended (Pod deleted, home PVC kept) and
resume on the next connection.

## Documentation

https://frauniki.github.io/kubepark/ — quickstart, installation, design docs
(architecture, security model, state machine) and the CRD API reference, in
English and Japanese.

## Development

```sh
make build      # build the server binary (operator|gateway|agent) and the CLI
make test       # unit + envtest
make lint       # golangci-lint (with the logcheck plugin)
make test-e2e   # e2e against a local kind cluster
```

See [AGENTS.md](AGENTS.md) for the project layout and contributor rules.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
