package transaction

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Status string

const (
	StatusApproved Status = "approved"
	StatusPending  Status = "pending"
	StatusRejected Status = "rejected"
)

type Direction string

const (
	DirectionDebit  Direction = "debit"
	DirectionCredit Direction = "credit"
)

var (
	ErrValidation          = errors.New("validation error")
	ErrSourceNotFound      = errors.New("source user not found")
	ErrDestinationNotFound = errors.New("destination user not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrNotFound            = errors.New("transaction not found")
	ErrNotPending          = errors.New("transaction is not pending")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}

func newValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}

type Transaction struct {
	ID            uuid.UUID
	SourceID      uuid.UUID
	DestinationID uuid.UUID
	Amount        decimal.Decimal
	Status        Status
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateInput struct {
	SourceID      uuid.UUID
	DestinationID uuid.UUID
	Amount        decimal.Decimal
}

type CreateOutput struct {
	Transaction Transaction
}

type SaveInput struct {
	Transaction Transaction
}

type SaveOutput struct {
	Transaction Transaction
}

func (in CreateInput) Validate() error {
	if in.SourceID == uuid.Nil {
		return newValidationError("source is required")
	}

	if in.DestinationID == uuid.Nil {
		return newValidationError("destination is required")
	}

	if in.SourceID == in.DestinationID {
		return newValidationError("source and destination must be different")
	}

	if in.Amount.LessThanOrEqual(decimal.Zero) {
		return newValidationError("amount must be positive")
	}

	return nil
}

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

type ListInput struct {
	UserID uuid.UUID
	After  *Cursor
}

type ListOutput struct {
	Transactions []Transaction
	NextCursor   *Cursor
}

func (in ListInput) Validate() error {
	if in.UserID == uuid.Nil {
		return newValidationError("user_id is required")
	}

	return nil
}

type ApproveInput struct {
	ID uuid.UUID
}

type ApproveOutput struct {
	Transaction Transaction
}

type RejectInput struct {
	ID uuid.UUID
}

type RejectOutput struct {
	Transaction Transaction
}

func (in ApproveInput) Validate() error {
	if in.ID == uuid.Nil {
		return newValidationError("id is required")
	}

	return nil
}

func (in RejectInput) Validate() error {
	if in.ID == uuid.Nil {
		return newValidationError("id is required")
	}

	return nil
}
