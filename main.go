package main

import (
  "bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
  "time"
)

const DefaultShellTimeout uint = 600

type Repository struct {
	Name string
}

type GithubJson struct {
	Repository Repository
	Ref        string
  OriginalPayload string
}

type Config struct {
	Hooks []Hook
}

type Hook struct {
	Repo   string
	Branch string
	Shell  string
	ShellTimeout  uint
	Token  string
}


func loadConfig(configFile *string) {
	var config Config
	configData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal(err)
	}
	for _, hook := range config.Hooks {
		addHandler(hook)
	}
}

func setLog(logFile *string) {
	if "-" == *logFile {
		log.SetOutput(os.Stdout)
	} else {
		log_handler, err := os.OpenFile(*logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			panic("cannot write log")
		}
		log.SetOutput(log_handler)
	}
	log.SetFlags(5)
}

func startWebserver() {
	log.Println("starting webserver")
	http.ListenAndServe(":"+*port, nil)
}

func matchHook(data GithubJson, hook Hook) bool {
	fullBranch := "refs/heads/" + hook.Branch

	return data.Repository.Name == hook.Repo && (hook.Branch == "*" || data.Ref == fullBranch)
}

func addHandler(hook Hook) {
	uri := "/" + hook.Repo

	if hook.Token != "" {
		uri += "/" + hook.Token
	}

  shellJobs := make(chan GithubJson, 1000)

  go func() {
    for {
      shellJob := <-shellJobs
      executeShell(hook, shellJob)
    }
  }()

	http.HandleFunc(uri, func(w http.ResponseWriter, r *http.Request) {
		payload := r.FormValue("payload")

		var data GithubJson
		err := json.Unmarshal([]byte(payload), &data)
		if err != nil {
			log.Println(err)
		}

    data.OriginalPayload = payload

		if matchHook(data, hook) {
      log.Printf("matched repo %s\n", hook.Repo)
      shellJobs <- data
		}
	})
}

func executeShell(hook Hook, data GithubJson) {
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

var (
	port       = flag.String("port", "7654", "port to listen on")
	configFile = flag.String("config", "./config.json", "config")
	logFile    = flag.String("log", "./log", "log file")
)

func init() {
	flag.Parse()
}

func main() {
	setLog(logFile)
	loadConfig(configFile)
	startWebserver()
}
