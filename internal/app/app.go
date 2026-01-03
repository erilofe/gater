package app

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/pkg/server"
)

// Run initializes and starts the Gater application.
func Run() {
	log.Println("Running Gater...")

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8081"
	}

	postServiceURL := os.Getenv("POST_SERVICE_URL")
	if postServiceURL == "" {
		postServiceURL = "http://localhost:8082"
	}

	router := gin.Default()

	// Set up a catch-all route to handle all incoming requests
	// and forward them to the Proxy function
	router.Any("/users/*path", server.Proxy(userServiceURL))
	router.Any("/posts/*path", server.Proxy(postServiceURL))

	// Start the server on port 8080
	router.Run(":8080")
}
