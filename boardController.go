package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)


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
	env.db.First(&userToAdd, "username = ?", usernameToAdd)
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
	env.db.First(&userToRemove, "username = ?", usernameToRemove)

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
	rows, err := env.db.Table("users").Select("username").Joins("JOIN board_members on users.id = board_members.user_id").Where("board_members.board_id = ?", board.ID).Rows()
	usernames := []string{}
	for rows.Next() {
		// Want usernames here
		username := ""
		rows.Scan(&username)
		usernames = append(usernames, username)
	}
	templateVars := map[string]interface{}{"usernames": usernames, "board_id": board.ID}
	err = templates.ExecuteTemplate(c.Writer, "boardMembers.html", templateVars)

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

