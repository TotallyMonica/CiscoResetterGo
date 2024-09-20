package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.bug.st/serial/enumerator"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

type ClientInfo struct {
	Hostname  string
	LocalAddr string
	Ports     []*enumerator.PortDetails
}

func main() {
	server := "localhost"
	port := 8080
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
	if err != nil {
		log.Fatalf("Error while connecting to %s:%d: %s\n", server, port, err)
	}
	defer conn.Close()

	var clientInfo ClientInfo

	clientInfo.LocalAddr = conn.LocalAddr().String()
	clientInfo.Hostname, err = os.Hostname()
	if err != nil {
		log.Fatalf("Error while getting hostname: %s\n", err)
	}

	clientInfo.Ports, err = enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatalf("Error while getting ports: %s\n", err)
	}

	marshalledBody, err := json.Marshal(clientInfo)
	if err != nil {
		log.Fatalf("Error while marshalling body: %s\n", err)
	}

	proto := "http"
	client, err := http.Post(fmt.Sprintf("%s://%s:%d/api/client/demo/", proto, server, port), "application/json", bytes.NewBuffer(marshalledBody))
	if err != nil {
		log.Fatalf("Error while connecting to %s://%s:%d/api/client/demo/: %s\n", proto, server, port, err)
	}
	defer client.Body.Close()

	resp, err := io.ReadAll(client.Body)
	if err != nil {
		log.Fatalf("Error while reading response body: %s\n", err)
	}

	log.Printf("Response: %s\n", string(resp))
}
