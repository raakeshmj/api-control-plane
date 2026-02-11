package main

import (
	"log"
	"os"

	"github.com/raakeshmj/apigatewayplane/internal/config"
	"github.com/raakeshmj/apigatewayplane/internal/server"
)

func main() {
	cfg := config.Load()

	srv := server.New(cfg)

	if err := srv.Start(); err != nil {
		log.Printf("Server failed to start: %v", err)
		os.Exit(1)
	}
}
