package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
)

type Server struct {
	conn    *net.Conn
	rCh     chan string
	wCh     chan string
	closeCh chan bool
}

func (connH *Server) closeConn() {
	(*connH.conn).Close()
	connH.closeCh <- true
}

func receivePort(args []string) (port int, ok bool) {
	if len(args) > 2 {
		fmt.Println("Too many command line arguments. Should specify only port.")
		return 0, false
	} else if len(args) < 2 {
		fmt.Println("Port isn't specified as a command line argument. Default port 8080 will be used as a server application port.")
		return 8080, true
	}

	port, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Println("Port entered as a command line argument is invalid. Couldn't convert it to an integer.")
		return 0, false
	}
	return port, true
}

type SafeMap struct {
	m  map[string]*net.Conn
	mx *sync.Mutex
}

var connMap = SafeMap{make(map[string]*net.Conn), &sync.Mutex{}}

func (m *SafeMap) sendAll(msg string) {
	m.mx.Lock()
	defer m.mx.Unlock()
	for _, conn := range connMap.m {
		_, _ = (*conn).Write([]byte(msg))
	}
}

func (m *SafeMap) get(key string) (*net.Conn, bool) {
	m.mx.Lock()
	defer m.mx.Unlock()
	v, ex := m.m[trimNewLine(key)]
	return v, ex
}

func (m *SafeMap) set(key string, c *net.Conn) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.m[trimNewLine(key)] = c
}

func (m *SafeMap) rm(key string) {
	m.mx.Lock()
	defer m.mx.Unlock()
	delete(m.m, trimNewLine(key))
}

func trimNewLine(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func read(reader *bufio.Reader) (string, bool) {
	message, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return "", false
		}
		return "", false
	}
	if trimNewLine(message) == "exit" {
		return "", false
	}
	return message, true
}

func (connH *Server) ListenFromRemote() {
	reader := bufio.NewReader(*connH.conn)
	defer (*connH.conn).Close()

	name := receiveName(reader, connH.conn)
	if len(name) == 0 {
		fmt.Printf("Connection closed for %v\n", name)
		return
	}
	connMap.set(name, connH.conn)
	fmt.Printf("%v joined the server. Total connections: %v\n", name, len(connMap.m))

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error occurred while reading new line from connection.")
		}
		message = message[:len(message)-1]
		if message == "exit" {
			break
		}
		fmt.Println(name + ": " + message)
	}
	fmt.Printf("Connection closed for %v\n", name)
	connMap.rm(name)
	fmt.Printf("%v left the server. Total connections: %v\n", name, len(connMap.m))
}

func listenFromConsole(wCh chan string, closeCh chan bool) {
	reader := bufio.NewReader(os.Stdin)

	for {
		msg, ok := read(reader)
		if !ok {
			closeCh <- true
			return
		}
		wCh <- msg
	}
}

func startSending(wCh chan string) {
	for {
		msg := <-wCh
		connMap.sendAll(msg)
	}
}

func receiveName(r *bufio.Reader, conn *net.Conn) string {
	name, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Printf("Connection closed for %v\n", (*conn).RemoteAddr())
			return ""
		}
		fmt.Println("Error occurred while reading new line from connection.")
	}
	fmt.Println("Check if connection exists.")
	if _, exists := connMap.get(name); exists == true {
		fmt.Printf("Connection with %v exists!\n", name[:len(name)-1])
		_, _ = (*conn).Write([]byte("exists\n"))
		return receiveName(r, conn)
	}
	n, e := (*conn).Write([]byte("ok\n"))
	if n == 0 || e != nil {
		return ""
	}
	return name[:len(name)-1]
}

func waitClosing(closeCh chan bool) {
	<-closeCh
	fmt.Println("Server is closing! Bye!")
	os.Exit(1)
}

func main() {
	port, ok := receivePort(os.Args)
	if ok == false {
		return
	}

	fmt.Printf("Launching GO server at %v\n", port)

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))

	if err != nil {
		fmt.Printf("Error occurred during listening to port %v\n", port)
	}

	rCh := make(chan string)
	wCh := make(chan string)
	closeCh := make(chan bool)

	go startSending(wCh)
	go listenFromConsole(wCh, closeCh)
	go waitClosing(closeCh)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Could't accept the connection during listening to port %v\n", port)
		}
		fmt.Printf("Connection is established with %v\n", conn.RemoteAddr())

		connH := &Server{&conn, rCh, wCh, closeCh}
		go connH.ListenFromRemote()
	}
}
