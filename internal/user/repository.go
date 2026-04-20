package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const selectUserByID = `SELECT id, name, email, balance, created_at, updated_at FROM users WHERE id = $1`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Get(ctx context.Context, in GetInput) (GetOutput, error) {
	var user User
	err := r.pool.QueryRow(ctx, selectUserByID, in.ID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Balance, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GetOutput{}, ErrNotFound
		}
		return GetOutput{}, fmt.Errorf("query user: %w", err)
	}

	return GetOutput{User: user}, nil
}
