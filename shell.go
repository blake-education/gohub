package main

import (
	"bytes"
	"log"
	"os/exec"
	"time"
)

func ExecuteShell(hook Hook, data GithubJson) {
	log.Printf("executing shell script %s\n", hook.Shell)

	// shellTimeout or default
	shellTimeout := hook.ShellTimeout
	if shellTimeout == 0 {
		shellTimeout = DefaultShellTimeout
	}

	// setup the command
	cmd := exec.Command(hook.Shell, hook.Repo)
	cmd.Env = []string{"PAYLOAD=" + data.OriginalPayload}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// start it
	if err := cmd.Start(); err != nil {
		log.Println("unable to start shell script:", err)
		return
	}

	// wait in a goroutine
	donec := make(chan error, 1)
	go func() {
		donec <- cmd.Wait()
	}()

	// channel select
	select {
	case <-time.After(time.Duration(shellTimeout) * time.Second):
		cmd.Process.Kill()
		log.Printf("shell script timed out after %vs", shellTimeout)

	case waitErr := <-donec:
		// waitErr implies that the script didn't end well
		if waitErr != nil {
			if msg, ok := waitErr.(*exec.ExitError); ok {
				log.Printf("shell script error: %s", msg)
			} else { // some other kind of worse error
				log.Fatal(waitErr)
			}
		}
	}

	log.Printf("shell script output was:\n%v\n", output.String())
	log.Println("shell finished")
}
