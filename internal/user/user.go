package user

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Balance   decimal.Decimal
	CreatedAt time.Time
	UpdatedAt time.Time
}

type GetInput struct {
	ID uuid.UUID
}

type GetOutput struct {
	User User
}
