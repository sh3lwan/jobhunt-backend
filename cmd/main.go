package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

	// Dependencies (Postgres, Kafka) may not be up yet — keep retrying
	// instead of dying, so the API comes up by itself once they return.
	srv, err := server.NewServer(config, logger)

	for err != nil {
		logger.Printf("Failed to create server (dependency down?): %v — retrying in 10s", err)
		time.Sleep(10 * time.Second)
		srv, err = server.NewServer(config, logger)
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
		JwtSecret:   utils.GetEnvOrDefault("JWT_SECRET", ""),

		ScraperTopic:    utils.GetEnvOrDefault("KAFKA_SCRAPER_TOPIC", "job-scraping-requests"),
		ScrapePlatforms: splitCSV(utils.GetEnvOrDefault("SCRAPE_PLATFORMS", "greenhouse,ashby,lever,remotive")),
		ParserURL:       utils.GetEnvOrDefault("PARSER_URL", "http://localhost:5001"),

		CandidateConstraints: utils.GetEnvOrDefault("CANDIDATE_CONSTRAINTS", ""),

		GoogleClientID:     utils.GetEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: utils.GetEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  utils.GetEnvOrDefault("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		FrontendURL:        utils.GetEnvOrDefault("FRONTEND_URL", "http://localhost:3000"),
	}

	err = validateConfig(config)

	return config, err
}

func splitCSV(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
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
