package web

import (
	"bytes"
	"fmt"
	"go.bug.st/serial/enumerator"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Job struct {
	Number int
	Output string
	Exists bool
	Params RunParams
}

type RunParams struct {
	PortConfig       SerialConfiguration
	DeviceType       string
	Verbose          bool
	Reset            bool
	Defaults         bool
	DefaultsFile     string
	DefaultsContents string
}

type SerialConfiguration struct {
	Port     string
	BaudRate int
	DataBits int
	Parity   string
	StopBits float32
}

var jobs []Job

func jobHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")
	pathTemplate := filepath.Join("templates", "job.html")

	reqJob, err := strconv.Atoi(endpoint[1])
	if err != nil {
		fmt.Printf("jobHandler: Requested job %s is invalid\n", r.PathValue("job"))
		http.Error(w, "Invalid job given", http.StatusBadRequest)
		return
	}

	var job Job

	localJobs := jobs

	for _, job = range localJobs {
		if job.Number == reqJob {
			break
		}
	}
	if job.Exists == false {
		fmt.Printf("jobHandler: Requested job %d not found\n", reqJob)
		http.Error(w, fmt.Sprintf("Job %d not found", reqJob), http.StatusTeapot)
		return
	}

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err = tmpl.ExecuteTemplate(w, "layout", job)
	if err != nil {
		// Log the detailed error
		log.Print(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func serveTemplate(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", filepath.Clean(r.URL.Path)+".html")
	endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")

	// We want to do nothing with jobs here
	if endpoint[0] == "jobs" {
		jobHandler(w, r)
		return
	}

	if endpoint[0] == "" {
		_, err := os.Stat(filepath.Join("templates", "index.html"))
		if err != nil {
			http.Redirect(w, r, "/port", http.StatusPermanentRedirect)
		} else {
			pathTemplate = filepath.Join("templates", "index.html")
		}
	}

	// Return a 404 if the template doesn't exist
	info, err := os.Stat(pathTemplate)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("serveTemplate: Requested page %s does not exist\n", pathTemplate)
			http.NotFound(w, r)
			return
		}
	}

	// Return a 404 if the request is for a directory
	if info.IsDir() {
		fmt.Printf("serveTemplate: Requested page %s is a directory\n", pathTemplate)
		http.NotFound(w, r)
		return
	}

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))

	switch {
	// Handle port-specific functionality
	case endpoint[0] == "port":
		data, err := enumerator.GetDetailedPortsList()
		if err != nil {
			// Log the detailed error
			log.Print(err.Error())
			// Return a generic "Internal Server Error" message
			http.Error(w, http.StatusText(500), 500)
			return
		}

		err = tmpl.ExecuteTemplate(w, "layout", data)

	// Render reset-specific functionality
	case endpoint[0] == "reset":
		var rules RunParams
		rules.PortConfig.Port = r.PostFormValue("port")
		rules.PortConfig.BaudRate, _ = strconv.Atoi(r.PostFormValue("baud"))
		rules.PortConfig.DataBits, _ = strconv.Atoi(r.PostFormValue("data"))
		rules.PortConfig.Parity = r.PostFormValue("parity")
		stopBits64, err := strconv.ParseFloat(r.PostFormValue("stop"), 32)
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		rules.PortConfig.StopBits = float32(stopBits64)

		rules.DeviceType = r.PostFormValue("device")
		rules.Verbose = r.PostFormValue("verbose") == "verbose"
		rules.Reset = r.PostFormValue("reset") == "reset"
		rules.Defaults = r.PostFormValue("defaults") == "defaults"
		file, header, err := r.FormFile("defaultsFile")
		if err != nil {
			return
		} else {
			// Parse file name
			rules.DefaultsFile = header.Filename

			// Parse file contents
			var buf bytes.Buffer
			io.Copy(&buf, file)
			rules.DefaultsContents = buf.String()
			buf.Reset()
		}

		newJob := Job{
			Number: len(jobs) + 1,
			Output: "",
			Exists: true,
			Params: rules,
		}

		jobs = append(jobs, newJob)

		fmt.Printf("POST Data: %+v\n", newJob)
		err = tmpl.ExecuteTemplate(w, "layout", newJob)

	// Render device-specific configuration
	case endpoint[0] == "device":
		var serialConf SerialConfiguration
		serialConf.Port = r.PostFormValue("device")
		serialConf.BaudRate, _ = strconv.Atoi(r.PostFormValue("baud"))
		serialConf.DataBits, _ = strconv.Atoi(r.PostFormValue("data"))
		serialConf.Parity = r.PostFormValue("parity")
		switch r.PostFormValue("stop") {
		case "one":
			serialConf.StopBits = 1
		case "opf":
			serialConf.StopBits = 1.5
		case "two":
			serialConf.StopBits = 2
		}

		fmt.Printf("POST Data: %+v\n", serialConf)
		err = tmpl.ExecuteTemplate(w, "layout", serialConf)

	// We want all jobs to be handled by the jobs handler
	case endpoint[0] == "jobs":
		return

	// Default behavior for endpoints
	default:
		err = tmpl.ExecuteTemplate(w, "layout", nil)
	}
	if err != nil {
		log.Print(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func ServeWeb() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("GET /jobs/{id}/", jobHandler)
	http.HandleFunc("/", serveTemplate)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
