package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sh3lwan/jobhunter/internal/handlers"
	"github.com/sh3lwan/jobhunter/internal/middleware"
	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"github.com/sh3lwan/jobhunter/internal/router"
	"github.com/sh3lwan/jobhunter/internal/schedular"
)

type Config struct {
	Addr        string
	DatabaseURL string
	KafkaBroker string
	CVTopic     string
	ResultTopic string

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type Server struct {
	config   *Config
	server   *http.Server
	producer *mq.Producer
	consumer *mq.Consumer
	db       *pgxpool.Pool
	logger   *log.Logger
}

func NewServer(config *Config, logger *log.Logger) (*Server, error) {

	// Create a connection pool
	db, err := createDBPool(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	// Create message queue components
	producer := mq.NewProducer(config.KafkaBroker, config.CVTopic)

	//defer producer.Close()

	// Create repository
	queries := repository.New(db)

	// Create consumer
	consumer := mq.NewConsumer(queries, db, config.ResultTopic, config.KafkaBroker)

	h := handlers.NewHandler(queries, producer)

	mux := router.NewRouter(h)

	handler := middleware.CORS(mux)

	// Ensure uploads directory exists
	err = os.MkdirAll("storage/uploads", 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         config.Addr,
		Handler:      handler,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return &Server{
		config:   config,
		server:   httpServer,
		consumer: consumer,
		producer: producer,
		db:       db,
		logger:   logger,
	}, nil
}

func createDBPool(databaseURL string) (*pgxpool.Pool, error) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)

	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// Start starts the server and handles graceful shutdown
func (s *Server) Start(ctx context.Context) error {
	// Close kafka producer on shutdown
	defer s.producer.Close()

	// Start consumer in a goroutine
	go func() {
		if err := s.consumer.Consume(); err != nil {
			s.logger.Printf("Consumer error: %v", err)
		}
	}()

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		s.logger.Printf("Starting server at %s", s.config.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	queries := repository.New(s.db)

	schedular.StartSchedular(queries, ctx)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrChan:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.logger.Println("Shutting down server...")
		return s.shutdown()
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Printf("Server shutdown error: %v", err)
	}

	// Close database connection
	s.db.Close()

	// Close producer
	if err := s.producer.Close(); err != nil {
		s.logger.Printf("Producer close error: %v", err)
	}

	s.logger.Println("Server shutdown complete")

	return nil
}
