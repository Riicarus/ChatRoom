package server

import (
	"fmt"
	"net"
	"strings"
)

type User struct {
	Name string
	Addr string
	// msg: server -> user -> client
	Chan chan string
	conn net.Conn

	server *Server
}

// create user and start listen message routine
func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()

	user := &User{
		Name: userAddr,
		Addr: userAddr,
		Chan: make(chan string),
		conn: conn,
		server: server,
	}

	// start a routine to listen message sent to user from server
	go user.ReceiveServerMessage()

	return user
}

func (user *User) Online() {
		// add user to online user map
		user.server.mapLock.Lock()
		user.server.OnlineUserMap[user.Name] = user
		user.server.mapLock.Unlock()
	
		// broad cast user online msg to all online users
		user.SendBroadCastMessage("is online")
}

func (user *User) Offline() {
	// delete user from online user map
	user.server.mapLock.Lock()
	delete(user.server.OnlineUserMap, user.Name)
	user.server.mapLock.Unlock()

	// broad cast user offline msg to all online users
	user.SendBroadCastMessage("is offline")

	// close sources
	user.conn.Close()
	close(user.Chan)
}

func (user *User) HandleReceiveClientMessage(msg string) {
	if msg == "who" {
		// show online users' info to the user
		onlineUsers := user.server.ShowOnlineUsers()
		user.SendMessageToClient(fmt.Sprintf("%v", onlineUsers))
	} else if len(msg) > 7 && msg[:7] == "rename:" {
		// rename user
		newName := strings.Split(msg, ":")[1]
		user.server.RenameUser(user, newName)
	} else {
		user.SendBroadCastMessage(msg)
	}
}

func (user *User) SendBroadCastMessage(msg string) {
	user.server.PutBroadCastMessage(user, msg)
}

func (user *User) SendMessageToClient(msg string) {
	user.conn.Write([]byte(msg + "\n"))
}

// listen user channel, send message to user client when message comes
func (user *User) ReceiveServerMessage() {
	for {
		msg := <- user.Chan

		user.SendMessageToClient(msg)
	}
}