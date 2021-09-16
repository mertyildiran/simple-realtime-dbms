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
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	jp "github.com/ohler55/ojg/jp"
	oj "github.com/ohler55/ojg/oj"
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

func equ(operand1 string, operand2 string) bool {
	return operand1 == operand2
}

func neq(operand1 string, operand2 string) bool {
	return operand1 != operand2
}

var operations = map[string]interface{}{
	"==": equ,
	"!=": neq,
}

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

		_mode, data := handleMessage(scanner.Text(), conn)

		switch mode {
		case NONE:
			mode = _mode
		case INSERT:
			insertData(data)
		case QUERY:
			streamRecords(conn, data)
		}
	}

	fmt.Println("Client at " + remoteAddr + " disconnected.")

	if mode == INSERT {
		os.Remove(DB_FILE)
	}
}

func handleMessage(message string, conn net.Conn) (mode ConnectionMode, data []byte) {
	fmt.Println("> " + message)

	if len(message) > 0 && message[0] == '/' {
		switch {
		case message == "/insert":
			mode = INSERT
			return

		case strings.HasPrefix(message, "/query"):
			mode = QUERY

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

func streamRecords(conn net.Conn, data []byte) (err error) {
	var path, value, operator string
	var qs []string

	query := string(data)
	// conn.Write([]byte(fmt.Sprintf("query: %s\n", query)))

	for key, _ := range operations {
		if strings.Contains(query, key) {
			operator = key
			qs = strings.Split(query, key)
		}
	}

	if operator == "" {
		err = errors.New("Unidentified operation.")
	}

	path = strings.TrimSpace(qs[0])
	value = strings.TrimSpace(qs[1])
	value = value[1 : len(value)-1]

	fmt.Printf("path: %v\n", path)
	fmt.Printf("value: %v\n", value)
	fmt.Printf("operator: %v\n", operator)

	f, err := os.Open(DB_FILE)
	check(err)
	f.Seek(0, 0)

	for {
		l := make([]byte, 8)
		_, err = io.ReadAtLeast(f, l, 8)
		if err == io.EOF {
			break
		}
		check(err)
		length := int(binary.LittleEndian.Uint64(l))

		b := make([]byte, length)
		_, err = io.ReadAtLeast(f, b, length)
		if err == io.EOF {
			break
		}
		check(err)

		truth, err := JsonPath(path, string(b), value, operator)
		check(err)

		if truth {
			conn.Write([]byte(fmt.Sprintf("%s\n", b)))
		}
	}

	return
}

func JsonPath(path string, text string, ref string, operator string) (truth bool, err error) {
	obj, err := oj.ParseString(text)
	if err != nil {
		return
	}

	x, err := jp.ParseString(path)
	if err != nil {
		return
	}
	result := x.Get(obj)

	var exists bool
	var value string

	if len(result) < 1 {
		exists = false
	} else {
		exists = true
		switch result[0].(type) {
		case string:
			value = result[0].(string)
		case int64:
			value = strconv.FormatInt(result[0].(int64), 10)
		case float64:
			value = strconv.FormatFloat(result[0].(float64), 'f', 6, 64)
		case bool:
			value = strconv.FormatBool(result[0].(bool))
		case nil:
			value = "null"
		default:
			exists = false
		}
	}

	if exists {
		truth = operations[operator].(func(string, string) bool)(value, ref)
	}

	return
}
