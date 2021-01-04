package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
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
	mx *sync.RWMutex
}

var connMap = SafeMap{make(map[string]*net.Conn), &sync.RWMutex{}}

func (m *SafeMap) sendAll(msg string) {
	m.mx.RLock()
	defer m.mx.RUnlock()
	for _, conn := range connMap.m {
		_, _ = fmt.Fprint(*conn, "Server: "+msg)
	}
}

func (m *SafeMap) sendTo(sender, name, msg string) {
	conn, exists := connMap.get(name)
	if !exists {
		fmt.Printf("Can't send message to %v. Connection with %v doesn't exist.", name, name)
		return
	}
	_, _ = fmt.Fprintf(*conn, sender+": "+msg)
}

func (m *SafeMap) get(key string) (*net.Conn, bool) {
	m.mx.RLock()
	defer m.mx.RUnlock()
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

func parseMsg(msg string) (sender, receiver, body string, ok bool) {
	i := strings.IndexByte(msg, ':')
	if i == -1 {
		return "", "", "", false
	}
	sender = msg[:i]
	rest := strings.TrimSpace(msg[i+1:])
	i = strings.Index(rest, "<-")
	if i == -1 {
		return sender, "", rest, true
	}
	return sender, strings.TrimSpace(rest[:i]), strings.TrimSpace(rest[i+2:]), true
}

func (connH *Server) ListenFromRemote() {
	reader := bufio.NewReader(*connH.conn)
	defer (*connH.conn).Close()

	name := authorize(reader, connH)
	if len(name) == 0 {
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

		s, r, msg, ok := parseMsg(message)
		if ok == false {
			continue
		}
		message = trimNewLine(msg)
		if message == "exit" {
			break
		}
		if len(r) != 0 {
			connMap.sendTo(s, r, msg+"\n")
		} else {
			fmt.Println(s + ": " + message)
		}
	}
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
	extractPrivateName := func(msg string) (name string, rest string, ok bool) {
		i := strings.Index(msg, "<-")
		if i == -1 {
			return "", msg, false
		}
		return strings.TrimSpace(msg[:i]), strings.TrimSpace(msg[i+2:]) + "\n", true
	}
	for {
		msg := <-wCh
		privateName, rest, private := extractPrivateName(msg)
		if !private {
			connMap.sendAll(msg)
		} else {
			connMap.sendTo("Server", privateName, rest)
		}
	}
}

func authorize(r *bufio.Reader, connH *Server) string {
	name, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return ""
		}
		fmt.Println("Error occurred while reading new line from connection.")
	}
	name = trimNewLine(name)
	if _, exists := connMap.get(name); exists == true {
		_, _ = (*connH.conn).Write([]byte("Server: hidden: exists\n"))
		return authorize(r, connH)
	}
	_, _ = (*connH.conn).Write([]byte("Server: hidden: ok\n"))
	return trimNewLine(name)
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
