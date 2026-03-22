package main

import (
	"io"

	"github.com/tituscarl/kwatch/cmd"
	"k8s.io/klog/v2"
)

func main() {
	klog.SetOutput(io.Discard)
	cmd.Execute()
}
