package main

import (
	"bytes"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

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

	// Validate access to Splunk Cloud Services and tenant

	index := Index{"Unknown", -1, "Unknown", []string{}, []Service{}, "Unknown"}
	f, err := os.OpenFile("main.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	logger := slog.New(slog.NewTextHandler(f, nil))
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
		logger.Info("OK application successfully pinged")
		splunkcollector("OK application successfully pinged", "INFO", SplunkTenant, SplunkToken)
		httpjsonresponse("OK", http.StatusOK, w)
	})

	http.HandleFunc("/warn", func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("OK Warn")
		splunkcollector("OK Warn generated", "WARN", SplunkTenant, SplunkToken)
		httpjsonresponse("WARNING", 199, w)
	})

	http.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		logger.Error("OK Error")
		splunkcollector("OK Error generated", "ERROR", SplunkTenant, SplunkToken)
		httpjsonresponse("ERROR", http.StatusInternalServerError, w)
	})

	var PORT string
	if PORT = os.Getenv("PORT"); PORT == "" {
		PORT = "8080"
	}
	var HOST string
	if HOST = os.Getenv("HOST"); PORT == "" {
		HOST = ""
	}
	logger.Info("logging the start of this app on " + PORT)
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

func splunkcollector(msg, level, tenant, token string) {
	jsonBody := []byte("{\"event\": \"" + msg + "\", \"fields\":{\"log_level\":\"" + level + "\"},\"sourcetype\": \"httpevent\",\"source\":\"lorensampleapp\"}")
	bodyReader := bytes.NewReader(jsonBody)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, err := http.NewRequest(http.MethodPost, "https://"+tenant+"/services/collector/event", bodyReader)

	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		//os.Exit(1)
	}
	req.Header.Set("Authorization", "Splunk "+token)

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		//os.Exit(1)
	}
	fmt.Println(res)
}

func checkForTenantToken() {
	if os.Getenv("BEARER_TOKEN") == "" {
		exitOnErr(fmt.Errorf("$BEARER_TOKEN must be set"))
	}
	if os.Getenv("TENANT") == "" {
		exitOnErr(fmt.Errorf("$TENANT must be set"))
	}
}
