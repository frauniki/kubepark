load("ext://restart_process", "docker_build_with_restart")

DOCKERFILE = """FROM alpine:latest
WORKDIR /
COPY ./bin/manager /
CMD ["/manager"]
"""


def manifests():
    return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'


def generate():
    return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'


def binary():
    return "CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/manager cmd/main.go"


# Generate manifests and go files
local_resource(
    "make manifests",
    manifests(),
    deps=["api", "controllers", "hooks"],
    ignore=["*/*/zz_generated.deepcopy.go"],
)
local_resource(
    "make generate",
    generate(),
    deps=["api", "hooks"],
    ignore=["*/*/zz_generated.deepcopy.go"],
)

# Deploy CRD
local_resource(
    "CRD",
    manifests() + "kustomize build config/crd | kubectl apply -f -",
    deps=["api"],
    ignore=["*/*/zz_generated.deepcopy.go"],
)

# Deploy manager
watch_file("./config/")
k8s_yaml(kustomize("./config/dev"))

local_resource(
    "Watch & Compile",
    generate() + binary(),
    deps=["controllers", "api", "pkg", "cmd/main.go"],
    ignore=["*/*/zz_generated.deepcopy.go"],
)
docker_build_with_restart(
    "controller:latest",
    ".",
    dockerfile_contents=DOCKERFILE,
    entrypoint=["/manager"],
    only=["./bin/manager"],
    live_update=[
        sync("./bin/manager", "/manager"),
    ],
)

local_resource(
    "Sandbox Sample",
    "kubectl apply -f ./config/samples/kubepark_v1alpha1_sandbox.yaml",
    deps=["./config/samples/kubepark_v1alpha1_sandbox.yaml"],
)

local_resource(
    "SamdboxTemplate Sample",
    "kubectl apply -f ./config/samples/kubepark_v1alpha1_sandboxtemplate.yaml",
    deps=["./config/samples/kubepark_v1alpha1_sandboxtemplate.yaml"],
)
