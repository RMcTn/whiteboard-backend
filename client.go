package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newLine = []byte{'\n'}
	space   = []byte{' '}
)

type DrawnPointMessage struct {
	Id    uuid.UUID
	Point Point
}

var upgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			//if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			//	log.Printf("error: %v", err)
			//}
			// TODO: Check if it was message too big/read limit exceeded
			log.Printf("error: %v", err)
			c.conn.WriteMessage(websocket.TextMessage, []byte("CLOSING SINCE MESSAGE TOO BIG"))
			break
		}
		// TODO: @FIX The message sent by the client will also be replayed back to themselves, stop this (could this be used as a "received message" on client side to show success?, or alert user to dropped connectivity?)
		message = bytes.TrimSpace(bytes.Replace(message, newLine, space, -1))
//		log.Printf("Message: %s", message)
//		log.Printf("Message length %d", len(message))
		var drawnPointMessage DrawnPointMessage
		err = json.Unmarshal(message, &drawnPointMessage)
		if err != nil {
			log.Printf("Error unmarshalling message %s: %v", message, err)
			// TODO: Should return something here?
			c.conn.WriteMessage(websocket.TextMessage, []byte("Invalid UUID"))
			continue
		}
		c.hub.broadcast <- message

		pointsFormatted := fmt.Sprintf(`{"points": [{"X": %f, "Y": %f}]}`, drawnPointMessage.Point.X, drawnPointMessage.Point.Y)
		line := Line{Id: drawnPointMessage.Id, Points: datatypes.JSON(pointsFormatted), BoardId: c.hub.boardId}
		// TODO: Update updated time to time now
		db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"points": gorm.Expr(`jsonb_set(lines.points::jsonb, array['points'], (lines.points->'points')::jsonb || ?::jsonb)`, fmt.Sprintf(`[{"X": %f, "Y": %f}]`, drawnPointMessage.Point.X, drawnPointMessage.Point.Y))}),
		}).Create(&line)
		//Before gorm on conflict: db.Exec(`INSERT INTO lines(id, points) VALUES(?, ?) ON CONFLICT (id) DO UPDATE SET points = jsonb_set(lines.points::jsonb, array['points'], (lines.points->'points')::jsonb || ?::jsonb)`, drawnPointMessage.Id, fmt.Sprintf(`{ "points": [{"X": %f, "Y": %f}] }`, drawnPointMessage.Point.X, drawnPointMessage.Point.Y), fmt.Sprintf(`[{"X": %f, "Y": %f}]`, drawnPointMessage.Point.X, drawnPointMessage.Point.Y))
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				log.Printf("Error: hub closing channel")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			writer, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			writer.Write(message)

			// Add queued messages to current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				writer.Write(newLine)
				writer.Write(<-c.send)
			}

			if err := writer.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (env *Env) serveWs(boardHubs []*Hub, c *gin.Context) ([]*Hub, error) {
	boardId, err := strconv.Atoi(c.Request.URL.Query()["board"][0])
	if err != nil {
		http.Error(c.Writer, "Requires board id", http.StatusBadRequest)
	}
	log.Printf("Board id: %v", boardId)
	// TODO: Check if board even exists
	var board Board
	err = db.First(&board, boardId).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// TODO: Better http error code?
			http.Error(c.Writer, "Non existent board", http.StatusBadRequest)
			return boardHubs, nil
		}
	}

	// TODO: SPEEDUP: This is quite slow now, too many db calls potentially?
	sessionId, _ := getSessionIdFromCookie(c)
	user, err := env.getUserFromSession(sessionId)

	if err != nil {
		return nil, errors.New("No session for this user");
	}

	boardMember := BoardMember{}

	// TODO: Check if there's a result from this if the query fails
	env.db.First(&boardMember, "board_id = ? AND user_id = ?", boardId, user.ID)

	if boardMember.BoardID == 0 {
		return nil, errors.New(fmt.Sprintf("User %d has no membership for board %d", user.ID, boardId));
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("error: %v", err)
		return boardHubs, nil
	}

	var boardHubToUse *Hub
	// TODO: boardHubs could just be a map?
	for _, boardHub := range boardHubs {
		if boardHub.boardId == boardId {
			boardHubToUse = boardHub
		}
	}
	if boardHubToUse == nil {
		boardHubToUse = newHub(boardId)
		go boardHubToUse.run()
		boardHubs = append(boardHubs, boardHubToUse)
	}
	client := &Client{hub: boardHubToUse, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
	return boardHubs, nil
}
