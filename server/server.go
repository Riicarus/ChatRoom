package server

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"
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

		OnlineUserMap:     make(map[string]*User),
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

	isAlive := make(chan bool)

	go func() {
		// receive msg sent by user
		buf := make([]byte, 4096)

		needToClose := false

		for {
			len, err := conn.Read(buf)

			if len == 0 {
				needToClose = true
			} else if err != nil && err != io.EOF { // there's a io.EOF flag when the client conn's write channel is closed
				fmt.Println("net.Conn Read err: ", err)
				needToClose = true
			}

			if needToClose {
				runtime.Goexit()
			}

			msg := string(buf[:len-1])
			// handle receive msg from client
			user.HandleReceiveClientMessage(msg)

			// set alive at each message receiving option
			isAlive <- true
		}
	}()

	// kick out when not alive for a setting time
	server.KickOutAfterTime(user, isAlive, 300)
}

func (server *Server) PutBroadCastMessage(sender *User, msg string) {
	sendMsg := "[" + sender.Addr + "]" + sender.Name + ": " + msg

	// put msg to broad cast message chan
	server.ServerMessageChan <- sendMsg
}

func (server *Server) BroadCastMessage() {
	for {
		msg := <-server.ServerMessageChan

		// send msg to all online users' chan
		server.mapLock.RLock()
		for _, user := range server.OnlineUserMap {
			user.Chan <- msg
		}
		server.mapLock.RUnlock()
	}
}

func (server *Server) ShowOnlineUsers() []string {
	onlineUsers := make([]string, 0)

	server.mapLock.RLock()
	for name := range server.OnlineUserMap {
		onlineUsers = append(onlineUsers, name)
	}
	server.mapLock.RUnlock()

	return onlineUsers
}

func (server *Server) RenameUser(user *User, newName string) {
	server.mapLock.Lock()
	_, contains := server.OnlineUserMap[newName]
	if contains {
		user.SendMessageToClient("the name has been used")
	} else {
		delete(server.OnlineUserMap, user.Name)
		user.Name = newName
		server.OnlineUserMap[newName] = user
	}
	server.mapLock.Unlock()
}

func (server *Server) KickOutAfterTime(user *User, isAlive chan bool, seconds int) {
	for {
		select {
		case <-isAlive:
			// do nothing
		case <-time.After(time.Second * time.Duration(seconds)):
			user.SendMessageToClient("you are kick out(not alive for long)")
			user.Offline()

			runtime.Goexit()
		}
	}
}
