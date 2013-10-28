package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const DefaultShellTimeout uint = 600
const DefaultPort string = "7654"
const DefaultLogFile string = "-"

type Repository struct {
	Name string
}

type GithubJson struct {
	Repository      Repository
	Ref             string
	OriginalPayload string
}

type Config struct {
	Hooks   []Hook
	Port    string
	LogFile string
}

type Hook struct {
	Repo         string
	Branch       string
	Shell        string
	ShellTimeout uint
	Token        string
}

func loadConfig(configFile *string) Config {
	config := Config{
		Port:    DefaultPort,
		LogFile: DefaultLogFile,
	}

	flag.Parse()

	configData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal(err)
	}

	// override from flags
	if *port != "" {
		config.Port = *port
	}
	if *logFile != "" {
		config.LogFile = *logFile
	}

	// TODO validate

	return config
}

func setLog(logFile string) {
	if "-" == logFile {
		log.SetOutput(os.Stdout)
	} else {
		log_handler, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			panic("cannot write log")
		}
		log.SetOutput(log_handler)
	}
	log.SetFlags(5)
}

func startWebserver(port string) {
	log.Println("starting webserver on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func matchHook(data GithubJson, hook Hook) bool {
	fullBranch := "refs/heads/" + hook.Branch

	return data.Repository.Name == hook.Repo && (hook.Branch == "*" || data.Ref == fullBranch)
}

func matchGithubJson(a, b GithubJson) bool {
	log.Println("matcher a == b", a, b)
	return a.Ref == b.Ref && a.Repository == b.Repository
}

func setupHooks(hooks []Hook) {
	for _, hook := range hooks {
		addHandler(hook)
	}
}

func addHandler(hook Hook) {
	uri := "/" + hook.Repo

	if hook.Token != "" {
		uri += "/" + hook.Token
	}

	// this channel gives a stream of unique jobs
	// that is, if a job is submitted that matches a job already waiting in the queue
	// it isn't re-added
	uniqueShellJobs := make(chan GithubJson)

	shellJobs := CoalescingBufferList(uniqueShellJobs, matchGithubJson)

	// consume from the channel forever
	go func() {
		for {
			ExecuteShell(hook, <-uniqueShellJobs)
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

		// the hook matched our criteria, put it on the queue
		if matchHook(data, hook) {
			log.Printf("matched repo %s\n", hook.Repo)
			shellJobs <- data
		}
	})
}

var (
	port       = flag.String("port", "", "port to listen on")
	configFile = flag.String("config", "./config.json", "config")
	logFile    = flag.String("log", "", "log file")
)

func main() {
	config := loadConfig(configFile)
	setLog(config.LogFile)
	setupHooks(config.Hooks)
	startWebserver(config.Port)
}
