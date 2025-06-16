package server

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sh3lwan/jobhunter/internal/handlers"
	"github.com/sh3lwan/jobhunter/internal/middleware"
	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"log"
	"net/http"
	"os"
)

type Server struct {
	Addr     string
	Kafka    *mq.Producer
	Consumer *mq.Consumer
	DB       *pgxpool.Pool
	Handler  *handlers.Handler
	Mux      *http.ServeMux
}

func NewServer(addr string) *Server {
	mux := http.NewServeMux()

	cvTopic := os.Getenv("KAFKA_CV_TOPIC")
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	p := mq.NewProducer(kafkaBroker, cvTopic)

	// Create a connection pool
	dsn := os.Getenv("DATABASE_URL")
	dbpool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}

	queries := repository.New(dbpool)

	c := mq.NewConsumer(queries)

	handler := handlers.NewHandler(queries, p)
	//handler = middleware.CORS(handler)

	s := &Server{
		addr,
		p,
		c,
		dbpool,
		handler,
		mux,
	}

	s.routes()

	return s
}

func (s *Server) Start() {
	fmt.Printf("Starting server at %s\n", s.Addr)

	go s.Consumer.Consume()

	defer s.DB.Close()

	cors := middleware.CORS(s.Mux)

	if err := http.ListenAndServe(s.Addr, cors); err != nil {
		log.Fatal(err.Error())
	}
}

func (s *Server) routes() {

	s.Mux.HandleFunc("GET /", s.Handler.HealthCheck)

	s.Mux.HandleFunc("POST /api/v1/upload", s.Handler.UploadCV)

	s.Mux.HandleFunc("GET /api/v1/cvs", s.Handler.ListCVs)

	s.Mux.HandleFunc("GET /api/v1/fetch", s.Handler.FetchJobs)

	s.Mux.HandleFunc("GET /api/v1/stream", s.Handler.StreamCVStatus)
}
