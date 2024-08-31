package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"html/template"
	"io"
	"main/routers"
	"main/switches"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var log = logging.MustGetLogger("")

type Job struct {
	Number    int
	Output    string
	Status    string
	Initiator string
	Params    RunParams
}

type IndexHelper struct {
	SerialPorts []*enumerator.PortDetails
	Jobs        []Job
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
var output = make(chan string)

func findJob(num int) int {
	for i, job := range jobs {
		if job.Number == num {
			return i
		}
	}
	return -1
}

func snitchOutput(c chan string, job int) {
	serialOutput := <-c
	jobIdx := findJob(job)
	if jobIdx == -1 {
		log.Fatalf("snitchOutput: Could not find job %d\n", job)
	}
	for !strings.HasSuffix(serialOutput, "---EOF---") {
		jobs[jobIdx].Output += serialOutput
		delimited := strings.Split(jobs[jobIdx].Output, "\n")
		fmt.Printf("Line count on job %d: %d\n", job, len(delimited))
		if len(delimited) > 30 {
			jobs[jobIdx].Output = strings.Join(delimited[len(delimited)-30:], "\n")
		}
		serialOutput = <-c
	}
	jobs[jobIdx].Status = "EOF"
}

func runJob(rules RunParams, jobNum int) {
	mode := &serial.Mode{
		BaudRate: rules.PortConfig.BaudRate,
		DataBits: rules.PortConfig.DataBits,
	}

	switch rules.PortConfig.Parity {
	case "no":
		mode.Parity = serial.NoParity
	case "even":
		mode.Parity = serial.EvenParity
	case "odd":
		mode.Parity = serial.OddParity
	case "space":
		mode.Parity = serial.SpaceParity
	case "mark":
		mode.Parity = serial.MarkParity
	}

	switch rules.PortConfig.StopBits {
	case 1:
		mode.StopBits = serial.OneStopBit
	case 2:
		mode.StopBits = serial.TwoStopBits
	case 1.5:
		mode.StopBits = serial.OnePointFiveStopBits
	}

	if rules.DeviceType == "switch" {
		if rules.Reset {
			jobIdx := findJob(jobNum)
			if jobIdx == -1 {
				log.Warningf("How did we get here?\nJob number for switch requested: %d\nGot index %d\n", jobNum, jobIdx)
				jobs[jobIdx].Status = "Errored"
			} else {
				go switches.Reset(rules.PortConfig.Port, *mode, rules.Verbose, output)
				jobs[jobIdx].Status = "Resetting"
				go snitchOutput(output, jobNum)
				for jobs[jobIdx].Status != "EOF" {
					time.Sleep(1 * time.Minute)
				}
				jobs[jobIdx].Status = "Finished resetting"
			}
		}
		if rules.Defaults {
			var defaults switches.SwitchConfig
			err := json.Unmarshal([]byte(rules.DefaultsContents), &defaults)
			if err != nil {
				log.Warningf("Job %d failed: %s\n", jobNum, err)
				return
			}

			go switches.Defaults(rules.PortConfig.Port, *mode, defaults, rules.Verbose, output)
			jobIdx := findJob(jobNum)
			jobs[jobIdx].Status = "Applying defaults"
			go snitchOutput(output, jobNum)
			for jobs[jobIdx].Status != "EOF" {
				time.Sleep(1 * time.Minute)
			}
		}
		jobIdx := findJob(jobNum)
		jobs[jobIdx].Status = "Done"
	} else if rules.DeviceType == "router" {
		if rules.Reset {
			go routers.Reset(rules.PortConfig.Port, *mode, rules.Verbose, output)
			jobIdx := findJob(jobNum)
			if jobIdx == -1 {
				log.Warningf("How did we get here? Job number for switch requested: %d\n", jobNum)
			} else {
				jobs[jobIdx].Status = "Applying defaults"
				go snitchOutput(output, jobNum)
				for jobs[jobIdx].Status != "EOF" {
					time.Sleep(1 * time.Minute)
				}
			}
		}
		if rules.Defaults {
			var defaults routers.RouterDefaults
			err := json.Unmarshal([]byte(rules.DefaultsContents), &defaults)
			if err != nil {
				log.Warningf("Job %d failed: %s\n", jobNum, err)
				return
			}

			go routers.Defaults(rules.PortConfig.Port, *mode, defaults, rules.Verbose, output)
			jobIdx := findJob(jobNum)
			jobs[jobIdx].Status = "Applying defaults"
			go snitchOutput(output, jobNum)
			for jobs[jobIdx].Status != "EOF" {
				time.Sleep(1 * time.Minute)
			}
		}
	}
}

func portConfig(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", "port.html")
	//endpoint := strings.Split(strings.TrimSpace(filepath.Clean(r.URL.Path)[1:]), "/")
	fmt.Printf("portConfig: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	data, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Info(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func jobListHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", "jobs.html")
	fmt.Printf("jobListHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err := tmpl.ExecuteTemplate(w, "layout", jobs)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func portListHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate := filepath.Join("templates", "layout.html")
	pathTemplate := filepath.Join("templates", "ports.html")
	fmt.Printf("port: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	ports, _ := enumerator.GetDetailedPortsList()

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err := tmpl.ExecuteTemplate(w, "layout", ports)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
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

	jobIdx := findJob(reqJob)
	if jobIdx == -1 {
		fmt.Printf("jobHandler: Requested job %d not found\n", reqJob)
		http.Error(w, fmt.Sprintf("Job %d not found", reqJob), http.StatusTeapot)
		return
	}
	job = jobs[jobIdx]

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err = tmpl.ExecuteTemplate(w, "layout", job)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
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
		log.Info(err.Error())
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
	case "1":
		rules.PortConfig.StopBits = 1
		break
	case "one":
		rules.PortConfig.StopBits = 1
		break
	case "1.5":
		rules.PortConfig.StopBits = 1.5
		break
	case "opf":
		rules.PortConfig.StopBits = 1.5
		break
	case "2":
		rules.PortConfig.StopBits = 2
		break
	case "two":
		rules.PortConfig.StopBits = 2
		break
	default:
		rules.PortConfig.StopBits = -1
	}

	rules.DeviceType = r.PostFormValue("device")
	rules.Verbose = r.PostFormValue("verbose") == "verbose"
	rules.Reset = r.PostFormValue("reset") == "reset"
	rules.Defaults = r.PostFormValue("defaults") == "defaults"
	file, header, err := r.FormFile("defaultsFile")
	if err == nil {
		// Parse file name
		rules.DefaultsFile = header.Filename

		// Parse file contents
		var buf bytes.Buffer
		_, err = io.Copy(&buf, file)
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			log.Fatal(err)
		}
		rules.DefaultsContents = buf.String()
		buf.Reset()
	}

	jobNum := len(jobs) + 1

	newJob := Job{
		Number:    jobNum,
		Output:    "",
		Status:    "Created",
		Params:    rules,
		Initiator: strings.Join(strings.Split(r.RemoteAddr, ":")[:len(strings.Split(r.RemoteAddr, ":"))-1], ":"),
	}

	jobs = append(jobs, newJob)

	go runJob(rules, jobNum)

	fmt.Printf("POST Data: %+v\n", newJob)
	err = tmpl.ExecuteTemplate(w, "layout", newJob)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
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

	serialPorts, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
	}

	var indexHelper IndexHelper
	indexHelper.Jobs = jobs
	indexHelper.SerialPorts = serialPorts

	tmpl := template.Must(template.ParseFiles(layoutTemplate, pathTemplate))
	err = tmpl.ExecuteTemplate(w, "layout", indexHelper)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func ServeWeb() {
	muxer := http.NewServeMux()
	muxer.HandleFunc("GET /{$}", serveTemplate)
	muxer.HandleFunc("GET /port/{$}", portConfig)
	muxer.HandleFunc("GET /list/ports/{$}", portListHandler)
	muxer.HandleFunc("GET /device/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{port}/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{port}/{baud}/{$}", deviceConfig)
	muxer.HandleFunc("POST /device/{port}/{baud}/{data}/{parity}/{stop}/{$}", deviceConfig)
	muxer.HandleFunc("POST /reset/{$}", resetDevice)
	muxer.HandleFunc("GET /list/jobs/{$}", jobListHandler)
	muxer.HandleFunc("GET /jobs/{id}/{$}", jobHandler)
	fmt.Printf("Listening on port %d\n", 8080)
	log.Fatal(http.ListenAndServe(":8080", muxer))
}
