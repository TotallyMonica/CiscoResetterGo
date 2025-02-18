package crglogging

import (
	"bufio"
	"fmt"
	"github.com/op/go-logging"
	"os"
	"testing"
)

func TestLogToFile(t *testing.T) {
	type args struct {
		filename string
		level    logging.Level
		message  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "LogDebugToFile",
		args: args{
			filename: fmt.Sprintf("/tmp/DEBUG_File_tst.log"),
			level:    logging.DEBUG,
			message:  "Sample Message\n",
		},
		want: `\d+:\d+:\d+\.\d+ .* DEBUG 001 Sample Message`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New("cicd_test")
			logger.NewLogTarget("file", tt.args.filename, true)
			logger.Debug(tt.args.filename, tt.args.level, tt.args.message)

			file, err := os.Open(tt.args.filename)
			if err != nil {
				t.Fatalf("Failed to open log file: %v", err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if scanner.Text() != tt.want {
					t.Errorf("%s got = %v, want %v", tt.name, scanner.Text(), tt.want)
				}
			}
		})
	}
}

func TestLogToMemory(t *testing.T) {
	t.SkipNow()
}
