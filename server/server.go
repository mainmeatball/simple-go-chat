package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"simpleGoChat/chat"
	"strconv"
	"sync"
)

type Server struct {
	conn chat.ConnectionHandler
}

func (connH *Server) StartSending() {
	defer connH.conn.CloseConn()

	fmt.Println("I'm a server and starting to send something.")
	for {
		msg := <-connH.conn.WCh
		n, err := fmt.Fprintf(*connH.conn.Conn, msg)
		if n == 0 || err != nil {
			if err == io.EOF {
				return
			}
		}
	}
}

type SafeMap struct {
	m  map[string]*net.Conn
	mx *sync.Mutex
}

var connMap = SafeMap{make(map[string]*net.Conn), &sync.Mutex{}}

func (m *SafeMap) get(key string) (*net.Conn, bool) {
	m.mx.Lock()
	defer m.mx.Unlock()
	v, ex := m.m[chat.TrimNewLine(key)]
	return v, ex
}

func (m *SafeMap) set(key string, c *net.Conn) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.m[chat.TrimNewLine(key)] = c
}

func (m *SafeMap) rm(key string) {
	m.mx.Lock()
	defer m.mx.Unlock()
	delete(m.m, chat.TrimNewLine(key))
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

func listenMessages(conn *net.Conn) {
	reader := bufio.NewReader(*conn)
	defer (*conn).Close()

	name := receiveName(reader, conn)
	if len(name) == 0 {
		fmt.Printf("Connection closed for %v\n", name)
		return
	}
	connMap.set(name, conn)
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

func main() {
	port, ok := chat.ReceivePort(os.Args)
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

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Could't accept the connection during listening to port %v\n", port)
		}
		fmt.Printf("Connection is established with %v\n", conn.RemoteAddr())

		connH := &chat.ConnectionHandler{Conn: &conn, RCh: rCh, WCh: wCh, CloseCh: closeCh, S: nil}
		connH.S = &Server{conn: *connH}
		connH.Handle()
	}
}
