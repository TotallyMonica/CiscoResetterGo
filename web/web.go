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

func portConfig(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", "port.html")
	//endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")
	fmt.Printf("portConfig: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	data, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		log.Print(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	//endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")
	pathTemplate := filepath.Join("templates", "job.html")
	fmt.Printf("jobHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	reqJob, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fmt.Printf("jobHandler: Requested job %s is invalid\n", r.PathValue("job"))
		http.Error(w, "Invalid job given", http.StatusBadRequest)
		return
	}

	var job Job

	for _, job = range jobs {
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

func deviceConfig(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", "device.html")
	//endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")
	fmt.Printf("deviceConfig: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	//port := r.PathValue("port")
	//if port != "" {
	//	fmt.Printf("%s requested port %s\n", r.RemoteAddr, port)
	//}
	//if r.PathValue("baud") != "" {
	//	baud, err := strconv.Atoi(r.PathValue("baud"))
	//	if err != nil {
	//		log.Print(err.Error())
	//		http.Error(w, fmt.Sprintf("Invalid baud %s\n", r.PathValue("baud")), http.StatusBadRequest)
	//		return
	//	}
	//	if port != "" {
	//		fmt.Printf("%s requested baud %d\n", r.RemoteAddr, baud)
	//	}
	//}

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
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
	default:
		serialConf.StopBits = -1
	}

	fmt.Printf("POST Data: %+v\n", serialConf)
	err := tmpl.ExecuteTemplate(w, "layout", serialConf)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func resetDevice(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", "reset.html")
	// endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")
	fmt.Printf("resetDevice: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))

	var rules RunParams
	rules.PortConfig.Port = r.PostFormValue("port")
	rules.PortConfig.BaudRate, _ = strconv.Atoi(r.PostFormValue("baud"))
	rules.PortConfig.DataBits, _ = strconv.Atoi(r.PostFormValue("data"))
	rules.PortConfig.Parity = r.PostFormValue("parity")
	stopBits := r.PostFormValue("stop")
	switch stopBits {
	case "one":
		rules.PortConfig.StopBits = 1
	case "opf":
		rules.PortConfig.StopBits = 1.5
	case "two":
		rules.PortConfig.StopBits = 2
	default:
		rules.PortConfig.StopBits = -1
	}

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
	fmt.Printf("serveTemplate: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	// We want to do nothing with jobs here
	if endpoint[0] == "jobs" {
		jobHandler(w, r)
		return
	}

	if endpoint[0] == "" {
		_, err := os.Stat(filepath.Join("templates", "index.html"))
		if err != nil {
			http.Redirect(w, r, "/port", http.StatusTemporaryRedirect)
			return
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
	err = tmpl.ExecuteTemplate(w, "layout", nil)
	if err != nil {
		// Log the detailed error
		log.Print(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func ServeWeb() {
	muxer := http.NewServeMux()
	muxer.HandleFunc("GET /{$}", serveTemplate)
	muxer.HandleFunc("GET /port/{$}", portConfig)
	muxer.HandleFunc("GET /device/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{port}/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{port}/{baud}/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{port}/{baud}/{data}/{parity}/{stop}/{$}", deviceConfig)
	muxer.HandleFunc("POST /reset/{$}", resetDevice)
	muxer.HandleFunc("GET /jobs/{id}/{$}", jobHandler)
	fmt.Printf("Listening on port %d\n", 8080)
	log.Fatal(http.ListenAndServe(":8080", muxer))
}
