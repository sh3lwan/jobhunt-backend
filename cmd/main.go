package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/sh3lwan/jobhunter/internal/server"
	"github.com/sh3lwan/jobhunter/pkg/utils"
)

func main() {
	logger := log.New(os.Stdout, "server: ", log.LstdFlags)

	config, err := loadEnvironmentVariables()

	if err != nil {
		logger.Fatalf("Failed to validate config: %v", err)
	}

	srv, err := server.NewServer(config, logger)

	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Shutdown signal received")
		cancel()
	}()

	if err := srv.Start(ctx); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}
}

func loadEnvironmentVariables() (*server.Config, error) {
	err := godotenv.Load()

	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf(":%s", utils.GetEnvOrDefault("APP_PORT", "8080"))

	config := &server.Config{
		Addr:        addr,
		DatabaseURL: utils.GetEnvOrDefault("DATABASE_URL", ""),
		KafkaBroker: utils.GetEnvOrDefault("KAFKA_BROKER", ""),
		CVTopic:     utils.GetEnvOrDefault("KAFKA_CV_TOPIC", ""),
		ResultTopic: utils.GetEnvOrDefault("KAFKA_RESULT_TOPIC", ""),
		JwtSecret:  utils.GetEnvOrDefault("JWT_SECRET", ""),
	}

	err = validateConfig(config)

	return config, err
}

// validateConfig validates the server configuration
func validateConfig(config *server.Config) error {
	if config.Addr == "" {
		return fmt.Errorf("address is required")
	}
	if config.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	if config.KafkaBroker == "" {
		return fmt.Errorf("kafka broker is required")
	}
	if config.CVTopic == "" {
		return fmt.Errorf("CV topic is required")
	}
	if config.ResultTopic == "" {
		return fmt.Errorf("result topic is required")
	}
	return nil
}
