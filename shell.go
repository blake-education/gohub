package main

import (
	"log"
	"os"
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
	cmd.Env = []string{
		"PAYLOAD=" + data.OriginalPayload,
		"BRANCH=" + data.Branch,
		"REPO=" + data.Name,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// start it
	if err := cmd.Start(); err != nil {
		log.Println("unable to start shell script:", err)
		return
	}

	pid := cmd.Process.Pid

	log.Printf("[%d] started shell script", pid)

	// wait in a goroutine
	donec := make(chan error, 1)
	go func() {
		donec <- cmd.Wait()
	}()

	// channel select
	select {
	case <-time.After(time.Duration(shellTimeout) * time.Second):
		cmd.Process.Kill()
		<-donec // allow wait goroutine to exit
		log.Printf("[%d] shell script timed out after %vs", pid, shellTimeout)
	case waitErr := <-donec:
		// waitErr implies that the script didn't end well
		if waitErr != nil {
			if msg, ok := waitErr.(*exec.ExitError); ok {
				log.Printf("[%d] shell script error: %s", pid, msg)
			} else { // some other kind of worse error
				log.Fatal(waitErr)
			}
		}
	}

	log.Printf("[%d] shell finished", pid)
}
