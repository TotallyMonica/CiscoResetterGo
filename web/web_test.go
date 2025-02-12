package web

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type parameters struct {
	proto  string
	host   string
	port   int
	path   string
	method string
}

type resetParams struct {
	Port   string `json:"port"`
	Baud   int    `json:"baud"`
	Data   int    `json:"data"`
	Parity string `json:"parity"`
	Stop   string `json:"stop"`

	Device  string `json:"string"`
	Verbose string `json:"string"`
	Reset   string `json:"reset"`
}

type portsAvailable struct {
	port string
	used bool
}

type testParams struct {
	name string
	args parameters
	want int
}

const TOTAL_TESTS = 7

var currentTest = 0

func makeDummySerial(stdout chan string, term chan bool) {
	reader, writer := io.Pipe()

	go func() {
		sentAlready := false

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
			if strings.Contains(scanner.Text(), "/dev/") && !sentAlready {
				delimited := strings.Split(scanner.Text(), " ")
				for _, word := range delimited {
					if strings.Contains(word, "/dev/") {
						stdout <- word
						sentAlready = true
						break
					}
				}
			}
		}
	}()

	cmd := exec.Command("socat", "-d", "-d", "pty,raw,echo=0", "pty,raw,echo=0")
	cmd.Stderr = writer
	_ = cmd.Start()

	go func() {
		switch {
		case <-term:
			fmt.Printf("Terminating\n")
			cmd.Process.Kill()
		}
	}()
}

func buildConditions(paths []string, allowedMethods []string) []testParams {
	methodList := []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"}
	hostList := []string{"localhost", "127.0.0.1", "[::1]"}
	protoList := []string{"http"}
	portList := []int{8080}

	tests := make([]testParams, 0)

	for _, path := range paths {
		for _, method := range methodList {
			for _, host := range hostList {
				for _, proto := range protoList {
					for _, port := range portList {
						added := false
						params := parameters{
							proto:  proto,
							host:   host,
							port:   port,
							path:   path,
							method: method,
						}
						for _, allowed := range allowedMethods {
							if method == allowed {
								tests = append(tests, testParams{
									name: fmt.Sprintf("%s %s://%s:%d%s", method, proto, host, port, path),
									want: http.StatusOK,
									args: params,
								})
								added = true
							}
						}

						if !added {
							tests = append(tests, testParams{
								name: fmt.Sprintf("%s %s://%s:%d%s", method, proto, host, port, path),
								want: http.StatusMethodNotAllowed,
								args: params,
							})
						}
					}
				}
			}
		}
	}

	return tests
}

func startWebServer() {
	go ServeWeb()
	time.Sleep(100 * time.Millisecond)
}

func closeWebServer(t *testing.T) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequest("GET", "http://localhost:8080", nil)
	if err != nil {
		t.Errorf("Couldn't create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		t.Errorf("Couldn't call close web server: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		t.Errorf("Received unexpected status code %d from web server", resp.StatusCode)
	}
}

func TestIndex(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}

	for _, tt := range buildConditions([]string{"/", ""}, []string{"GET"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), nil)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}

func TestPortConfig(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}

	for _, tt := range buildConditions([]string{"/port/"}, []string{"GET"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), nil)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}

// Legal methods: GET
// Legal paths: /list/ports/
func TestListPorts(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}

	for _, tt := range buildConditions([]string{"/list/ports/"}, []string{"GET"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), nil)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}

// Legal methods: GET, POST
// Legal paths: /device/, /device/{port}/, /device/{port}/{baud}/{{data}/{parity}/{stop}/
func TestDeviceConfig(t *testing.T) {
	t.SkipNow()
}

// Legal methods: GET, POST
// Legal paths: /reset/
func TestReset(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}

	for _, tt := range buildConditions([]string{"/reset/"}, []string{"POST"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)

		devUsed := "/dev/ttyS0"
		stdoutChan := make(chan string)
		killSwitch := make(chan bool)
		if tt.want == http.StatusOK {
			go makeDummySerial(stdoutChan, killSwitch)
			devUsed = <-stdoutChan
		}

		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Minute * 1,
			}

			// Build out body
			var b bytes.Buffer
			w := multipart.NewWriter(&b)

			keys := []string{"device", "verbose", "reset", "port", "baud", "data", "parity", "stop"}
			values := []string{"router", "verbose", "reset", devUsed, "9600", "8", "no", "1"}
			for idx, key := range keys {
				fw, err := w.CreateFormField(key)
				if err != nil {
					t.Errorf("Test failed while creating key values: %s\n", err)
				}

				_, err = io.Copy(fw, bytes.NewReader([]byte(values[idx])))
				if err != nil {
					t.Errorf("Test failed while adding values to keys: %s\n", err)
				}
			}

			w.Close()

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), &b)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			req.Header.Add("Content-Type", w.FormDataContentType())

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}

// Legal methods: GET
// Legal paths: /list/jobs/
func TestListJobs(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}

	for _, tt := range buildConditions([]string{"/list/jobs/"}, []string{"GET"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), nil)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}

// Legal methods: GET
// Legal paths: /jobs/{id}/
func TestJobAccess(t *testing.T) {
	t.SkipNow()
}

// Legal methods: GET, POST
// Legal paths: /api/client/{client}/
func TestApiClient(t *testing.T) {
	t.SkipNow()
}

// Legal methods: GET, POST
// Legal paths: /api/jobs/{job}/
func TestApiJobs(t *testing.T) {
	t.SkipNow()
}

// Legal methods: GET
// Legal paths: /builder/
func TestBuilder(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}
	for _, tt := range buildConditions([]string{"/builder/"}, []string{"GET"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), nil)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}

// Legal methods: GET, POST
// Legal paths: /builder/{device}/
func TestBuilderDevice(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	if currentTest == 0 {
		startWebServer()
	}
	currentTest += 1
	if currentTest == TOTAL_TESTS {
		t.Cleanup(func() {
			closeWebServer(t)
		})
	}

	for _, tt := range buildConditions([]string{"/builder/switch/", "/builder/router/"}, []string{"GET", "POST"}) {
		t.Logf("Testing full path: %s %s://%s:%d%s", tt.args.method, tt.args.proto, tt.args.host, tt.args.port, tt.args.path)
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			client := &http.Client{
				Timeout: time.Second * 10,
			}

			// Build out body
			var body io.Reader

			if tt.args.path == "/builder/switch/" {
				body = bytes.NewReader([]byte("vlan=2&vlanTag0=10&vlanIp0=192.168.10.2&vlanSubnetMask0=255.255.255.0&vlanTag1=20&vlanIp1=192.168.20.2&vlanSubnetMask1=255.255.255.0&vlanShutdown1=shutdown&switchports=3&switchPortName0=GigabitEthernet0%2F1&switchPortType0=access&switchPortVlan0=10&switchPortShutdown0=shutdown&switchPortName1=GigabitEthernet0%2F2&switchPortType1=trunk&switchPortVlan1=10&switchPortName2=GigabitEthernet0%2F3&switchPortType2=access&switchPortVlan2=20&physports=2&portType0=console&portRangeStart0=0&portRangeEnd0=0&loginPort0=passwd&transportPort0=ssh%26telnet&passwordPort0=ABcd1234&portType1=vty&portRangeStart1=0&portRangeEnd1=15&loginPort1=local&transportPort1=ssh&passwordPort1=ABcd1234&gateway=192.168.10.1&enablepw=ABcd1234&domainname=pb218.lab&hostname=BenchSwitch&banner=Unauthorized+Access+Only%21&sshbits=2048&sshuser=admin&sshpasswd=ABcd1234&sshenable=enablessh"))
			} else if tt.args.path == "/builder/router/" {
				body = bytes.NewReader([]byte("physportcount=2&portName0=GigabitEthernet0%2F0%2F0&portIp0=192.168.10.1&portSubnetMask0=255.255.255.0&portName1=GigabitEthernet0%2F0%2F1&portIp1=192.168.20.1&portSubnetMask1=255.255.255.0&portShutdown1=shutdown&consoleportcount=2&portType0=console&portRangeStart0=0&portRangeEnd0=0&loginPort0=passwd&transportPort0=ssh%26telnet&passwordPort0=ABcd1234&portType1=vty&portRangeStart1=0&portRangeEnd1=4&loginPort1=local&transportPort1=ssh&passwordPort1=ABcd1234&enablepw=ABcd1234&domainname=pb218.lab&hostname=BenchRtr&banner=Unauthorized+Access+Only%21&defaultroute=GigabitEthernet0%2F0%2F0&sshbits=2048&sshuser=admin&sshpasswd=ABcd1234&sshenable=enablessh"))
			}

			// Build out request
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("%s://%s:%d%s", tt.args.proto, tt.args.host, tt.args.port, tt.args.path), body)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}

			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, resp.StatusCode, tt.want)
			}
		})
	}
}
