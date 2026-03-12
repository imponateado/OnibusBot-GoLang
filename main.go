package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Carregar .env se existir
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN não configurado no ambiente ou .env")
	}

	version := "1.0.8-go"
	
	service, err := NewBotService(token, version)
	if err != nil {
		log.Fatalf("Erro ao criar serviço do bot: %v", err)
	}

	service.Start()
}
