package server

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type Server struct {
	Ip   string
	Port int

	// online user map
	OnlineUserMap map[string]*User
	mapLock       sync.RWMutex

	// msg: client -> server -> user
	// broad cast message to all online user's chan
	ServerMessageChan chan string
}

func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:   ip,
		Port: port,

		OnlineUserMap: make(map[string]*User),
		ServerMessageChan: make(chan string),
	}

	return server
}

func (server *Server) Start() {
	// socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.Ip, server.Port))

	if err != nil {
		fmt.Println("net.Listen err: ", err)
	}
	// close listen socket
	defer listener.Close()

	// start a message broad cast routine
	go server.BroadCastMessage()

	for {
		// accept connection
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("net.Listener.Accept err: ", err)
		}

		// do handle
		// use routine to asynchronously handle connection
		// make it able to handle multi conn without being blocked
		go server.HandleConnection(conn)
	}
}

func (server *Server) HandleConnection(conn net.Conn) {
	user := NewUser(conn, server)

	user.Online()

	// receive msg sent by user
	buf := make([]byte, 4096)
	for {
		len, err := conn.Read(buf)
		if len == 0 {
			user.Offline()
			return
		}

		// there's a io.EOF flag when reading to a message's end
		if err != nil && err != io.EOF {
			fmt.Println("net.Conn Read err: ", err)
			user.Offline()
			return
		}

		msg := string(buf[: len - 1])
		// put msg to broad cast chan
		user.SendMessage(msg)
	}
}

func (server *Server) PutBroadCastMessage(sender *User, msg string) {
	sendMsg := "[" + sender.Addr + "]" + sender.Name + ": " + msg
	
	// put msg to broad cast message chan
	server.ServerMessageChan <- sendMsg
}

func (server *Server) BroadCastMessage() {
	for {
		msg := <- server.ServerMessageChan

		// send msg to all online users' chan
		server.mapLock.RLock()
		for _, user := range server.OnlineUserMap {
			user.Chan <- msg
		}
		server.mapLock.RUnlock()
	}
}
