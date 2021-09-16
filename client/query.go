/*
A very simple TCP client written in Go.

This is a toy project that I used to learn the fundamentals of writing
Go code and doing some really basic network stuff.

Maybe it will be fun for you to read. It's not meant to be
particularly idiomatic, or well-written for that matter.
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"time"
)

var host = flag.String("host", "localhost", "The hostname or IP to connect to; defaults to \"localhost\".")
var port = flag.Int("port", 8000, "The port to connect to; defaults to 8000.")
var query = flag.String("query", "", "Query for filtering the records.")

func main() {
	flag.Parse()

	dest := *host + ":" + strconv.Itoa(*port)
	fmt.Printf("Connecting to %s...\n", dest)

	conn, err := net.Dial("tcp", dest)

	if err != nil {
		if _, t := err.(*net.OpError); t {
			fmt.Println("Some problem connecting.")
		} else {
			fmt.Println("Unknown error: " + err.Error())
		}
		os.Exit(1)
	}

	go readConnection(conn)

	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	conn.Write([]byte("/query\n"))

	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	conn.Write([]byte(fmt.Sprintf("%s\n", *query)))

	for {
	}
}

func readConnection(conn net.Conn) {
	for {
		scanner := bufio.NewScanner(conn)

		for {
			ok := scanner.Scan()
			text := scanner.Text()

			command := handleCommands(text)
			if !command {
				fmt.Printf("\b\b** %s\n> ", text)
			}

			if !ok {
				fmt.Println("Reached EOF on server connection.")
				break
			}
		}
	}
}

func handleCommands(text string) bool {
	r, err := regexp.Compile("^%.*%$")
	if err == nil {
		if r.MatchString(text) {

			switch {
			case text == "%quit%":
				fmt.Println("\b\bServer is leaving. Hanging up.")
				os.Exit(0)
			}

			return true
		}
	}

	return false
}
