package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"html/template"
	"io"
	"main/common"
	"main/routers"
	"main/switches"
	"main/templates"
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
	BackupConfig     common.Backup
}

type SerialConfiguration struct {
	Port      string
	BaudRate  int
	DataBits  int
	Parity    string
	StopBits  float32
	ShortHand string
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
	for !strings.HasSuffix(strings.TrimSpace(serialOutput), "---EOF---") {
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
				go switches.Reset(rules.PortConfig.Port, *mode, rules.BackupConfig, rules.Verbose, output)
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
			go routers.Reset(rules.PortConfig.Port, *mode, rules.BackupConfig, rules.Verbose, output)
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
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	portTemplate, err := layoutTemplate.Parse(templates.Port)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("portConfig: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	data, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = portTemplate.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Info(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func jobListHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		// TODO: We know *what* the error will likely be, should this be customized at all?
		http.Error(w, http.StatusText(500), 500)
		return
	}

	jobsTemplate, err := layoutTemplate.Parse(templates.Jobs)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("jobListHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	err = jobsTemplate.ExecuteTemplate(w, "layout", jobs)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func portListHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	portsTemplate, err := layoutTemplate.Parse(templates.Ports)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	fmt.Printf("port: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = portsTemplate.ExecuteTemplate(w, "layout", ports)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	jobTemplate, err := layoutTemplate.Parse(templates.Job)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("jobHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	vars := mux.Vars(r)

	reqJob, err := strconv.Atoi(vars["id"])
	if err != nil {
		fmt.Printf("jobHandler: Requested job %s is invalid\n", vars["id"])
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

	err = jobTemplate.ExecuteTemplate(w, "layout", job)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func deviceConfig(w http.ResponseWriter, r *http.Request) {
	layoutTemplate, err := template.New("template").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Warningf("Error while parsing template: %s\n", err)
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	var deviceTemplate *template.Template
	if os.Getenv("DEBUGHTTPPAGE") == "1" {
		log.Info("Presenting raw file as environment variable DEBUGHTTPPAGE is set\n")
		pathTemplate := filepath.Join("templates", "device.html")
		deviceTemplate, err = layoutTemplate.ParseFiles(pathTemplate)
	} else {
		deviceTemplate, err = layoutTemplate.Parse(templates.Device)
	}
	if err != nil {
		// Log the detailed error
		log.Warningf("Error while parsing template: %s\n", err)
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

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

	// Form results, formatted for reasonable usage
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

	err = deviceTemplate.ExecuteTemplate(w, "layout", serialConf)
	if err != nil {
		log.Warningf("Error while executing template: %s\n", err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func resetDevice(w http.ResponseWriter, r *http.Request) {
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	resetTemplate, err := layoutTemplate.Parse(templates.Reset)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	fmt.Printf("resetDevice: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	// Format form results
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

	// Build the shorthand version
	truncated := fmt.Sprintf("%.1f", rules.PortConfig.StopBits)
	if strings.HasSuffix(truncated, ".0") {
		truncated = truncated[:len(truncated)-2]
	}
	rules.PortConfig.ShortHand = fmt.Sprintf("%d %d%c%s", rules.PortConfig.BaudRate, rules.PortConfig.DataBits, rules.PortConfig.Parity[0], truncated)

	// Format some more HTML results to be presented in a table
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

	rules.BackupConfig.Backup = r.PostFormValue("backup") == "backup"
	if r.PostFormValue("dhcp") != "dhcp" {
		rules.BackupConfig.Source = r.PostFormValue("source")
		rules.BackupConfig.SubnetMask = r.PostFormValue("mask")
	}
	rules.BackupConfig.Destination = r.PostFormValue("destination")
	rules.BackupConfig.UseBuiltIn = r.PostFormValue("builtin") == "builtin"

	log.Debugf("POST Data: %+v\n", rules)

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

	err = resetTemplate.ExecuteTemplate(w, "layout", newJob)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	indexTemplate, err := layoutTemplate.Parse(templates.Index)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("serveIndex: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	serialPorts, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
	}

	// Because templates have to only take one struct, we have to have a special struct just for it
	var indexHelper IndexHelper
	indexHelper.Jobs = jobs
	indexHelper.SerialPorts = serialPorts

	err = indexTemplate.ExecuteTemplate(w, "layout", indexHelper)
	if err != nil {
		// Log the detailed error
		log.Info(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func ServeWeb() {
	// Gorilla muxer to support Windows 7
	muxer := mux.NewRouter()
	muxer.HandleFunc("/", serveIndex).Methods("GET")
	muxer.HandleFunc("/port/", portConfig).Methods("GET")
	muxer.HandleFunc("/list/ports/", portListHandler).Methods("GET")
	muxer.HandleFunc("/device/", deviceConfig).Methods("GET")
	muxer.HandleFunc("/device/", deviceConfig).Methods("POST")
	muxer.HandleFunc("/device/{port}/", deviceConfig).Methods("POST")
	muxer.HandleFunc("/device/{port}/{baud}/", deviceConfig).Methods("POST")
	muxer.HandleFunc("/device/{port}/{baud}/{data}/{parity}/{stop}/", deviceConfig).Methods("POST")
	muxer.HandleFunc("/reset/", resetDevice).Methods("POST", "GET")
	muxer.HandleFunc("/list/jobs/", jobListHandler).Methods("GET")
	muxer.HandleFunc("/jobs/{id}/", jobHandler).Methods("GET")

	server := &http.Server{
		Handler:      muxer,
		Addr:         "0.0.0.0:8080",
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	fmt.Printf("Listening on port %d\n", 8080)
	log.Fatal(server.ListenAndServe())
}
