package main

import (
	"fmt"
	"os"
	"os/exec"
)

type smokeCommand struct {
	name string
	args []string
}

func main() {
	commands := []smokeCommand{
		{name: "controlplane_runtime", args: []string{"run", "./cmd/verify/controlplane_runtime"}},
		{name: "chat_policy", args: []string{"run", "./cmd/verify/chat_policy"}},
	}

	for _, command := range commands {
		fmt.Println("[RUN]", command.name)
		cmd := exec.Command("go", command.args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("[FAIL]", command.name)
			fmt.Printf("verify result: FAIL smoke(controlplane_runtime,chat_policy) failed_at=%s\n", command.name)
			os.Exit(1)
		}
		fmt.Println("[PASS]", command.name)
	}

	fmt.Println("verify result: PASS smoke(controlplane_runtime,chat_policy)")
}
