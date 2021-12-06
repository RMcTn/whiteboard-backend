package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var templates = template.Must(template.ParseGlob("frontend/*.html"))

type Env struct {
	db *gorm.DB
}

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

	board := Board{}
	// TODO: Error handle here
	db.First(&board, c.Query("boardId"))
	err := templates.ExecuteTemplate(writer, "whiteboard.html", board)

	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
//	http.ServeFile(writer, request, "frontend/whiteboard.html")
}

type Point struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

type Line struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Id        uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	Points    datatypes.JSON
	BoardId   int
	Board     Board
}

type Board struct {
	gorm.Model
	BoardName string
}

var db *gorm.DB


func (env *Env) PostBoard(c *gin.Context) {
	boardName := c.PostForm("boardName")
	board := Board{BoardName: boardName}
	env.db.Create(&board)
	log.Printf("New board created %d with name %s", board.ID, board.BoardName)
	c.Redirect(http.StatusFound, fmt.Sprintf("?boardId=%d", board.ID))
}

func (env *Env) NewBoard(c *gin.Context) {
	err := templates.ExecuteTemplate(c.Writer, "newBoard.html", nil)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) GetBoard(c *gin.Context) {
	boardId := c.Params.ByName("boardId")
	log.Printf("BoardId: %s", boardId)
	board := Board{}
	db.First(&board, boardId)
	log.Printf("Boardname %s id %d\n", board.BoardName, board.ID)
	log.Printf("=========================")

	err := templates.ExecuteTemplate(c.Writer, "boardDetails.html", board)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
	// render template
}

func main() {
	// TODO: format check in ws

	// TODO: For "room/board" concept for now, just have 2-3 boards that are joinable by clicking a button
	flag.Parse()
	log.SetFlags(0)

	dsn := "host=localhost user=postgres password=password dbname=whiteboard port=5432 sslmode=disable"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database %v")
	}

	err = db.AutoMigrate(&Board{})
	if err != nil {
		log.Fatalf("Failed to migrate %v: ", err)
	}
	err = db.AutoMigrate(&Line{})
	if err != nil {
		log.Fatalf("Failed to migrate %v: ", err)
	}

	boardHubs := make([]*Hub, 0)

	// TODO: Use addr
	r := gin.Default()
	env := &Env{db: db}
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Pong",
		})
	})
	r.GET("/ws", func(context *gin.Context) {
		// TODO: Surely can just pass a slice?
		boardHubs = serveWs(boardHubs, context)
	})
	r.GET("/", serveHome)
	r.POST("/board", env.PostBoard)
	r.GET("/board", env.NewBoard)
	r.GET("/board/:boardId", env.GetBoard)
	r.Run(":8081")
}
