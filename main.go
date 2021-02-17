package main

import (
		"github.com/gorilla/websocket"
		"github.com/satori/go.uuid"
		"encoding/json"
		"fmt"
		"net/http"
)

// keeps track of the active clients, the messages to be boradcasted, the clients waiting to be registered, and clientes waiting to become unregistered
type ClientManager struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

// structure to keep track of a client's unique ID, socket connection, and messages to be sent
type Client struct {
    id     string
    socket *websocket.Conn
    send   chan []byte
}

// messages send will be in json format to include helpful information rather than a simple string
type Message struct {
    Sender    string `json:"sender,omitempty"`
    Recipient string `json:"recipient,omitempty"`
    Content   string `json:"content,omitempty"`
}

// initialize a client manager for our chat app
var manager = ClientManager{
	clients:	make(map[*Client]bool),
	broadcast:	make(chan []byte),
	register:	make(chan *Client),
	unregister:	make(chan *Client),
}

// routines that the manager will use
// will handle clients to register, clients to unregister, and messages to broadcasts
func (manager *ClientManager) start() {
	// infinite loop
    for {
		// equivalent to a switch statement
        select {
        // get value from manager's register and assign it to conn
        // maps new client to clients map
        case conn := <-manager.register:
            manager.clients[conn] = true
            jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
            manager.send(jsonMessage, conn)
		// get value from manager's unregister and assign it to conn
		// close client, delete it from clients list, delete it, and send a message to the remaining clients that it has been disconnected
        case conn := <-manager.unregister:
            if _, ok := manager.clients[conn]; ok {
                close(conn.send)
                delete(manager.clients, conn)
                jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
                manager.send(jsonMessage, conn)
            }
		// get value from manager's broadcast and assing it to message
		// for each message send it to the recipient, if a message cannot be sent to the client close it and delete it
        case message := <-manager.broadcast:
            for conn := range manager.clients {
                select {
                case conn.send <- message:
                default:
                    close(conn.send)
                    delete(manager.clients, conn)
                }
            }
        }
    }
}

// iterate through clients and send queued messages
func (manager *ClientManager) send(message []byte, ignore *Client) {
    for conn := range manager.clients {
        if conn != ignore {
            conn.send <- message
        }
    }
}

// reads messages to be broadcasted
func (c *Client) read() {
	// send c to unregister and close it after the function returns
    defer func() {
        manager.unregister <- c
        c.socket.Close()
    }()
	
	// infinite loop
    for {
        _, message, err := c.socket.ReadMessage()
        // if an error message exists, then send c to unregister and close it
        if err != nil {
            manager.unregister <- c
            c.socket.Close()
            break
        }
		// create message to be broadcasted/sent
        jsonMessage, _ := json.Marshal(&Message{Sender: c.id, Content: string(message)})
        manager.broadcast <- jsonMessage
    }
}

// sends the client's message
func (c *Client) write() {
	// close the client after the function returns
    defer func() {
        c.socket.Close()
    }()
	
	// infinite loop
    for {
        select {
        // send client's message
        case message, ok := <-c.send:
            if !ok {
				// close the socket message
                c.socket.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
			// send the socket text message
            c.socket.WriteMessage(websocket.TextMessage, message)
        }
    }
}

// handles the http reuests and responses
func wsPage(res http.ResponseWriter, req *http.Request) {
    conn, error := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
    if error != nil {
		fmt.Println("hello")
        http.NotFound(res, req)
        return
    }
    
    // fix for new uuid api
    u, _ := uuid.NewV4()
    
    client := &Client{id: u.String(), socket: conn, send: make(chan []byte)}

    manager.register <- client

    go client.read()
    go client.write()
}


func main() {
    fmt.Println("Starting application...")
    go manager.start()
    http.HandleFunc("/ws", wsPage)
    http.ListenAndServe(":12345", nil)
}
