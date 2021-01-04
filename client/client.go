package main

import (
	"fmt"
	"net"
	"os"
	"simpleGoChat/chat"
	"strconv"
)

func main() {
	port, ok := chat.ReceivePort(os.Args)
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

	var connH chat.Handler = &chat.ConnectionHandler{Conn: &conn, RCh: rCh, WCh: wCh, CloseCh: closeCh}
	connH.Handle()
}
