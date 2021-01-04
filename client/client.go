package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

type ConnectionHandler struct {
	Conn    *net.Conn
	RCh     chan string
	WCh     chan string
	CloseCh chan bool
}

func (connH *ConnectionHandler) CloseConn() {
	(*connH.Conn).Close()
	connH.CloseCh <- true
}

func (connH *ConnectionHandler) ListenFromRemote() {
	reader := bufio.NewReader(*connH.Conn)
	defer connH.CloseConn()

	for i := 0; ; i++ {
		message, ok := connH.read(reader)
		if !ok {
			return
		}
		fmt.Println("Server: " + TrimNewLine(message))
		if i == 0 {
			connH.RCh <- message
		}
	}
}

func (connH *ConnectionHandler) ListenFromConsole() {
	reader := bufio.NewReader(os.Stdin)
	defer connH.CloseConn()

	login(connH, reader)
	for {
		message, ok := connH.read(reader)
		if !ok {
			return
		}
		connH.WCh <- message
	}
}

func (connH *ConnectionHandler) StartSending() {
	defer connH.CloseConn()

	for {
		msg := <-connH.WCh
		n, err := fmt.Fprintf(*connH.Conn, msg)
		if n == 0 || err != nil {
			if err == io.EOF {
				return
			}
		}
	}
}

func (connH *ConnectionHandler) read(reader *bufio.Reader) (string, bool) {
	message, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Printf("Connection closed for %v\n", (*connH.Conn).RemoteAddr())
			return "", false
		}
		return "", false
	}
	if TrimNewLine(message) == "exit" {
		fmt.Printf("Connection closed for %v\n", (*connH.Conn).RemoteAddr())
		return "", false
	}
	return message, true
}

func login(connH *ConnectionHandler, r *bufio.Reader) (name string) {
	fmt.Print("Please, introduce yourself. Enter your name: ")
	var response string
	for response != "ok\n" {
		n, err := r.ReadString('\n')
		name = n
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Connection closed for %v", (*connH.Conn).RemoteAddr())
				break
			}
			fmt.Println("Error occurred while reading new line from connection.")
		}
		connH.WCh <- name
		response = <-connH.RCh
		if response == "exists\n" {
			fmt.Print("Username is taken. Please, choose another username: ")
			continue
		}
	}
	fmt.Printf("Welcome, %v!\n", name[:len(name)-1])
	return
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

func TrimNewLine(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func (connH *ConnectionHandler) Handle() {
	go connH.StartSending()
	go connH.ListenFromConsole()
	go connH.ListenFromRemote()

	<-connH.CloseCh
}

func main() {
	port, ok := receivePort(os.Args)
	if ok == false {
		return
	}

	fmt.Printf("Connected to localhost %v\n", port)

	conn, err := net.Dial("tcp", "localhost:"+strconv.Itoa(port))

	if err != nil {
		fmt.Printf("Error occurred during listening to port %v\n", port)
	}

	rCh := make(chan string)
	wCh := make(chan string)
	closeCh := make(chan bool)

	connH := &ConnectionHandler{Conn: &conn, RCh: rCh, WCh: wCh, CloseCh: closeCh}
	connH.Handle()
}
