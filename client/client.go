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
	"encoding/json"
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

type Car struct {
	Id    int      `json:"id"`
	Model string   `json:"model"`
	Brand CarBrand `json:"brand"`
	Year  int      `json:"year"`
}

type CarBrand struct {
	Name string `json:"name"`
}

type School struct {
	Id         int          `json:"id"`
	Name       string       `json:"name"`
	League     SchoolLeague `json:"league"`
	Address    string       `json:"address"`
	Enrollment int          `json:"enrollment"`
	Score      float64      `json:"score"`
	Year       int          `json:"year"`
}

type SchoolLeague struct {
	Name string `json:"name"`
}

func main() {
	flag.Parse()

	a := &Car{
		Model: "Camaro",
		Brand: CarBrand{
			Name: "Chevrolet",
		},
		Year: 2021,
	}

	b := &School{
		Name: "Harvard",
		League: SchoolLeague{
			Name: "Ivy",
		},
		Address:    "Massachusetts",
		Enrollment: 5000,
		Score:      4.8,
		Year:       1636,
	}

	// data, _ := json.Marshal(a)

	// fmt.Printf("data: %v\n", string(data))

	// data, _ = json.Marshal(b)

	// fmt.Printf("data: %v\n", string(data))

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

	var data []byte
	for i := 1; i < 101; i++ {
		conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

		if i%2 == 1 {
			b.Id = i
			data, _ = json.Marshal(b)
		} else {
			a.Id = i
			data, _ = json.Marshal(a)
		}

		conn.Write(data)

		conn.Write([]byte("\n"))

		time.Sleep(100 * time.Millisecond)
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
