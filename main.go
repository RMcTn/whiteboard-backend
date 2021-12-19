package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var templates = template.Must(template.ParseGlob("frontend/*.html"))

type Env struct {
	db *gorm.DB
	// TODO: Should this be a map to user ID?
	// TODO: Add time to live to the session id 
	sessions map[uuid.UUID]string
	Environment string
}

func (env *Env) getUserFromSession(sessionId uuid.UUID) (User, error) {
		username, ok := env.sessions[sessionId]
		if ok {
			log.Printf("Session %s is valid", sessionId.String())
			user := User{}
			// TODO: Check the err here
			env.db.Where("username = ?", username).First(&user)
			return user, nil
		} else {
			return User{}, errors.New(fmt.Sprintf("Session %s not found", sessionId.String()))
			
		}

}

func getSessionIdFromCookie(c *gin.Context) (uuid.UUID, error) {
	cookie, err := c.Cookie("sessionId")
	if err != nil {
		return uuid.Nil, err
	}
	sessionId, err := uuid.Parse(cookie)
	if err != nil {
		return uuid.Nil, err
	}
	return sessionId, nil
}

func (env *Env) ServeHome(c *gin.Context) {
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
	if c.Query("boardId") != "" {
		db.First(&board, c.Query("boardId"))
	}

	templateVars := map[string]interface{}{}
	sessionId, _ := getSessionIdFromCookie(c)
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		templateVars["loggedIn"] = false
		err = templates.ExecuteTemplate(writer, "whiteboard.html", templateVars)

		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}
		return;
	} else {
		templateVars["loggedIn"] = true
	}
	userIsMemberOfBoard := env.isUserMemberOfBoard(user, board)
	if !userIsMemberOfBoard {
		templateVars["error"] = "You don't have permission for this board"
	} else {
		templateVars["boardName"] = board.BoardName
		templateVars["boardId"] = board.ID
	}

	if board.ID == 0 && user.ID != 0 {
		// User is logged in but hasn't selected a board, not an error
		templateVars["error"] = nil
	}
	templateVars["env"] = env.Environment
	err = templates.ExecuteTemplate(writer, "whiteboard.html", templateVars)

	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
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
	BoardName string `gorm:"not null"`
}

type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	PasswordHash string `gorm:"not null"`
}

type BoardMember struct {
	BoardID uint `gorm:"primaryKey;autoincrement:false"`
	UserID uint `gorm:"primaryKey;autoincrement:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

var db *gorm.DB

func main() {
	// TODO: format check in ws

	// TODO: Auth middleware rather than manual auth in each route
	flag.Parse()
	log.SetFlags(0)

	err := godotenv.Load()

	if err != nil {
		log.Println("Could not load .env file")
	}
	
	db, err = gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{})
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
	err = db.AutoMigrate(&User{})
	if err != nil {
		log.Fatalf("Failed to migrate %v: ", err)
	}
	err = db.AutoMigrate(&BoardMember{})
	if err != nil {
		log.Fatalf("Failed to migrate %v: ", err)
	}

	boardHubs := make([]*Hub, 0)

	r := gin.Default()
	// TODO: Move over to using gin for template rendering
	r.LoadHTMLGlob("frontend/*.html")
	environmentToRun := os.Getenv("ENV")
	if environmentToRun == "" {
		environmentToRun = "dev"
	}

	env := &Env{db: db, sessions: make(map[uuid.UUID]string), Environment: environmentToRun}

	log.Printf("Running in %s mode", env.Environment)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Pong",
		})
	})

	r.GET("/ws", func(context *gin.Context) {
		// TODO: Surely can just pass a slice?
		tempBoardHubs, err := env.serveWs(boardHubs, context)
		if err != nil {
			log.Printf("User couldn't join board")
			err = templates.ExecuteTemplate(context.Writer, "notAuthorized.html", nil)

			if err != nil {
				http.Error(context.Writer, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		boardHubs = tempBoardHubs
	})
	r.GET("/", env.ServeHome)
	r.POST("/board", env.PostBoard)
	r.GET("/board", env.NewBoard)
	r.GET("/board/:boardId", env.GetBoard)
	r.POST("/signup", env.CreateUser)
	r.GET("/signup", env.NewUser)
	r.POST("/signin", env.SignInUser)
	r.GET("/signin", env.SignInPage)
	r.GET("/user/:userId", env.GetUser)
	r.GET("/boards", env.GetBoardsForUser)
	r.POST("/board/:boardId/add_user", env.AddUserToBoard)
	r.POST("/board/:boardId/remove_user", env.RemoveUserFromBoard)
	r.GET("/board/:boardId/members", env.GetBoardMembers)
	port := os.Getenv("PORT")
	r.Run(":" + port)
}
