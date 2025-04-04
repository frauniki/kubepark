package v1alpha1

type SandboxPhase string

const (
	SandboxUnknown   SandboxPhase = ""
	SandboxPending   SandboxPhase = "Pending"
	SnadboxRunning   SandboxPhase = "Running"
	SandboxFailed    SandboxPhase = "Failed"
	SandboxError     SandboxPhase = "Error"
	SandboxCompleted SandboxPhase = "Completed"
)
