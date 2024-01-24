package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	cfenv "github.com/cloudfoundry-community/go-cfenv"
)

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
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		logger.Error("force kill")
		os.Exit(1)
	})

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("OK PING")
		data := Data{Response: "OK"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	})

	http.HandleFunc("/warn", func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("OK Warn")
		data := Data{Response: "WARNING"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(199)
		json.NewEncoder(w).Encode(data)
	})

	http.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		logger.Error("OK Error")
		data := Data{Response: "ERROR"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(data)
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
