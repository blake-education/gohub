package main

import (
  "bufio"
	"log"
  "io"
	"os/exec"
	"time"
)


// make a goroutine and channel for reading lines from a pipe
func readPipe(rawPipe io.Reader, _ error) (c chan string) {
  pipe := bufio.NewReader(rawPipe)
  c = make(chan string)

  go func() {
    for {
      line, err := pipe.ReadString('\n')
      if(err != nil) {
        break
      }
      c <- line
    }
  }()

  return c
}


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

  stderrc := readPipe(cmd.StderrPipe())
  stdoutc := readPipe(cmd.StdoutPipe())

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

  looping := true
  for {
    // channel select
    select {
    case <-time.After(time.Duration(shellTimeout) * time.Second):
      cmd.Process.Kill()
      log.Printf("[%d] shell script timed out after %vs", pid, shellTimeout)
      looping = false

    case line := <-stderrc:
      log.Printf("[%d] sh err: %s", pid, line)

    case line := <-stdoutc:
      log.Printf("[%d] sh out: %s", pid, line)

    case waitErr := <-donec:
      // waitErr implies that the script didn't end well
      if waitErr != nil {
        if msg, ok := waitErr.(*exec.ExitError); ok {
          log.Printf("[%d] shell script error: %s", pid, msg)
        } else { // some other kind of worse error
          log.Fatal(waitErr)
        }
      }
      looping = false
    }

    if !looping {
      break
    }
  }

	log.Printf("[%d] shell finished", pid)
}
