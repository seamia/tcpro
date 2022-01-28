package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
)

type (
	pipe struct {
		Name   string `json:"name,omitempty"`
		From   string `json:"from"` // 10.1.1.55
		Local  string `json:"thru"` // 10.1.1.1:9999
		Remote string `json:"to"`   // 10.1.1.96:22
	}
)

func main() {

	script := "pipes"
	if len(os.Args) > 1 {
		script = os.Args[1]
	}

	var pipes []pipe
	if data, err := ioutil.ReadFile(script); err == nil {
		if err := json.Unmarshal(data, &pipes); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse file (%s): %v\n", script, err)
			return
		}
	} else {
		fmt.Fprintf(os.Stderr, "failed to open file (%s): %v\n", script, err)
		return
	}

	if len(pipes) > 0 {
		for _, one := range pipes {
			createPipe(one)
		}
		wait()
	}
}

func copy(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}

func wait() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
}

func createPipe(p pipe) {
	listener, err := net.Listen("tcp", p.Local)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conn (%s), failed to listen (%s): %v\n", p.Name, p.Local, err)
		os.Exit(111)
	}

	fmt.Fprintf(os.Stdout, "---------------- (%s) ----------------\n", p.Name)
	go func() {
		defer listener.Close()

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println("error accepting connection", err)
				continue
			}
			log.Println("New connection", conn.RemoteAddr())

			if len(p.From) > 0 {
				if !strings.HasPrefix(conn.RemoteAddr().String(), p.From) {
					log.Println("unauth src ...")
					continue
				}
			}

			go func() {
				defer conn.Close()
				conn2, err := net.Dial("tcp", p.Remote)
				if err != nil {
					log.Println("error dialing remote addr", err)
					return
				}
				defer conn2.Close()
				closer := make(chan struct{}, 2)
				go copy(closer, conn2, conn)
				go copy(closer, conn, conn2)

				<-closer
				log.Println("Connection complete", conn.RemoteAddr())
			}()
		}
	}()
}
