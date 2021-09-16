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
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	SINGLE
)

type Commands int

const (
	CMD_INSERT string = "/insert"
	CMD_QUERY  string = "/query"
	CMD_SINGLE string = "/single"
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

var connections []net.Conn
var offsets []int64

func main() {
	flag.Parse()

	fmt.Println("Starting server...")

	src := *addr + ":" + strconv.Itoa(*port)
	listener, _ := net.Listen("tcp", src)
	fmt.Printf("Listening on %s.\n", src)

	defer listener.Close()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		quitConnections()
		os.Remove(DB_FILE)
		os.Exit(1)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Some connection error: %s\n", err)
		}

		go handleConnection(c, conn)
	}
}

func periodicFileSyncer(f *os.File) {
	for {
		time.Sleep(10 * time.Millisecond)
		f.Sync()
	}
}

func handleConnection(c chan os.Signal, conn net.Conn) {
	connections = append(connections, conn)
	remoteAddr := conn.RemoteAddr().String()
	fmt.Println("Client connected from " + remoteAddr)

	scanner := bufio.NewScanner(conn)

	var mode ConnectionMode = NONE
	var f *os.File
	var err error

	defer f.Close()

	for {
		ok := scanner.Scan()

		if !ok {
			break
		}

		_mode, data := handleMessage(scanner.Text(), conn)

		switch mode {
		case NONE:
			mode = _mode
			if mode == INSERT {
				f, err = os.OpenFile(DB_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				check(err)
				go periodicFileSyncer(f)
			}
		case INSERT:
			insertData(f, data)
		case QUERY:
			streamRecords(conn, data)
		case SINGLE:
			retrieveSingle(conn, data)
		}
	}

	fmt.Println("Client at " + remoteAddr + " disconnected.")

	if mode == INSERT {
		os.Remove(DB_FILE)
	}
}

func quitConnections() {
	for _, conn := range connections {
		conn.Write([]byte("%quit%\n"))
	}
}

func handleMessage(message string, conn net.Conn) (mode ConnectionMode, data []byte) {
	fmt.Println("> " + message)

	if len(message) > 0 && message[0] == '/' {
		switch {
		case message == CMD_INSERT:
			mode = INSERT
			return

		case strings.HasPrefix(message, CMD_QUERY):
			mode = QUERY

		case message == CMD_SINGLE:
			mode = SINGLE

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

func insertData(f *os.File, data []byte) {
	var length int64 = int64(len(data))

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(length))
	n, err := f.Write(b)
	check(err)
	fmt.Printf("wrote %d bytes\n", n)

	n, err = f.Write(data)
	check(err)
	fmt.Printf("wrote %d bytes\n", n)

	if len(offsets) == 0 {
		offsets = append(offsets, 8+length)
	} else {
		lastOffset := offsets[len(offsets)-1]
		offsets = append(offsets, lastOffset+8+length)
	}
}

func readRecord(f *os.File, seek int64) (b []byte, n int64, err error) {
	n = seek
	l := make([]byte, 8)
	_, err = io.ReadAtLeast(f, l, 8)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return
	}
	n += 8
	check(err)
	length := int(binary.LittleEndian.Uint64(l))

	b = make([]byte, length)
	_, err = io.ReadAtLeast(f, b, length)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		n -= 8
		return
	}
	n += int64(length)
	check(err)
	return
}

func streamRecords(conn net.Conn, data []byte) (err error) {
	var path, value, operator string
	var qs []string

	query := string(data)

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

	var n int64 = 0
	var i int = 0

	for {
		time.Sleep(10 * time.Millisecond)
		f, err := os.Open(DB_FILE)
		if err != nil {
			continue
		}
		f.Seek(n, 0)

		for {
			var b []byte
			b, n, err = readRecord(f, n)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}

			truth, err := JsonPath(path, string(b), value, operator)
			check(err)

			if truth {
				conn.Write([]byte(fmt.Sprintf("%s\n", b)))
			}
		}

		f.Close()
		i++
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

func retrieveSingle(conn net.Conn, data []byte) (err error) {
	index, _ := strconv.Atoi(string(data))
	if index-1 > len(offsets) {
		conn.Write([]byte(fmt.Sprintf("Index out of range: %d\n", index)))
		return
	}
	n := offsets[index]
	f, err := os.Open(DB_FILE)
	check(err)
	f.Seek(offsets[index], 0)
	var b []byte
	b, n, err = readRecord(f, n)
	conn.Write([]byte(fmt.Sprintf("%s\n", b)))
	return
}
