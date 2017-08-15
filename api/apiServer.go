package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	jwt_lib "github.com/dgrijalva/jwt-go"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"

	"golang.org/x/crypto/bcrypt"

	"bargain/liquefy/db"
	lq "bargain/liquefy/models"
)

type ApiServer interface {
	Start()
}

type apiServer struct {}

func NewApiServer() ApiServer {
	return apiServer{}
}

func (server apiServer) Start() {
	router := gin.Default()
	webserver := router.Group("/private/")

	//TODO: Custom Middlewear to check Webserver Secret
	//	secretWebserver := "putsecrethere"
	//  secretJWT := "mySUPERPASSWrod"

	//webserver.Use(func() gin.HandlerFunc {
	//return func(c *gin.Context) {
	//	if ah := req.Header.Get("Authorization"); ah != secretWebserver {
	//		c.AbortWithError(401, err)
	//  }
	// }})

	webserver.POST("/user", func(c *gin.Context) {
		user := &lq.User{}
		if c.BindJSON(&user) == nil {
			createdUser, err := generateUser(user)
			if err != nil {
				c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
			}
			tokenString, err := generateToken(createdUser.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
			}
			c.JSON(http.StatusCreated, tokenString)
		}
	})

	webserver.POST("/user/gentoken", func(c *gin.Context) {
		//TODO: Check if the user token is valid @Sach can try this maybe
		userid := c.Param("userid")

		intuserid, err := strconv.Atoi(userid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		tokenString, err := generateToken(uint(intuserid))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		}
		c.JSON(http.StatusCreated, gin.H{"token": tokenString})
	})

	/*
		Set this header in your request to get here.
		Authorization: Bearer `token`
	*/

	private := router.Group("/api/")
	private.Use(TokenValidator("mySUPERPASSWrod"))

	// PRIVATE WEBSITE API //
	private.GET("/user", GetUser)
	private.POST("/linkAwsAccount", LinkAwsAccount)
	private.POST("/setupAwsAccount", SetupAwsAccount)

	// THIS STUFF BELOW IS PUBLIC SWAGGER API //

	// Jobs Information
	private.POST("/job", CreateJob)
	private.GET("/jobs", ListJobs)
	private.GET("/job/:jobid", GetJob)
	private.DELETE("/job/:jobid", DeleteJob)

	// Instance Information
	private.GET("/instances", ListInstances)
	private.GET("/instance/:instanceid", GetInstance)
	private.DELETE("/instance/:instanceid", DeleteInstance)

	private.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"error": "Welcome to Liquefy"})
	})

	router.StaticFile("/api_spec", "./swagger/swagger.json")
	router.Static("/apidoc", "./docs")

	// ROUTER
	router.Run(":3030")
}

func TokenValidator(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {

		//Strip any "" that might be in the auth section
		c.Request.Header.Set("Authorization", strings.Replace(c.Request.Header.Get("Authorization"), "\"", "", -1))

		token, err := jwt_lib.ParseFromRequest(c.Request, func(token *jwt_lib.Token) (interface{}, error) {
			b := ([]byte(secret))
			return b, nil
		})

		if err != nil {
			log.Errorf("Error: Parsing the bearer jwt token : %s", err)
			c.AbortWithError(401, errors.New("Incorrect token provided"))
		}

		if uid, ok := token.Claims["ID"].(float64); ok {

			user, err := db.Users().Get(uint(uid))
			if err != nil {
				log.Errorf("Error Validating User Token : %s", err)
				c.AbortWithError(401, err)
			}

			if token.Raw == user.ApiKey {
				c.Keys = make(map[string]interface{})
				c.Keys["user"] = user
				c.Keys["userid"] = user.ID
			} else {
				c.AbortWithError(401, errors.New("Invalid User Auth Token"))
			}
		}
	}
}

// User Auth Related functions //
func generateUser(user *lq.User) (*lq.User, error) {
	// Check if user exists
	existingUser, _ := db.Users().GetByEmail(user.Email)

	if existingUser.ID > 0 {
		return nil, errors.New("User with email already present")
	}

	// Hash the user password
	passBytes := []byte(user.Password)
	hashedPassword, err := bcrypt.GenerateFromPassword(passBytes, 10)
	if err != nil {
		return nil, errors.New("Invalid user password")
	}

	log.Info("Creating user")

	user.Password = string(hashedPassword[:])
	user.PublicID = uuid.NewRandom().String()
	err = db.Users().Create(user)

	if err != nil {
		return nil, errors.New("User with email already present")
	}
	return user, nil
}

func generateToken(userid uint) (string, error) {
	// Create the token
	token := jwt_lib.New(jwt_lib.GetSigningMethod("HS256"))

	// Set some claims
	token.Claims["ID"] = userid

	//TODO: Set token Expirations @Sach can try this maybe
	token.Claims["exp"] = time.Now().Add(time.Hour * 10000).Unix()

	//TODO : Make JWT A env var @Sach can try this maybe
	// Sign and get the complete encoded token as a string
	tokenString, err := token.SignedString([]byte("mySUPERPASSWrod"))
	if err != nil {
		return "", err
	}

	//Update the new user uifo
	err = db.Users().Update(userid,"api_key", tokenString)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
