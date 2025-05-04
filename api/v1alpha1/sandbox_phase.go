package v1alpha1

type SandboxPhase string

const (
	SandboxUnknown   SandboxPhase = ""
	SandboxPending   SandboxPhase = "Pending"
	SandboxRunning   SandboxPhase = "Running"
	SandboxFailed    SandboxPhase = "Failed"
	SandboxError     SandboxPhase = "Error"
	SandboxCompleted SandboxPhase = "Completed"
)
