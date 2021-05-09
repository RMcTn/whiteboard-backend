package main

import (
	"encoding/json"
	"github.com/google/uuid"
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

	points map[uuid.UUID][]Point
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		points:     make(map[uuid.UUID][]Point),
	}
}

type TestMessageAllPointsForUUID struct {
	Id     uuid.UUID `json:"id"`
	Points []Point   `json:"points"`
	Event  string    `json:"event"`
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// send the points map
			for k, v := range h.points {
				log.Printf("%v %v", k, v)
				uuidAndPoints := TestMessageAllPointsForUUID{Id: k, Points: v, Event: "New connection"}
				jsonMessage, err := json.Marshal(uuidAndPoints)
				if err != nil {
					log.Printf("Error marshalling uuid and points: %v", err)
					break
				}
				client.send <- jsonMessage
				log.Printf(string(jsonMessage))
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
