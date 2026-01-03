package app

import (
	"log"

	"github.com/pietroagazzi/gater/pkg/utils"
)

func Run() {
	log.Println("Running Gater...")

	message := utils.Greet("Utente")
	log.Println(message)
}
