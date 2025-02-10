package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"html/template"
	"io"
	"log"
	"main/common"
	"main/crglogging"
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

const WEB_LOGGER_NAME = "WebLogger"

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
		serialOutput = <-c
	}
	jobs[jobIdx].Status = "EOF"
}

func runJob(rules RunParams, jobNum int) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

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
				webLogger.Errorf("How did we get here?\nJob number for switch requested: %d\nGot index %d\n", jobNum, jobIdx)
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
				webLogger.Warningf("Job %d failed: %s\n", jobNum, err)
				return
			}

			go switches.Defaults(rules.PortConfig.Port, *mode, defaults, rules.Verbose, output)
			jobIdx := findJob(jobNum)
			jobs[jobIdx].Status = "Applying defaults"
			go snitchOutput(output, jobNum)
			for jobs[jobIdx].Status != "EOF" {
				time.Sleep(1 * time.Minute)
			}
			jobs[jobIdx].Status = "Finished resetting"
		}
		jobIdx := findJob(jobNum)
		jobs[jobIdx].Status = "Done"
	} else if rules.DeviceType == "router" {
		if rules.Reset {
			go routers.Reset(rules.PortConfig.Port, *mode, rules.BackupConfig, rules.Verbose, output)
			jobIdx := findJob(jobNum)
			if jobIdx == -1 {
				webLogger.Errorf("How did we get here? Job number for switch requested: %d\n", jobNum)
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
				webLogger.Warningf("Job %d failed: %s\n", jobNum, err)
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

		jobIdx := findJob(jobNum)
		jobs[jobIdx].Status = "Done"
	}
}

// New Client API Endpoint
func newClientApi(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	var clientTemplate *template.Template
	if os.Getenv("DEBUGHTTPPAGE") == "1" {
		webLogger.Info("Presenting raw file as environment variable DEBUGHTTPPAGE is set\n")
		pathTemplate := filepath.Join("templates", "api", "client.html")
		clientTemplate, err = layoutTemplate.ParseFiles(pathTemplate)
	} else {
		clientTemplate, err = layoutTemplate.Parse(templates.Client)
	}
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("Error while parsing template: %s\n", err)
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("clientHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	if r.Method == "POST" {
		ports, err := enumerator.GetDetailedPortsList()
		if err != nil {
			// Log the detailed error
			webLogger.Errorf(err.Error())
			// Return a generic "Internal Server Error" message
			http.Error(w, http.StatusText(500), 500)
			return
		}
		jsonPorts, err := json.Marshal(ports)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(jsonPorts)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}
		return
	} else if r.Method == "GET" {
		err = clientTemplate.ExecuteTemplate(w, "layout", nil)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}
	}
}

// Client job info
func clientJobApi(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(400), 400)
			return
		}

		fmt.Printf("clientJobApi: %s sent data to %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

		var body Job
		err = json.Unmarshal(rawBody, &body)
		if err != nil {
			webLogger.Errorf("clientJobApi: Error while unmarshalling data from %s: %s\n", r.RemoteAddr, err.Error())
			http.Error(w, http.StatusText(400), 400)
			return
		}

		jobIdx := findJob(body.Number)
		if jobIdx == -1 {
			webLogger.Errorf("Job %d could not be found.\n", body.Number)
			http.Error(w, http.StatusText(404), 404)
			return
		}
		jobs[jobIdx] = body

		webLogger.Infof("Updated job %d info from client %s\n", body.Number, r.RemoteAddr)

		jsonJob, err := json.Marshal(jobs)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(jsonJob)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}
		return
	} else if r.Method == "GET" {
		jsonJob, err := json.Marshal(jobs)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(jsonJob)
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
		}
		return
	}
}

func portConfig(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	portTemplate, err := layoutTemplate.Parse(templates.Port)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("portConfig: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	data, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = portTemplate.ExecuteTemplate(w, "layout", data)
	if err != nil {
		webLogger.Errorf(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func jobListHandler(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		// TODO: We know *what* the error will likely be, should this be customized at all?
		http.Error(w, http.StatusText(500), 500)
		return
	}

	jobsTemplate, err := layoutTemplate.Parse(templates.Jobs)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("jobListHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	err = jobsTemplate.ExecuteTemplate(w, "layout", jobs)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func portListHandler(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	portsTemplate, err := layoutTemplate.Parse(templates.Ports)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	fmt.Printf("port: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = portsTemplate.ExecuteTemplate(w, "layout", ports)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while parsing the layout for jobs: %s\n", err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	jobTemplate, err := layoutTemplate.Parse(templates.Job)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while parsing the job template: %s\n", err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("jobHandler: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	vars := mux.Vars(r)

	reqJob, err := strconv.Atoi(vars["id"])
	if err != nil {
		webLogger.Errorf("jobHandler: Requested job %s is invalid\n", vars["id"])
		http.Error(w, "Invalid job given", http.StatusBadRequest)
		return
	}

	var job Job

	jobIdx := findJob(reqJob)
	if jobIdx == -1 {
		webLogger.Errorf("jobHandler: Requested job %d not found\n", reqJob)
		http.Error(w, fmt.Sprintf("Job %d not found", reqJob), http.StatusTeapot)
		return
	}
	job = jobs[jobIdx]

	// Determine the amount of lines to print out if requested
	if len(r.URL.Query().Get("lines")) != 0 {
		lineCount, err := strconv.Atoi(r.URL.Query().Get("lines"))

		// Ensure line count is an integer
		if err != nil {
			webLogger.Errorf("jobHandler: Requested line count is invalid: %s\n", r.URL.Query().Get("lines"))
			http.Error(w, "Invalid line count", http.StatusBadRequest)
			return
		}

		// Only print the requested number of lines
		webLogger.Infof("jobHandler: Client %s requested %d line(s) from job %d\n", r.RemoteAddr, lineCount, reqJob)
		if len(strings.Split(job.Output, "\n")) > lineCount {
			job.Output = strings.Join(strings.Split(job.Output, "\n")[len(strings.Split(job.Output, "\n"))-lineCount:], "\n")
		}
	}

	err = jobTemplate.ExecuteTemplate(w, "layout", job)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while executing the template for job %d: %s\n", jobIdx, err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func deviceConfig(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("template").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("Error while parsing template: %s\n", err)
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	var deviceTemplate *template.Template
	if os.Getenv("DEBUGHTTPPAGE") == "1" {
		webLogger.Info("Presenting raw file as environment variable DEBUGHTTPPAGE is set\n")
		pathTemplate := filepath.Join("templates", "device.html")
		deviceTemplate, err = layoutTemplate.ParseFiles(pathTemplate)
	} else {
		deviceTemplate, err = layoutTemplate.Parse(templates.Device)
	}
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("Error while parsing template: %s\n", err)
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
		webLogger.Errorf("Error while executing template: %s\n", err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func resetDevice(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
	resetTemplate, err := layoutTemplate.Parse(templates.Reset)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
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

	webLogger.Debugf("POST Data: %+v\n", rules)

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
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func builderHome(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	webLogger.Infof("builderHome: Client %s requested %s with method %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path), r.Method)
	var builderPage *template.Template
	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	params := mux.Vars(r)

	devType := params["device"]

	if r.Method == "POST" {
		w.Header().Set("Content-Type", "application/json")

		err = r.ParseForm()
		if err != nil {
			webLogger.Errorf(err.Error())
			http.Error(w, http.StatusText(500), 500)
			return
		}

		fmt.Printf("Post form values:\n")
		for key, value := range r.PostForm {
			fmt.Printf("\t%s: %s\n", key, value)
		}

		var formattedJson []byte

		if devType == "switch" {
			// Build out json file
			var createdTemplate switches.SwitchConfig
			createdTemplate.Version = 0.02

			// Root level values
			createdTemplate.DefaultGateway = r.PostFormValue("gateway")
			createdTemplate.EnablePassword = r.PostFormValue("enablepw")
			createdTemplate.Hostname = r.PostFormValue("hostname")
			createdTemplate.Banner = r.PostFormValue("banner")
			createdTemplate.DomainName = r.PostFormValue("domainname")

			// Switchport parsing
			switchPorts := make([]switches.SwitchPortConfig, 0)
			switchPortCount, err := strconv.Atoi(r.PostFormValue("switchports"))
			if err != nil && r.PostFormValue("switchports") != "" {
				webLogger.Errorf("Error while getting the number of switchports: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}

			// Add switch ports to list
			for i := 0; i < switchPortCount; i++ {
				var switchPort switches.SwitchPortConfig

				switchPort.Port = r.PostFormValue(fmt.Sprintf("switchPortName%d", i))
				switchPort.SwitchportMode = r.PostFormValue(fmt.Sprintf("switchPortType%d", i))
				switchPort.Vlan, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("switchPortVlan%d", i)))
				if err != nil && r.PostFormValue(fmt.Sprintf("switchPortVlan%d", i)) != "" {
					webLogger.Errorf("Error while getting the vlan tag for port %s: %s\n", switchPort.Port, err.Error())
					http.Error(w, http.StatusText(500), 500)
					return
				}
				switchPort.Shutdown = r.PostFormValue(fmt.Sprintf("switchPortShutdown%d", i)) == "shutdown"

				switchPorts = append(switchPorts, switchPort)
			}

			createdTemplate.Ports = switchPorts

			// VLAN parsing
			vlans := make([]switches.VlanConfig, 0)
			vlanCount, err := strconv.Atoi(r.PostFormValue("vlan"))
			if err != nil && r.PostFormValue("vlan") != "" {
				webLogger.Errorf("Error while getting the number of vlans: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}

			for i := 0; i < vlanCount; i++ {
				var vlan switches.VlanConfig
				vlan.Vlan, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("vlanTag%d", i)))
				if err != nil {
					webLogger.Errorf("Error while getting the vlan tag on key %s: %s\n", fmt.Sprintf("vlanTag%d", i), err.Error())
					http.Error(w, http.StatusText(500), 500)
					return
				}

				vlan.IpAddress = r.PostFormValue(fmt.Sprintf("vlanIp%d", i))
				vlan.SubnetMask = r.PostFormValue(fmt.Sprintf("vlanSubnetMask%d", i))
				vlan.Shutdown = r.PostFormValue(fmt.Sprintf("vlanShutdown%d", i)) == "shutdown"

				vlans = append(vlans, vlan)
			}

			createdTemplate.Vlans = vlans

			// Console line parsing
			consoleLines := make([]switches.LineConfig, 0)
			consoleLineCount, err := strconv.Atoi(r.PostFormValue("physports"))
			if err != nil && r.PostFormValue("physports") != "" {
				webLogger.Errorf("Error while getting the number of console lines: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}

			for i := 0; i < consoleLineCount; i++ {
				var consoleLine switches.LineConfig
				consoleLine.StartLine, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("portRangeStart%d", i)))
				if err != nil {
					webLogger.Errorf("Error while getting the starting line on key %s: %s\n", fmt.Sprintf("portRangeStart%d", i), err.Error())
					http.Error(w, http.StatusText(500), 500)
					return
				}

				consoleLine.EndLine, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("portRangeEnd%d", i)))
				if err != nil {
					webLogger.Errorf("Error while getting the ending line on key %s: %s\n", fmt.Sprintf("portRangeEnd%d", i), err.Error())
					http.Error(w, http.StatusText(500), 500)
					return
				}

				consoleLine.Type = r.PostFormValue(fmt.Sprintf("portType%d", i))
				consoleLine.Password = r.PostFormValue(fmt.Sprintf("portPassword%d", i))

				if r.PostFormValue(fmt.Sprintf("loginPort%d", i)) == "passwd" {
					consoleLine.Password = r.PostFormValue(fmt.Sprintf("passwordPort%d", i))
				}

				if consoleLine.Type == "vty" {
					consoleLine.Transport = r.PostFormValue(fmt.Sprintf("transportPort%d", i))
				}

				consoleLine.Login = r.PostFormValue(fmt.Sprintf("loginPort%d", i))
				if consoleLine.Login == "passwd" || consoleLine.Login == "noAuth" {
					consoleLine.Login = ""
				}

				consoleLines = append(consoleLines, consoleLine)
			}

			createdTemplate.Lines = consoleLines

			// Parse SSH config
			var sshConfig switches.SshConfig
			sshConfig.Bits, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("sshbits")))
			if err != nil && r.PostFormValue("sshbits") != "" {
				webLogger.Errorf("Error while getting the ssh bits: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}
			sshConfig.Username = r.PostFormValue("sshuser")
			sshConfig.Password = r.PostFormValue("sshpasswd")
			sshConfig.Enable = r.PostFormValue("sshenable") == "enablessh"

			createdTemplate.Ssh = sshConfig

			formattedJson, err = json.Marshal(createdTemplate)
			if err != nil {
				webLogger.Errorf("%s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}
			w.Header().Add("Content-Disposition", "attachment; filename=\"switch_defaults.json\"")
		} else if devType == "router" {
			var createdTemplate routers.RouterDefaults
			createdTemplate.Version = 0.02

			createdTemplate.EnablePassword = r.PostFormValue("enablepw")
			createdTemplate.DomainName = r.PostFormValue("domainname")
			createdTemplate.Banner = r.PostFormValue("banner")
			createdTemplate.Hostname = r.PostFormValue("hostname")
			createdTemplate.DefaultRoute = r.PostFormValue("defaultroute")

			// Build out the list of ports available
			routerPorts := make([]routers.RouterPorts, 0)
			routerPortCount, err := strconv.Atoi(r.PostFormValue("physportcount"))
			if err != nil && r.PostFormValue("physportcount") != "" {
				webLogger.Errorf("Error while getting the number of router ports: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}

			for i := 0; i < routerPortCount; i++ {
				var routerPort routers.RouterPorts
				routerPort.Port = r.PostFormValue(fmt.Sprintf("portName%d", i))
				routerPort.IpAddress = r.PostFormValue(fmt.Sprintf("portIp%d", i))
				routerPort.SubnetMask = r.PostFormValue(fmt.Sprintf("portSubnetMask%d", i))
				routerPort.Shutdown = r.PostFormValue(fmt.Sprintf("portShutdown%d", i)) == "shutdown"

				routerPorts = append(routerPorts, routerPort)
			}

			// Build out the list of console lines
			consoleLines := make([]routers.LineConfig, 0)
			consoleLineCount, err := strconv.Atoi(r.PostFormValue("consoleportcount"))
			if err != nil && r.PostFormValue("consoleportcount") != "" {
				webLogger.Errorf("Error while getting the number of console lines: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}

			// Parse the console lines
			for i := 0; i < consoleLineCount; i++ {
				var consoleLine routers.LineConfig
				consoleLine.StartLine, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("portRangeStart%d", i)))
				if err != nil {
					webLogger.Errorf("Error while getting the starting line on key %s: %s\n", fmt.Sprintf("portRangeStart%d", i), err.Error())
					http.Error(w, http.StatusText(500), 500)
					return
				}

				consoleLine.EndLine, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("portRangeEnd%d", i)))
				if err != nil {
					webLogger.Errorf("Error while getting the ending line on key %s: %s\n", fmt.Sprintf("portRangeEnd%d", i), err.Error())
					http.Error(w, http.StatusText(500), 500)
					return
				}

				consoleLine.Type = r.PostFormValue(fmt.Sprintf("portType%d", i))
				consoleLine.Password = r.PostFormValue(fmt.Sprintf("portPassword%d", i))

				if r.PostFormValue(fmt.Sprintf("loginPort%d", i)) == "passwd" {
					consoleLine.Password = r.PostFormValue(fmt.Sprintf("passwordPort%d", i))
				}

				consoleLine.Login = r.PostFormValue(fmt.Sprintf("loginPort%d", i))
				if consoleLine.Login == "passwd" || consoleLine.Login == "noAuth" {
					consoleLine.Login = ""
				}

				if consoleLine.Type == "vty" {
					consoleLine.Transport = r.PostFormValue(fmt.Sprintf("transportPort%d", i))
				}

				consoleLines = append(consoleLines, consoleLine)
			}
			createdTemplate.Lines = consoleLines

			// Parse SSH settings
			var sshConfig routers.SshConfig
			sshConfig.Bits, err = strconv.Atoi(r.PostFormValue(fmt.Sprintf("sshbits")))
			if err != nil {
				webLogger.Errorf("Error while getting the ssh bits: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}
			sshConfig.Username = r.PostFormValue("sshuser")
			sshConfig.Password = r.PostFormValue("sshpasswd")
			sshConfig.Enable = r.PostFormValue("sshenable") == "enablessh"

			createdTemplate.Ssh = sshConfig

			formattedJson, err = json.Marshal(createdTemplate)
			if err != nil {
				webLogger.Errorf("Error while formatting json: %s\n", err.Error())
				http.Error(w, http.StatusText(500), 500)
				return
			}
			w.Header().Add("Content-Disposition", "attachment; filename=\"router_defaults.json\"")
		}

		w.Header().Add("Content-Length", fmt.Sprintf("%d", len(string(formattedJson))))

		fmt.Fprintf(w, string(formattedJson))

		return
	} else if r.Method == "GET" {
		switch strings.ToLower(devType) {
		case "router":
			builderPage, err = layoutTemplate.Parse(templates.BuilderRouter)
			break
		case "switch":
			builderPage, err = layoutTemplate.Parse(templates.BuilderSwitch)
			break
		default:
			builderPage, err = layoutTemplate.Parse(templates.BuilderHome)
			break
		}

		if err != nil {
			// Log the detailed error
			webLogger.Errorf("An error occurred while parsing the builder template: %s\n", err.Error())
			// Return a generic "Internal Server Error" message
			http.Error(w, http.StatusText(500), 500)
			return
		}

		fmt.Printf("defaults: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

		err = builderPage.ExecuteTemplate(w, "layout", nil)
		if err != nil {
			// Log the detailed error
			webLogger.Errorf("An error occurred while executing the builder template: %s\n", err.Error())
			// Return a generic "Internal Server Error" message
			http.Error(w, http.StatusText(500), 500)
			return
		}
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	webLogger := crglogging.GetLogger(WEB_LOGGER_NAME)

	layoutTemplate, err := template.New("layout").Parse(templates.Layout)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while parsing the layout template in index: %s\n", err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	indexTemplate, err := layoutTemplate.Parse(templates.Index)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while parsing the index template: %s\n", err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	fmt.Printf("serveIndex: %s requested %s\n", r.RemoteAddr, filepath.Clean(r.URL.Path))

	serialPorts, err := enumerator.GetDetailedPortsList()
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while getting the list of serial ports: %s\n", err.Error())
	}

	// Because templates have to only take one struct, we have to have a special struct just for it
	var indexHelper IndexHelper
	indexHelper.Jobs = jobs
	indexHelper.SerialPorts = serialPorts

	err = indexTemplate.ExecuteTemplate(w, "layout", indexHelper)
	if err != nil {
		// Log the detailed error
		webLogger.Errorf("An error occurred while executing the index template: %s\n", err.Error())
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
	muxer.HandleFunc("/api/client/{client}/", newClientApi).Methods("GET", "POST")
	muxer.HandleFunc("/api/jobs/{job}/", clientJobApi).Methods("GET", "POST")
	muxer.HandleFunc("/builder/", builderHome).Methods("GET")
	muxer.HandleFunc("/builder/{device}/", builderHome).Methods("GET", "POST")

	server := &http.Server{
		Handler:      muxer,
		Addr:         "0.0.0.0:8080",
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	webLogger := crglogging.New(WEB_LOGGER_NAME)

	webLogger.Infof("Listening on %s\n", server.Addr)
	webLogger.Fatalf("An error occurred while serving the web server: %s\n", server.ListenAndServe())
}
