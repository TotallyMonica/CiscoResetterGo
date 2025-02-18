package common

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/pin/tftp/v3"
	"io"
	"os"
	"testing"
	"time"
)

func tftpClient(filename string, closeChan chan bool) error {
	// Get test ready
	err := os.Remove(filename + "_recv")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("TFTP Client test while preparing test: %s\n", err)
	}

	// Initialize test
	tftpServer := "127.0.0.1:69"
	c, err := tftp.NewClient(tftpServer)
	if err != nil {
		return fmt.Errorf("TFTP Client test failed while connecting to server %s. Details: %s\n", tftpServer, err)
	}
	c.SetTimeout(5 * time.Second)

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("TFTP Client test failed while opening file. Details: %s\n", err)
	}
	defer file.Close()

	// Send file
	rf, err := c.Send(filename+"_recv", "octet")
	if err != nil {
		return fmt.Errorf("TFTP Client test failed while sending file. Details: %s\n", err)
	}
	_, err = rf.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("TFTP Client test failed while reading file. Details: %s\n", err)
	}

	closeChan <- true
	return nil
}

func compareTftpFiles(filename string) error {
	src, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("TFTP file validation test failed while opening source file %s. Details: %s\n", filename, err)
	}
	defer src.Close()

	dst, err := os.Open(filename + "_recv")
	if err != nil {
		return fmt.Errorf("TFTP file validation test failed while opening destination file %s_recv. Details: %s\n", filename, err)
	}
	defer dst.Close()

	srcContents, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("TFTP file validation test failed while reading source file %s. Details: %s\n", src.Name(), err)
	}

	dstContents, err := io.ReadAll(dst)
	if err != nil {
		return fmt.Errorf("TFTP file validation test failed while reading destination file %s. Details: %s\n", dst.Name(), err)
	}

	if bytes.Compare(srcContents, dstContents) != 0 {
		return fmt.Errorf("Source contents from file %s differs from destination file %s\n", src.Name(), dst.Name())
	}

	return nil
}

//func TestBuiltInTftpServer(t *testing.T) {
//	type args struct {
//		close chan bool
//	}
//	tests := []struct {
//		name string
//		args args
//		file string
//	}{
//		{
//			name: "Write File",
//			args: args{
//				close: make(chan bool),
//			},
//			file: "T:\\Dev\\CiscoResetterGo\\switch_defaults.json",
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			go BuiltInTftpServer(tt.args.close)
//			tftpClient(tt.file, tt.args.close)
//		})
//	}
//}

func TestIsSyslog(t *testing.T) {
	type args struct {
		output string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		"PKI timers",
		args{
			"Nov  6 19:26:59.869: %PKI-2-NON_AUTHORITATIVE_CLOCK: PKI timers have not been initialized due to non-authoritative system clock. Ensure system clock is configured/updated.",
		},
		true,
	}, {
		"IOS XE Platform",
		args{
			"*Nov  6 19:24:47.888: %IOSXE-3-PLATFORM: F0: kernel: dash_c2w_op_done: I2C Master time out: status register - 0x0",
		},
		true,
	}, {
		"Link state down: GigabitEthernet0/0/0",
		args{
			"*Nov  6 19:26:48.860: %LINK-3-UPDOWN: Interface GigabitEthernet0/0/0, changed state to down",
		},
		true,
	}, {
		"Link state down: GigabitEthernet0/0/1",
		args{
			"*Nov  6 19:26:48.860: %LINK-3-UPDOWN: Interface GigabitEthernet0/0/0, changed state to down",
		},
		true,
	}, {
		"Line protocol down: GigabitEthernet0/0/0",
		args{
			"*Nov  6 21:49:50.976: %LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet0/0/0, changed state to down",
		},
		true,
	}, {
		"Line protocol down: GigabitEthernet0/0/1",
		args{
			"*Nov  6 21:49:50.976: %LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet0/0/1, changed state to down",
		},
		true,
	}, {
		"Link state up: GigabitEthernet0/0/0",
		args{
			"*Nov  6 21:45:51.955: %LINK-3-UPDOWN: Interface GigabitEthernet0/0/0, changed state to up",
		},
		true,
	}, {
		"Link state up: GigabitEthernet0/0/1",
		args{
			"*Nov  6 21:45:51.955: %LINK-3-UPDOWN: Interface GigabitEthernet0/0/1, changed state to up",
		},
		true,
	}, {
		"Line protocol up: GigabitEthernet0/0/0",
		args{
			"*Nov  6 21:45:52.956: %LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet0/0/0, changed state to up",
		},
		true,
	}, {
		"Line protocol up: GigabitEthernet0/0/1",
		args{
			"*Nov  6 21:45:52.956: %LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet0/0/1, changed state to up",
		},
		true,
	}, {
		"Encrypted private config file",
		args{
			"*Nov  6 21:17:38.691: %SYS-2-PRIVCFG_ENCRYPT: Successfully encrypted private config file",
		},
		true,
	}, {
		"Reload requested",
		args{
			"*Nov  6 21:18:42.901: %SYS-5-RELOAD: Reload requested by console. Reload Reason: Reload Command.",
		},
		true,
	}, {
		"NVRam Erased",
		args{
			"*Nov  6 21:15:38.133: %SYS-7-NV_BLOCK_INIT: Initialized the geometry of nvram",
		},
		true,
	}, {
		"Configuration change",
		args{
			"*Nov  6 21:15:29.667: %SYS-5-CONFIG_I: Configured from console by console",
		},
		true,
	}, {
		"Non-authoritative time prefix",
		args{
			"*Nov  6 19:26:59.869: %PKI-2-NON_AUTHORITATIVE_CLOCK: PKI timers have not been initialized due to non-authoritative system clock. Ensure system clock is configured/updated.",
		},
		true,
	}, {
		"NTP not synchronized prefix",
		args{
			"Nov  6 19:26:59.869: %PKI-2-NON_AUTHORITATIVE_CLOCK: PKI timers have not been initialized due to non-authoritative system clock. Ensure system clock is configured/updated.",
		},
		true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSyslog(tt.args.output); got != tt.want {
				t.Errorf("IsSyslog() = %v, want %v", got, tt.want)
			}
		})
	}
}
