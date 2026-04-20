package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/mateoferrari97/belo-challenge/cmd/web/handler"
	_ "github.com/mateoferrari97/belo-challenge/docs/api"
	"github.com/mateoferrari97/belo-challenge/internal/platform/database"
	"github.com/mateoferrari97/belo-challenge/internal/platform/web"
	"github.com/mateoferrari97/belo-challenge/internal/transaction"
	"github.com/mateoferrari97/belo-challenge/internal/user"
)

const defaultReviewThreshold = "50000"

// @title       Belo Challenge
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := openDatabase(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	reviewThreshold, err := loadReviewThreshold()
	if err != nil {
		return err
	}

	userRepository := user.NewRepository(db)
	userService := user.NewService(userRepository)

	transactionRepository := transaction.NewRepository(db)
	transactionService := transaction.NewService(transactionRepository, userService, reviewThreshold)
	transactionHandler := handler.NewTransaction(transactionService)

	server := web.NewServer()
	server.Handle(http.MethodPost, "/v1/transactions", transactionHandler.CreateTransaction())
	server.Handle(http.MethodGet, "/v1/transactions", transactionHandler.GetUserTransactions())
	server.Handle(http.MethodPatch, "/v1/transactions/{id}/approve", transactionHandler.ApproveTransaction())
	server.Handle(http.MethodPatch, "/v1/transactions/{id}/reject", transactionHandler.RejectTransaction())
	server.Handle(http.MethodGet, "/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	return server.Run(ctx)
}

func openDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL not configured")
	}

	db, err := database.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return db, nil
}

func loadReviewThreshold() (decimal.Decimal, error) {
	raw := os.Getenv("REVIEW_THRESHOLD")
	if raw == "" {
		raw = defaultReviewThreshold
	}

	threshold, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse REVIEW_THRESHOLD: %w", err)
	}

	return threshold, nil
}
