/*
A very simple TCP server written in Go.

This is a toy project that I used to learn the fundamentals of writing
Go code and doing some really basic network stuff.

Maybe it will be fun for you to read. It's not meant to be
particularly idiomatic, or well-written for that matter.
*/
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
)

var addr = flag.String("addr", "", "The address to listen to; default is \"\" (all interfaces).")
var port = flag.Int("port", 8000, "The port to listen on; default is 8000.")

type ConnectionMode int

const (
	NONE ConnectionMode = iota
	INSERT
	QUERY
)

const DB_FILE string = "data.bin"

func main() {
	flag.Parse()

	fmt.Println("Starting server...")
	// os.Remove(DB_FILE)

	src := *addr + ":" + strconv.Itoa(*port)
	listener, _ := net.Listen("tcp", src)
	fmt.Printf("Listening on %s.\n", src)

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Some connection error: %s\n", err)
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	fmt.Println("Client connected from " + remoteAddr)

	scanner := bufio.NewScanner(conn)

	var mode ConnectionMode = NONE

	for {
		ok := scanner.Scan()

		if !ok {
			break
		}

		_mode, _, data := handleMessage(scanner.Text(), conn)

		switch mode {
		case NONE:
			mode = _mode
		case INSERT:
			insertData(data)
		}
	}

	fmt.Println("Client at " + remoteAddr + " disconnected.")

	if mode == INSERT {
		os.Remove(DB_FILE)
	}
}

func handleMessage(message string, conn net.Conn) (mode ConnectionMode, query string, data []byte) {
	fmt.Println("> " + message)

	if len(message) > 0 && message[0] == '/' {
		switch {
		case message == "/insert":
			mode = INSERT
			return

		case message == "/quit":
			fmt.Println("Quitting.")
			conn.Write([]byte("I'm shutting down now.\n"))
			fmt.Println("< " + "%quit%")
			conn.Write([]byte("%quit%\n"))
			os.Exit(0)

		default:
			conn.Write([]byte("Unrecognized command.\n"))
		}
	} else {
		// if err := json.Unmarshal([]byte(message), &data); err != nil {
		// 	panic(err)
		// }
		// fmt.Printf("data: %v\n", data)
		data = []byte(message)
	}

	return
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func insertData(data []byte) {
	f, err := os.OpenFile(DB_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	check(err)

	defer f.Close()

	var len int64 = int64(len(data))

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(len))
	n, err := f.Write(b)
	check(err)
	fmt.Printf("wrote %d bytes\n", n)

	n, err = f.Write(data)
	check(err)
	fmt.Printf("wrote %d bytes\n", n)

	f.Sync()
}
