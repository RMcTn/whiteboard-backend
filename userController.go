package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (env *Env) CreateUser(c *gin.Context) {
	validationErrors := []string{}
	username := c.PostForm("username")

	if username == "" {
		validationErrors = append(validationErrors, "Username cannot be empty")
	}

	password := c.PostForm("password")

	if password == "" {
		validationErrors = append(validationErrors, "Password cannot be empty")
	}

	minPasswordLength := 8
	if len(password) < minPasswordLength {
		validationErrors = append(validationErrors, fmt.Sprintf("Password must be at least %d characters long", minPasswordLength))
	}
	
	templateVars := map[string]interface{}{"username": username}
	if len(validationErrors) > 0 {
		templateVars["errors"] = validationErrors
		c.HTML(http.StatusUnprocessableEntity, "newUser.html", templateVars)
		return
	}

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
	user = User{Username: username, PasswordHash: string(passwordHash)}
	env.db.Create(&user)

	if user.ID == 0 {
		validationErrors = append(validationErrors, "Username is already taken")
		templateVars["errors"] = validationErrors
		c.HTML(http.StatusUnprocessableEntity, "newUser.html", templateVars)
		return
	}
	log.Printf("New user created: %s", user.Username)

	sessionId = uuid.New()
	env.sessions[sessionId] = username
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
	validationErrors := []string{}
	username := c.PostForm("username")

	if username == "" {
		validationErrors = append(validationErrors, "Username cannot be empty")
	}

	password := c.PostForm("password")

	if password == "" {
		validationErrors = append(validationErrors, "Password cannot be empty")
	}

	minPasswordLength := 8
	if len(password) < minPasswordLength {
		validationErrors = append(validationErrors, fmt.Sprintf("Password must be at least %d characters long", minPasswordLength))
	}
	
	templateVars := map[string]interface{}{"username": username}
	if len(validationErrors) > 0 {
		templateVars["errors"] = validationErrors
		c.HTML(http.StatusUnprocessableEntity, "signIn.html", templateVars)
		return
	}

	user := User{}
	env.db.Where("username = ?", username).First(&user)
	log.Println(user)

	// TODO: Handle err
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		validationErrors = append(validationErrors, "Incorrect details")
		templateVars["errors"] = validationErrors
		c.HTML(http.StatusUnprocessableEntity, "signIn.html", templateVars)
		return
	}
	log.Println("Success logging in")

	sessionId := uuid.New()
	env.sessions[sessionId] = username


	c.SetCookie("sessionId", sessionId.String(), 0, "/", c.Request.Host, false, false)
	c.Redirect(http.StatusFound, fmt.Sprintf("/user/%d", user.ID))
}

func (env *Env) GetUser(c *gin.Context) {
	sessionId, err := getSessionIdFromCookie(c)
	user, err := env.getUserFromSession(sessionId)
	if err != nil {
		err = templates.ExecuteTemplate(c.Writer, "signIn.html", nil)

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
	log.Printf("User %s id %d\n", user.Username, user.ID)
	log.Printf("=========================")

	err = templates.ExecuteTemplate(c.Writer, "user.html", user)

	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
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
