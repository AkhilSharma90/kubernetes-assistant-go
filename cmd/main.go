package main

import (
	"github.com/akhilsharma90/kubectl-assistant/cmd/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	cli.InitAndExecute()
}
