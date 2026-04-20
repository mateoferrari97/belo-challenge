package transaction

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mateoferrari97/belo-challenge/internal/user"
)

type repository interface {
	CreateTransaction(ctx context.Context, in SaveInput) (SaveOutput, error)
	GetUserTransactions(ctx context.Context, in ListInput) (ListOutput, error)
	ApproveTransaction(ctx context.Context, in ApproveInput) (ApproveOutput, error)
	RejectTransaction(ctx context.Context, in RejectInput) (RejectOutput, error)
}

type users interface {
	Get(ctx context.Context, in user.GetInput) (user.GetOutput, error)
}

type Service struct {
	repository      repository
	users           users
	reviewThreshold decimal.Decimal
}

func NewService(repository repository, users users, reviewThreshold decimal.Decimal) *Service {
	return &Service{repository: repository, users: users, reviewThreshold: reviewThreshold}
}

func (s *Service) CreateTransaction(ctx context.Context, in CreateInput) (CreateOutput, error) {
	if err := in.Validate(); err != nil {
		return CreateOutput{}, err
	}

	status := StatusApproved
	if in.Amount.GreaterThan(s.reviewThreshold) {
		status = StatusPending
	}

	id, err := uuid.NewV7()
	if err != nil {
		return CreateOutput{}, fmt.Errorf("generate transaction id: %w", err)
	}

	transaction := Transaction{
		ID:            id,
		SourceID:      in.SourceID,
		DestinationID: in.DestinationID,
		Amount:        in.Amount,
		Status:        status,
	}

	out, err := s.repository.CreateTransaction(ctx, SaveInput{Transaction: transaction})
	if err != nil {
		return CreateOutput{}, err
	}

	return CreateOutput{Transaction: out.Transaction}, nil //nolint:staticcheck // explicit mapping keeps service/repository output types decoupled despite today's identical shape
}

func (s *Service) GetUserTransactions(ctx context.Context, in ListInput) (ListOutput, error) {
	if err := in.Validate(); err != nil {
		return ListOutput{}, err
	}

	if _, err := s.users.Get(ctx, user.GetInput{ID: in.UserID}); err != nil {
		return ListOutput{}, err
	}

	return s.repository.GetUserTransactions(ctx, in)
}

func (s *Service) ApproveTransaction(ctx context.Context, in ApproveInput) (ApproveOutput, error) {
	if err := in.Validate(); err != nil {
		return ApproveOutput{}, err
	}

	return s.repository.ApproveTransaction(ctx, in)
}

func (s *Service) RejectTransaction(ctx context.Context, in RejectInput) (RejectOutput, error) {
	if err := in.Validate(); err != nil {
		return RejectOutput{}, err
	}

	return s.repository.RejectTransaction(ctx, in)
}
