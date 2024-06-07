package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	//心跳机制专用
	//发起帧
	ping = 50 * time.Second
	//响应帧
	pong = 60 * time.Second
	//写入消息时间
	writeWait = 10 * time.Second
	// 消息大小限制.
	maxMessageSize = 512
)

var Upgrade = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type client struct {
	conn     *websocket.Conn
	username string
	//消息通道
	send chan []byte
}

// 用户递增
var who = 0

type Room struct {
	clients map[*client]bool
}

var (
	lock = sync.Mutex{}
	//多个房间
	rooms = make(map[string]*Room)
)

// 读取消息
func (c *client) Read(r *Room) {
	defer func() {
		log.Printf("user %s exit the room\n", c.username)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pong))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pong)); return nil })
	for {
		msgType, msgByte, err := c.conn.ReadMessage()
		if err != nil {
			// 这里遇到错误一般是断开websocket链接，不管怎样，咱们关闭链接就是
			log.Println(err)
			break
		}
		// 这里只处理一个消息类型
		switch msgType {
		case websocket.TextMessage:
			msg := []byte(fmt.Sprintf("%s %s说:%s",
				time.Now().Format("01/02 03:04"), c.username, string(msgByte)))
			lock.Lock()
			for client := range r.clients {
				select {
				case client.send <- msg:
				default:
					close(client.send)
					delete(r.clients, client)
				}
			}
			lock.Unlock()
		default:
			log.Println("receive don`t know msg type is", msgType)
			continue
		}
	}

}

// 写入消息
func (c *client) Write(r *Room) {
	ticker := time.NewTicker(ping)
	defer func() {
		log.Printf("user %s exit the room\n", c.username)
		ticker.Stop()
		lock.Lock()
		delete(r.clients, c)
		lock.Unlock()
		c.conn.Close()
	}()

	for {
		select {
		case msg := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println(err)
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func wsTest(ctx *gin.Context) {
	roomName := ctx.Query("room")
	//      ?room=xxx
	if rooms[roomName] == nil {
		room := &Room{clients: make(map[*client]bool)}
		rooms[roomName] = room
	}
	//升级服务为websocket
	conn, err := Upgrade.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Println(err)
		ctx.JSON(http.StatusInternalServerError, "upgrade failed")
		return
	}

	//防止并发
	lock.Lock()
	who++
	c := client{
		conn:     conn,
		username: "talker" + strconv.Itoa(who),
		send:     make(chan []byte, 1024),
	}
	rooms[roomName].clients[&c] = true
	lock.Unlock()

	//开两个协程读写消息
	go c.Read(rooms[roomName])
	go c.Write(rooms[roomName])
}
