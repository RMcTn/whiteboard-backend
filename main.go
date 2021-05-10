package main

import (
	"encoding/json"
	"flag"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func serveHome(c *gin.Context) {
	writer := c.Writer
	request := c.Request
	log.Println(request.URL)
	if request.URL.Path != "/" {
		http.Error(writer, "Not found", http.StatusNotFound)
		return
	}
	if request.Method != "GET" {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(writer, request, "frontend/whiteboard.html")
}

type Point struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

type TestMessage struct {
	Id    uuid.UUID
	Point Point
}

type Line struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Id        uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	Points    datatypes.JSON
}

var db *gorm.DB

func main() {
	// TODO: front end stuff to send x + y object
	// TODO: format check in ws

	// TODO: For "room/board" concept for now, just have 2-3 boards that are joinable by clicking a button
	// TODO: get points persisting to db, prob do it before room/board concept as well
	flag.Parse()
	log.SetFlags(0)

	dsn := "host=localhost user=postgres password=password dbname=whiteboard port=5432 sslmode=disable"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database %v")
	}

	err = db.AutoMigrate(&Line{})
	if err != nil {
		log.Fatalf("Failed to migrate %v: ", err)
	}

	newUuid, _ := uuid.NewUUID()

	db.Create(&Line{Id: newUuid, Points: datatypes.JSON(`{ "points": [ {"X": 31, "Y": 53}, {"X": 153, "Y": 133}, {"X": 431, "Y": 221}] }`)})
	var line Line
	db.First(&line, newUuid)
	db.Model(&Line{}).Where("Id = ?", newUuid).Update("points", []byte(`{ "points": [{"X": 11, "Y": 33}] }`))
	// TODO: Raw SQL query for upserting? Copy from backend socket project?

	log.Printf("Line is: %v", line)
	var testMessage TestMessageAllPointsForUUID
	testMessage.Event = "New connection"

	jsMessage, err := json.Marshal(&line)
	log.Printf("Json is %s", jsMessage)
	var testMessageAllPointsForUUID TestMessageAllPointsForUUID
	testMessageAllPointsForUUID.Event = "New connection"
	err = json.Unmarshal(jsMessage, &testMessageAllPointsForUUID)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	log.Printf("Test message is %v", testMessageAllPointsForUUID)
	jsMessage, err = json.Marshal(&testMessageAllPointsForUUID)
	log.Printf("Test json message is %s", jsMessage)

	db.Exec(`INSERT INTO lines(id, points) VALUES(?, ?) ON CONFLICT (id) DO UPDATE SET points = jsonb_set(lines.points::jsonb, array['points'], (lines.points->'points')::jsonb || '[{"X": 111, "Y": 111}]'::jsonb)`, newUuid, `{ "points": [{"X": 3, "Y": 4}] }`)
	// https://gorm.io/docs/create.html#Upsert-On-Conflict
	//  const pathUpsert = `INSERT INTO ${tableNames.svgObject}(uuid, type, data, board_id) VALUES('${uuid}', 'path', '${data}', '${boardId}') ON CONFLICT (uuid) DO UPDATE SET data = jsonb_set(${tableNames.svgObject}.data::jsonb, array['points'], (${tableNames.svgObject}.data->'points')::jsonb || '[${JSON.stringify(msg.point)}]'::jsonb)`;
	// Problem with gorm onConflict is that new value will overwrite old, rather than appending
	// TODO: Check if gorm (with datatypes json) can append json columns at all

	hub := newHub()
	go hub.run()

	// TODO: Use addr
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Pong",
		})
	})
	r.GET("/ws", func(context *gin.Context) {
		serveWs(hub, context)
	})
	r.GET("/", serveHome)
	r.Run(":8081")
}
