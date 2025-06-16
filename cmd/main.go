package main

import (
	"github.com/joho/godotenv"
	"github.com/sh3lwan/jobhunter/internal/server"
	"log"
)

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatalf("err loading: %v", err)
	}

	svr := server.NewServer(":8080")

	svr.Start()

	defer svr.DB.Close()
}
