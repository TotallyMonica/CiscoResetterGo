package web

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

func startWebServer() {
	go ServeWeb()
}

func closeWebServer() error {
	_, err := http.Get("http://127.0.0.1:8080/api/debug/shutdown/")
	return err
}

func TestIndex(t *testing.T) {
	if os.Getenv("ALLOWDEBUGENDPOINTS") != "1" {
		t.Errorf("Could not run, debug environment variable for shutting down safely not set properly")
		t.FailNow()
	}

	startWebServer()

	client := &http.Client{}

	type args struct {
		path   string
		method string
	}

	tests := []struct {
		name string
		args args
		want int
	}{{
		name: "GET /",
		args: args{path: "/", method: "GET"},
		want: http.StatusOK,
	}, {
		name: "POST /",
		args: args{path: "/", method: "POST"},
		want: http.StatusMethodNotAllowed,
	}, {
		name: "DELETE /",
		args: args{path: "/", method: "DELETE"},
		want: http.StatusMethodNotAllowed,
	}, {
		name: "PUT /",
		args: args{path: "/", method: "PUT"},
		want: http.StatusMethodNotAllowed,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.args.method, fmt.Sprintf("http://127.0.0.1:8080/%s", tt.args.path), nil)
			if err != nil {
				t.Errorf("Test %s failed while creating request with error: %s", tt.name, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Test %s failed with error: %s", tt.name, err)
			} else if resp.StatusCode != tt.want {
				t.Errorf("Test %s failed with status code %d, want %d", tt.name, req.Response.StatusCode, tt.want)
			}
		})
	}

	err := closeWebServer()

	if err != nil {
		t.Errorf("Could not shut down web server, error: %s\n", err.Error())
	}
}
