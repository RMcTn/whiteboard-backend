package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var templates = template.Must(template.ParseGlob("frontend/*.html"))

type Env struct {
	db *gorm.DB
	// TODO: Should this be a map to user ID?
	// TODO: Add time to live to the session id 
	sessions map[uuid.UUID]string
}

func (env *Env) getUserFromSession(sessionId uuid.UUID) (User, error) {
		email, ok := env.sessions[sessionId]
		if ok {
			log.Printf("Session %s is valid", sessionId.String())
			user := User{}
			// TODO: Check the err here
			env.db.Where("email = ?", email).First(&user)
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
	// TODO: Has many users
	// TODO: Has an owner
}

type User struct {
	gorm.Model
	Email string `gorm:"unique;not null"`
	PasswordHash string `gorm:"not null"`
}

type BoardMember struct {
	BoardID uint `gorm:"primaryKey;autoincrement:false"`
	UserID uint `gorm:"primaryKey;autoincrement:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

var db *gorm.DB


func (env *Env) PostBoard(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		panic(err)
	}
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}

	boardName := c.PostForm("boardName")
	board := Board{BoardName: boardName}
	// TODO: Check for errors
	env.db.Create(&board)
	log.Printf("New board created %d with name %s", board.ID, board.BoardName)
	BoardMember := BoardMember{BoardID: board.ID, UserID: user.ID}
	// TODO: Check for errors
	env.db.Create(&BoardMember)
	c.Redirect(http.StatusFound, fmt.Sprintf("?boardId=%d", board.ID))
}

func (env *Env) NewBoard(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		panic(err)
	}
	_, err = env.getUserFromSession(sessionId)
	if err != nil {
		// TODO: Doesn't redirect due to turbo
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	err = templates.ExecuteTemplate(c.Writer, "newBoard.html", nil)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) GetBoard(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}

	boardId := c.Params.ByName("boardId")
	boardMember := BoardMember{}
// TODO: Check if there's a result from this if the query fails
	env.db.First(&boardMember, "board_id = ? AND user_id = ?", boardId, user.ID)

	if boardMember.BoardID == 0 {
		err = templates.ExecuteTemplate(c.Writer, "notAuthorized.html", nil)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		}
		return
	}


	log.Printf("BoardId: %s", boardId)
	board := Board{}
	db.First(&board, boardId)
	log.Printf("Boardname %s id %d\n", board.BoardName, board.ID)
	log.Printf("=========================")

	err = templates.ExecuteTemplate(c.Writer, "boardDetails.html", board)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) CreateUser(c *gin.Context) {
	// TODO: Validate email
	email := c.PostForm("email")

	// TODO: Validate password
	password := c.PostForm("password")
	// TODO: handle this error
	sessionId, err := getSessionIdFromCookie(c)
	user, err := env.getUserFromSession(sessionId)
	if err == nil {
		userUrl := fmt.Sprintf("/user/%d", user.ID)
		c.Redirect(http.StatusFound, userUrl)
		return
	}
	// TODO: Handle err
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user = User{Email: email, PasswordHash: string(passwordHash)}
	env.db.Create(&user)
	log.Printf("New user created: %s", user.Email)

	sessionId = uuid.New()
	env.sessions[sessionId] = email
	c.SetCookie("sessionId", sessionId.String(), 0, "/", c.Request.Host, false, false)

	userUrl := fmt.Sprintf("/user/%d", user.ID)
	c.Redirect(http.StatusFound, userUrl)
}

func (env *Env) NewUser(c *gin.Context) {
	cookie, err := c.Cookie("sessionId")

	if err != nil {
		log.Println("No existing session id from cookies")
	} else {
		log.Printf("Got cookie %s", cookie)
		sessionId, _ := uuid.Parse(cookie)
		user, err := env.getUserFromSession(sessionId)
		if err == nil {
			userUrl := fmt.Sprintf("/user/%d", user.ID)
			c.Redirect(http.StatusFound, userUrl)
			return
		}
		log.Printf("Could not find valid session for %s", cookie)
	}
	err = templates.ExecuteTemplate(c.Writer, "newUser.html", nil)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) SignInUser(c *gin.Context) {
	// TODO: Validate email
	email := c.PostForm("email")

	// TODO: Validate password
	password := c.PostForm("password")


	user := User{}
	env.db.Where("email = ?", email).First(&user)
	log.Println(user)

	// TODO: Handle err
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusUnauthorized)
		return
	}
	log.Println("Success logging in")

	sessionId := uuid.New()
	env.sessions[sessionId] = email


	c.SetCookie("sessionId", sessionId.String(), 0, "/", c.Request.Host, false, false)
	c.Redirect(http.StatusFound, fmt.Sprintf("/user/%d", user.ID))
}

func (env *Env) SignInPage(c *gin.Context) {
	cookie, err := c.Cookie("sessionId")

	if err != nil {
		log.Println("No existing session id from cookies")
	} else {
		log.Printf("Got cookie %s", cookie)
		sessionId, _ := uuid.Parse(cookie)
		user, err := env.getUserFromSession(sessionId)
		if err == nil {
			userUrl := fmt.Sprintf("/user/%d", user.ID)
			c.Redirect(http.StatusFound, userUrl)
			return
		}
		log.Printf("Could not find valid session for %s", cookie)
	}
	err = templates.ExecuteTemplate(c.Writer, "signIn.html", nil)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) GetUser(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		err = templates.ExecuteTemplate(c.Writer, "notAuthorized.html", nil)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	userId := c.Params.ByName("userId")

	// TODO: Error check this conversion
	userIdConv, _ := strconv.Atoi(userId)
	if user.ID != uint(userIdConv){
		err = templates.ExecuteTemplate(c.Writer, "notAuthorized.html", nil)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	log.Printf("UserId: %s", userId)
	user = User{}
	db.First(&user, userId)
	log.Printf("User %s id %d\n", user.Email, user.ID)
	log.Printf("=========================")

	err = templates.ExecuteTemplate(c.Writer, "user.html", user)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) GetBoardsForUser(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		panic(err)
	}
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}

	var results []map[string]interface{}

	env.db.Table("board_members").Select("boards.ID", "boards.board_name").Joins("JOIN boards on boards.id = board_members.board_id").Where("board_members.user_id = ?", user.ID).Find(&results)

	err = templates.ExecuteTemplate(c.Writer, "boards.html", results)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) AddUserToBoard(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	// Check if requesting user is member of the board

	boardMember := BoardMember{}

	// TODO: handle this err
	boardId, err := strconv.Atoi(c.Params.ByName("boardId"))

	// TODO: Check if there's a result from this if the query fails
	env.db.First(&boardMember, "board_id = ? AND user_id = ?", boardId, user.ID)

	if boardMember.BoardID == 0 {
		err = templates.ExecuteTemplate(c.Writer, "notAuthorized.html", nil)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// Get user id from form post
	// TODO: handle this err
	usernameToAdd := c.PostForm("userToAdd")
	

	userToAdd := User{}
	// TODO: If a user can't be found, user ID 0 will be added to the boardMember FIX
	env.db.First(&userToAdd, "email = ?", usernameToAdd)
	boardMember = BoardMember{}
	env.db.First(&boardMember, "board_id = ? AND user_id = ?", boardId, userToAdd.ID)
	// Check if user is already a member
	if boardMember.BoardID == 0 {
		// Create board member
		
		BoardMember := BoardMember{BoardID: uint(boardId), UserID: userToAdd.ID}
		// TODO: Check for errors
		env.db.Create(&BoardMember)
		log.Printf("Added user %d to board %d", userToAdd.ID, boardId)
		c.Redirect(http.StatusFound, fmt.Sprintf("/board/%d/members", boardId))
		return
	} else {
		log.Printf("User %d is already a member of board %d", userToAdd.ID, boardId)
	}
}

func (env *Env) GetBoardMembers(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	// TODO: Handle this error and make convienence function for getting board id from params
	boardId, _ := strconv.Atoi(c.Params.ByName("boardId"))
	board := Board{}
	board.ID = uint(boardId)
	userIsMemberOfBoard := env.isUserMemberOfBoard(user, board)

	if !userIsMemberOfBoard {
		err = templates.ExecuteTemplate(c.Writer, "notAuthorized.html", nil)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	rows, err := env.db.Table("users").Select("email").Joins("JOIN board_members on users.id = board_members.user_id").Where("board_members.board_id = ?", board.ID).Rows()
	usernames := []string{}
	for rows.Next() {
		// Want usernames here
		username := ""
		rows.Scan(&username)
		usernames = append(usernames, username)
	}
	err = templates.ExecuteTemplate(c.Writer, "boardMembers.html", usernames)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (env *Env) isUserMemberOfBoard(user User, board Board) bool {
	boardMember := BoardMember{}

	if result := env.db.First(&boardMember, "board_id = ? AND user_id = ?", board.ID, user.ID); result.Error != nil {
		return false
	}
	return true
}

func (env *Env) RemoveUserFromBoard(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	// Check if requesting user is member of the board

	boardMember := BoardMember{}

	// TODO: handle this err
	boardId, err := strconv.Atoi(c.Params.ByName("boardId"))

	// TODO: Check if there's a result from this if the query fails
	env.db.First(&boardMember, "board_id = ? AND user_id = ?", boardId, user.ID)

	if boardMember.BoardID == 0 {
		err = templates.ExecuteTemplate(c.Writer, "notAuthorized.html", nil)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// Get user id from form post
	// TODO: handle this err
	usernameToRemove := c.PostForm("userToRemove")
	

	userToRemove := User{}
	env.db.First(&userToRemove, "email = ?", usernameToRemove)

	boardMember = BoardMember{}
	env.db.First(&boardMember, "board_id = ? AND user_id = ?", boardId, userToRemove.ID)

	log.Printf("Board member is %d, and %d", boardMember.BoardID, boardMember.UserID)
	if boardMember.BoardID != 0 {
		// TODO: Check for errors
		env.db.Delete(&boardMember)
		log.Printf("Removed user %d from board %d", userToRemove.ID, boardId)
		c.Redirect(http.StatusFound, fmt.Sprintf("/board/%d/members", boardId))
		return
	} else {
		log.Printf("User %d was not a member of of board %d", userToRemove.ID, boardId)
		c.Redirect(http.StatusFound, fmt.Sprintf("/board/%d/members", boardId))
		return
	}
}

func main() {
	// TODO: format check in ws

	// TODO: Auth middleware rather than manual auth in each route
	flag.Parse()
	log.SetFlags(0)

	//dbHost := os.Getenv("DATABASE_HOST")
	//dbPort := os.Getenv("DATABASE_PORT")
	//dbUsername := os.Getenv("DATABASE_USERNAME")
	//dbPassword := os.Getenv("DATABASE_PASSWORD")
	
	//dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=whiteboard port=%s sslmode=disable", dbHost, dbUsername, dbPassword, dbPort)
	var err error
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

	// TODO: Use addr
	r := gin.Default()
	env := &Env{db: db, sessions: make(map[uuid.UUID]string)}
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
	r.GET("/wss", func(context *gin.Context) {
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
