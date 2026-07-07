package k8s

import "time"

type PodInfo struct {
	Name       string
	Namespace  string
	Status     string // Derived display status (Running, CrashLoopBackOff, etc.)
	Ready      string // "2/3" format
	Restarts   int32
	Age        time.Duration
	Node       string
	CPU        string // "120m" or ""
	Memory     string // "256Mi" or ""
	Deployment string // owning deployment name, "" if not owned by one
	Resources  PodResources
	Containers []ContainerInfo
	OOMKilled  bool // true if any container was OOMKilled (current or last state)
	Crashed    bool // true if any container terminated with non-zero exit (not OOMKilled)
}

type ContainerInfo struct {
	Name           string
	Ready          bool
	State          string
	Restarts       int32
	Image          string
	CPUReq         string    // e.g. "100m"
	CPULim         string    // e.g. "500m"
	MemReq         string    // e.g. "128Mi"
	MemLim         string    // e.g. "512Mi"
	LastTermReason string    // Last terminated reason (e.g. "OOMKilled")
	LastTermCode   int32     // Last terminated exit code
	LastTermAt     string    // When it was last terminated, relative ("5m ago")
	LastTermTime   time.Time // Actual termination timestamp, zero if unknown
}

type OOMEvent struct {
	PodName       string
	Namespace     string
	ContainerName string
	Restarts      int32
	MemLim        string // Memory limit when OOMKilled
	Ago           string // How long ago
}

type CrashEvent struct {
	PodName       string
	Namespace     string
	ContainerName string
	Restarts      int32
	ExitCode      int32
	Ago           string // How long ago
}

type PodResources struct {
	CPUReq string
	CPULim string
	MemReq string
	MemLim string

	// Raw byte limit kept alongside the formatted MemLim string so utilization
	// math doesn't have to re-parse a rounded display value.
	MemLimitBytes int64
}

type DeploymentInfo struct {
	Name       string
	Namespace  string
	Ready      string // "3/3" format
	UpToDate   int32
	Available  int32
	Desired    int32
	Age        time.Duration
	LastDeploy time.Duration // time since last rollout (newest ReplicaSet)
	Strategy   string
	Images     []string // container images from pod template
}

type EventInfo struct {
	Type      string // Normal, Warning
	Reason    string
	Object    string
	Message   string
	Age       time.Duration
	Timestamp time.Time // When the event last occurred, zero if unknown
	Count     int32
	Namespace string
}

type PodMetrics struct {
	CPU    string // e.g. "120m"
	Memory string // e.g. "256Mi"

	// Raw memory usage in bytes, before formatting lost precision.
	MemoryBytes int64
}
