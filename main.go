package main

import (
	"bytes"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	cfenv "github.com/cloudfoundry-community/go-cfenv"
)

//go:embed all:templates
var indexHtml embed.FS

//go:embed all:static
var staticAssets embed.FS

// Index holds fields displayed on the index.html template
type Index struct {
	AppName          string
	AppInstanceIndex int
	AppInstanceGUID  string
	Envars           []string
	Services         []Service
	SpaceName        string
}

// Service holds the name and label of a service instance
type Service struct {
	Name  string
	Label string
}

func main() {
	//init splunk
	checkForTenantToken()
	// Initialize the client
	SplunkToken := os.Getenv("BEARER_TOKEN")
	SplunkTenant := os.Getenv("TENANT")
	NewRelicToken := os.Getenv("NEWRELIC_TOKEN")

	// Validate access to Splunk Cloud Services and tenant

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("pcfgosampleappk8ssvc"),
		newrelic.ConfigLicense(NewRelicToken),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	if err != nil {
		panic(err)
	}

	index := Index{"Unknown", -1, "Unknown", []string{}, []Service{}, "Unknown"}
	f, err := os.OpenFile("main.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	//get tas info
	name := os.Getenv("ARTIFACT")

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println(exPath)
	entries, err := os.ReadDir("./")
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		fmt.Println(e.Name())
	}

	mw := io.MultiWriter(os.Stdout, f)
	logger := slog.New(slog.NewTextHandler(mw, nil))

	template := template.Must(template.ParseFiles("templates/index.html"))

	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.Dir("static"))))

	if cfenv.IsRunningOnCF() {
		appEnv, err := cfenv.Current()
		if err != nil {
			logger.Error(err.Error())
			log.Fatal(err)
		}
		if appEnv.Name != "" {
			index.AppName = appEnv.Name
		}
		if appEnv.Index > -1 {
			index.AppInstanceIndex = appEnv.Index
		}
		if appEnv.InstanceID != "" {
			index.AppInstanceGUID = appEnv.InstanceID
		}
		if appEnv.SpaceName != "" {
			index.SpaceName = appEnv.SpaceName
		}
		for _, svcs := range appEnv.Services {
			for _, svc := range svcs {
				index.Services = append(index.Services, Service{svc.Name, svc.Label})
			}
		}
		for _, envar := range os.Environ() {
			if strings.HasPrefix(envar, "TRAINING_") {
				index.Envars = append(index.Envars, envar)
			}
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
		w.Header().Set("Pragma", "no-cache")                                   // HTTP 1.0.
		w.Header().Set("Expires", "0")                                         // Proxies.
		if err := template.ExecuteTemplate(w, "index.html", index); err != nil {
			logger.Error("500 Error on template index.html:" + err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		logger.Error("force kill")
		os.Exit(1)
	})

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		splunkres := splunkcollector("OK application successfully pinged", "INFO", SplunkTenant, SplunkToken, index.AppName, name, logger)
		httpjsonresponse("OK:"+splunkres, http.StatusOK, w)
	})

	http.HandleFunc("/warn", func(w http.ResponseWriter, r *http.Request) {
		splunkres := splunkcollector("OK Warn generated", "WARN", SplunkTenant, SplunkToken, index.AppName, name, logger)
		httpjsonresponse("WARNING:"+splunkres, 199, w)
	})

	http.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		splunkres := splunkcollector("OK Error generated", "ERROR", SplunkTenant, SplunkToken, index.AppName, name, logger)
		httpjsonresponse("ERROR:"+splunkres, http.StatusInternalServerError, w)
	})

	http.HandleFunc(newrelic.WrapHandleFunc(app, "/nrerror", func(w http.ResponseWriter, r *http.Request) {
		httpjsonresponse("ERROR:"+"new relic", http.StatusInternalServerError, w)
	}))

	var PORT string
	if PORT = os.Getenv("PORT"); PORT == "" {
		PORT = "8080"
	}
	var HOST string
	if HOST = os.Getenv("HOST"); PORT == "" {
		HOST = ""
	}

	http.Handle("/metrics", promhttp.Handler())

	// add sleep to increase start up time
	StartUpSleep, err := strconv.Atoi(os.Getenv("STARTUP_SLEEP"))
	if err != nil {
		splunkcollector("failed to set sleep time from env var STARTUP_SLEEP. Setting to 0", "WARN", SplunkTenant, SplunkToken, index.AppName, name, logger)
		StartUpSleep = 0
	}
	splunkcollector("sleeping for "+strconv.Itoa(StartUpSleep)+" seconds", "INFO", SplunkTenant, SplunkToken, index.AppName, name, logger)
	time.Sleep(time.Duration(StartUpSleep) * time.Second)

	splunkcollector("logging the start of this app on "+HOST+":"+PORT, "INFO", SplunkTenant, SplunkToken, index.AppName, name, logger)
	fmt.Println(http.ListenAndServe(HOST+":"+PORT, nil))
}

type Data struct {
	Response string
}

func httpjsonresponse(response string, code int, w http.ResponseWriter) {
	data := Data{Response: response}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func splunkcollector(msg, level, tenant, token, tasApplicationName, name string, logger *slog.Logger) string {
	switch level {
	case "INFO":
		logger.Info(msg)
	case "WARN":
		logger.Warn(msg)
	case "ERROR":
		logger.Error(msg)
	}
	jsonBody := []byte("{\"event\": \"" + msg + "\", \"fields\":{\"log_level\":\"" + level + "\",\"app_name\":\"" + name + "\"},\"sourcetype\": \"httpevent\",\"host\":\"" + tasApplicationName + "\",\"source\":\"" + name + "\"}")
	bodyReader := bytes.NewReader(jsonBody)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, err := http.NewRequest(http.MethodPost, "https://"+tenant+"/services/collector/event", bodyReader)

	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		return "client: could not create request:" + err.Error()
	}
	req.Header.Set("Authorization", "Splunk "+token)

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		return "client: error making http request:" + err.Error()
	}
	fmt.Println(res)
	return tasApplicationName + " wrote to splunk successfully"
}

func checkForTenantToken() {
	if os.Getenv("BEARER_TOKEN") == "" {
		fmt.Printf("$BEARER_TOKEN must be set for splunk to log")
	}
	if os.Getenv("TENANT") == "" {
		fmt.Printf("$TENANT must be set splunk to log")
	}
}
