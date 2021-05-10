package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"log"
)

type Hub struct {
	// TODO: Hub to become each "board room" and send points to clients connected there
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

type TestMessageAllPointsForUUID struct {
	Id    uuid.UUID      `json:"id"`
	Data  datatypes.JSON `json:"data"`
	Event string         `json:"event"`
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// Grab all points from DB and send
			var lines []Line
			db.Find(&lines)
			for _, l := range lines {
				log.Printf("Line is %v", l)
				log.Printf("Line points is %s", l.Points)
				uuidAndPoints := TestMessageAllPointsForUUID{Id: l.Id, Data: l.Points, Event: "New connection"}
				jsonMessage, err := json.Marshal(uuidAndPoints)
				if err != nil {
					log.Printf("Error marshalling uuid and points: %v", err)
					break
				}
				log.Printf("Sending message to clients %s", jsonMessage)
				client.send <- jsonMessage

			}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
