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
	Resources  PodResources
	Containers []ContainerInfo
}

type ContainerInfo struct {
	Name       string
	Ready      bool
	State      string
	Restarts   int32
	Image      string
	CPUReq     string // e.g. "100m"
	CPULim     string // e.g. "500m"
	MemReq     string // e.g. "128Mi"
	MemLim     string // e.g. "512Mi"
}

type PodResources struct {
	CPUReq string
	CPULim string
	MemReq string
	MemLim string
}

type DeploymentInfo struct {
	Name      string
	Namespace string
	Ready     string // "3/3" format
	UpToDate  int32
	Available int32
	Desired   int32
	Age       time.Duration
	Strategy  string
}

type EventInfo struct {
	Type      string // Normal, Warning
	Reason    string
	Object    string
	Message   string
	Age       time.Duration
	Count     int32
	Namespace string
}

type PodMetrics struct {
	CPU    string // e.g. "120m"
	Memory string // e.g. "256Mi"
}
