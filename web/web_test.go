package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
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

type testParams struct {
	name string
	args parameters
	want int
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
}

func closeWebServer() {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequest("GET", "http://localhost:8080", nil)
	if err != nil {
		log.Fatalf("Couldn't create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		log.Fatalf("Couldn't call close web server: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		log.Fatalf("Received unexpected status code %d from web server", resp.StatusCode)
	}

	time.Sleep(1 * time.Minute)
}

func TestIndex(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	t.Cleanup(closeWebServer)

	startWebServer()

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

	t.Cleanup(closeWebServer)

	startWebServer()

	for _, tt := range buildConditions([]string{"/port/", "/port"}, []string{"GET"}) {
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

	t.Cleanup(closeWebServer)

	startWebServer()

	for _, tt := range buildConditions([]string{"/list/ports/", "/list/ports"}, []string{"GET"}) {
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
// Legal paths: /device/, /device/{port}/, /device/{port}/{baud}/{data}/{parity}/{stop}/
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

	t.Cleanup(closeWebServer)

	startWebServer()

	for _, tt := range buildConditions([]string{"/reset/", "/reset"}, []string{"GET", "POST"}) {
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
// Legal paths: /list/jobs/
func TestListJobs(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	t.Cleanup(closeWebServer)

	startWebServer()

	for _, tt := range buildConditions([]string{"/list/jobs/", "/list/jobs"}, []string{"GET"}) {
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

	t.Cleanup(closeWebServer)

	startWebServer()

	for _, tt := range buildConditions([]string{"/builder/", "/builder"}, []string{"GET"}) {
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

	t.Cleanup(closeWebServer)

	startWebServer()

	for _, tt := range buildConditions([]string{"/builder/switch/", "/builder/router/", "/builder/switch", "/builder/router"}, []string{"GET", "POST"}) {
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
