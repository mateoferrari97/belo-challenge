package transaction

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

const (
	selectTransactionForUpdate       = `SELECT source_id, destination_id, amount, status, created_at, updated_at FROM transactions WHERE id = $1 FOR UPDATE`
	selectTransactionStatusForUpdate = `SELECT status FROM transactions WHERE id = $1 FOR UPDATE`
	insertTransaction                = `INSERT INTO transactions (id, source_id, destination_id, amount, status) VALUES ($1, $2, $3, $4, $5) RETURNING created_at, updated_at`
	updateTransactionStatus          = `UPDATE transactions SET status = $1, updated_at = NOW() WHERE id = $2 RETURNING source_id, destination_id, amount, status, created_at, updated_at`

	selectUserBalanceForUpdate = `SELECT balance FROM users WHERE id = $1 FOR UPDATE`
	selectUserExists           = `SELECT 1 FROM users WHERE id = $1`
	updateUserBalance          = `UPDATE users SET balance = $1, updated_at = NOW() WHERE id = $2`

	insertTransactionLog = `INSERT INTO transactions_log (id, transaction_id, user_id, direction, amount, balance_before, balance_after) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	selectTransactionsByUser      = `SELECT id, source_id, destination_id, amount, status, created_at, updated_at FROM transactions WHERE source_id = $1 OR destination_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`
	selectTransactionsByUserAfter = `SELECT id, source_id, destination_id, amount, status, created_at, updated_at FROM transactions WHERE (source_id = $1 OR destination_id = $1) AND (created_at, id) < ($2, $3) ORDER BY created_at DESC, id DESC LIMIT $4`
)

const maxTransactionsPerPage = 20

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateTransaction(ctx context.Context, in SaveInput) (SaveOutput, error) {
	transaction, err := runInTx(ctx, r.pool, func(tx pgx.Tx) (Transaction, error) {
		return save(ctx, tx, in.Transaction)
	})

	return SaveOutput{Transaction: transaction}, err
}

func (r *Repository) GetUserTransactions(ctx context.Context, in ListInput) (ListOutput, error) {
	rows, err := r.getRows(ctx, in)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list transactions: %w", err)
	}

	defer rows.Close()

	transactions, err := scanTransactions(rows)
	if err != nil {
		return ListOutput{}, err
	}

	return paginate(transactions), nil
}

func (r *Repository) ApproveTransaction(ctx context.Context, in ApproveInput) (ApproveOutput, error) {
	transaction, err := runInTx(ctx, r.pool, func(tx pgx.Tx) (Transaction, error) {
		return approve(ctx, tx, in.ID)
	})

	return ApproveOutput{Transaction: transaction}, err
}

func (r *Repository) RejectTransaction(ctx context.Context, in RejectInput) (RejectOutput, error) {
	transaction, err := runInTx(ctx, r.pool, func(tx pgx.Tx) (Transaction, error) {
		return reject(ctx, tx, in.ID)
	})

	return RejectOutput{Transaction: transaction}, err
}

func runInTx[T any](ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	result, err := fn(tx)
	if err != nil {
		return zero, err
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}

func save(ctx context.Context, tx pgx.Tx, t Transaction) (Transaction, error) {
	if t.Status == StatusApproved {
		return saveApproved(ctx, tx, t)
	}

	return savePending(ctx, tx, t)
}

func saveApproved(ctx context.Context, tx pgx.Tx, t Transaction) (Transaction, error) {
	sourceBalance, destinationBalance, err := checkFunds(ctx, tx, t)
	if err != nil {
		return Transaction{}, err
	}

	if err = moveFunds(ctx, tx, t.SourceID, t.DestinationID, sourceBalance, destinationBalance, t.Amount); err != nil {
		return Transaction{}, err
	}

	saved, err := persistTransaction(ctx, tx, t)
	if err != nil {
		return Transaction{}, err
	}

	return saved, recordMovements(ctx, tx, saved, sourceBalance, destinationBalance)
}

func savePending(ctx context.Context, tx pgx.Tx, t Transaction) (Transaction, error) {
	if err := ensureUserExists(ctx, tx, t.SourceID, ErrSourceNotFound); err != nil {
		return Transaction{}, err
	}

	if err := ensureUserExists(ctx, tx, t.DestinationID, ErrDestinationNotFound); err != nil {
		return Transaction{}, err
	}

	return persistTransaction(ctx, tx, t)
}

func approve(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error) {
	t, err := loadPending(ctx, tx, id)
	if err != nil {
		return Transaction{}, err
	}

	sourceBalance, destinationBalance, err := checkFunds(ctx, tx, t)
	if err != nil {
		return Transaction{}, err
	}

	if err = moveFunds(ctx, tx, t.SourceID, t.DestinationID, sourceBalance, destinationBalance, t.Amount); err != nil {
		return Transaction{}, err
	}

	updated, err := setTransactionStatus(ctx, tx, id, StatusApproved)
	if err != nil {
		return Transaction{}, err
	}

	return updated, recordMovements(ctx, tx, updated, sourceBalance, destinationBalance)
}

func reject(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error) {
	if err := ensurePending(ctx, tx, id); err != nil {
		return Transaction{}, err
	}

	return setTransactionStatus(ctx, tx, id, StatusRejected)
}

func loadPending(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error) {
	t, err := lockTransaction(ctx, tx, id)
	if err != nil {
		return Transaction{}, err
	}

	if t.Status != StatusPending {
		return Transaction{}, ErrNotPending
	}

	return t, nil
}

func ensurePending(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	status, err := lockTransactionStatus(ctx, tx, id)
	if err != nil {
		return err
	}

	if status != StatusPending {
		return ErrNotPending
	}

	return nil
}

func checkFunds(ctx context.Context, tx pgx.Tx, t Transaction) (decimal.Decimal, decimal.Decimal, error) {
	sourceBalance, destinationBalance, err := lockUserBalances(ctx, tx, t.SourceID, t.DestinationID)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	if sourceBalance.LessThan(t.Amount) {
		return decimal.Zero, decimal.Zero, ErrInsufficientBalance
	}

	return sourceBalance, destinationBalance, nil
}

func lockTransaction(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error) {
	t := Transaction{ID: id}
	err := tx.QueryRow(ctx, selectTransactionForUpdate, id).Scan(
		&t.SourceID, &t.DestinationID, &t.Amount, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, ErrNotFound
		}
		return Transaction{}, fmt.Errorf("lock transaction: %w", err)
	}

	return t, nil
}

func lockTransactionStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Status, error) {
	var status Status
	err := tx.QueryRow(ctx, selectTransactionStatusForUpdate, id).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("lock transaction: %w", err)
	}

	return status, nil
}

func persistTransaction(ctx context.Context, tx pgx.Tx, t Transaction) (Transaction, error) {
	err := tx.QueryRow(ctx, insertTransaction,
		t.ID, t.SourceID, t.DestinationID, t.Amount, string(t.Status),
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Transaction{}, fmt.Errorf("insert transaction: %w", err)
	}

	return t, nil
}

func setTransactionStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status Status) (Transaction, error) {
	t := Transaction{ID: id}
	err := tx.QueryRow(ctx, updateTransactionStatus, string(status), id).Scan(
		&t.SourceID, &t.DestinationID, &t.Amount, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return Transaction{}, fmt.Errorf("flip transaction status: %w", err)
	}

	return t, nil
}

func lockUserBalances(ctx context.Context, tx pgx.Tx, sourceID, destinationID uuid.UUID) (decimal.Decimal, decimal.Decimal, error) {
	firstID, secondID := orderAsc(sourceID, destinationID)
	firstIsSource := firstID == sourceID

	var firstBalance decimal.Decimal
	if err := tx.QueryRow(ctx, selectUserBalanceForUpdate, firstID).Scan(&firstBalance); err != nil {
		return missingBalanceError(err, firstIsSource)
	}

	var secondBalance decimal.Decimal
	if err := tx.QueryRow(ctx, selectUserBalanceForUpdate, secondID).Scan(&secondBalance); err != nil {
		return missingBalanceError(err, !firstIsSource)
	}

	if firstIsSource {
		return firstBalance, secondBalance, nil
	}

	return secondBalance, firstBalance, nil
}

func ensureUserExists(ctx context.Context, tx pgx.Tx, id uuid.UUID, notFound error) error {
	var exists int
	err := tx.QueryRow(ctx, selectUserExists, id).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFound
		}
		return fmt.Errorf("check user exists: %w", err)
	}

	return nil
}

func moveFunds(ctx context.Context, tx pgx.Tx, sourceID, destinationID uuid.UUID, sourceBalance, destinationBalance, amount decimal.Decimal) error {
	if _, err := tx.Exec(ctx, updateUserBalance, sourceBalance.Sub(amount), sourceID); err != nil {
		return fmt.Errorf("debit source: %w", err)
	}

	if _, err := tx.Exec(ctx, updateUserBalance, destinationBalance.Add(amount), destinationID); err != nil {
		return fmt.Errorf("credit destination: %w", err)
	}

	return nil
}

func recordMovements(ctx context.Context, tx pgx.Tx, t Transaction, sourceBalance, destinationBalance decimal.Decimal) error {
	if err := recordMovement(ctx, tx, t.ID, t.SourceID, DirectionDebit, t.Amount, sourceBalance, sourceBalance.Sub(t.Amount)); err != nil {
		return fmt.Errorf("insert debit log: %w", err)
	}

	if err := recordMovement(ctx, tx, t.ID, t.DestinationID, DirectionCredit, t.Amount, destinationBalance, destinationBalance.Add(t.Amount)); err != nil {
		return fmt.Errorf("insert credit log: %w", err)
	}

	return nil
}

func recordMovement(ctx context.Context, tx pgx.Tx, transactionID, userID uuid.UUID, direction Direction, amount, balanceBefore, balanceAfter decimal.Decimal) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate log id: %w", err)
	}

	_, err = tx.Exec(ctx, insertTransactionLog, id, transactionID, userID, string(direction), amount, balanceBefore, balanceAfter)
	return err
}

func (r *Repository) getRows(ctx context.Context, in ListInput) (pgx.Rows, error) {
	limit := maxTransactionsPerPage + 1
	if in.After == nil {
		return r.pool.Query(ctx, selectTransactionsByUser, in.UserID, limit)
	}

	return r.pool.Query(ctx, selectTransactionsByUserAfter, in.UserID, in.After.CreatedAt, in.After.ID, limit)
}

func scanTransactions(rows pgx.Rows) ([]Transaction, error) {
	transactions := make([]Transaction, 0, maxTransactionsPerPage+1)
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.SourceID, &t.DestinationID, &t.Amount, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}

		transactions = append(transactions, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return transactions, nil
}

func paginate(transactions []Transaction) ListOutput {
	if len(transactions) <= maxTransactionsPerPage {
		return ListOutput{Transactions: transactions, NextCursor: nil}
	}

	lastKept := transactions[maxTransactionsPerPage-1]
	return ListOutput{
		Transactions: transactions[:maxTransactionsPerPage],
		NextCursor:   &Cursor{CreatedAt: lastKept.CreatedAt, ID: lastKept.ID},
	}
}

func missingBalanceError(err error, missingIsSource bool) (decimal.Decimal, decimal.Decimal, error) {
	if !errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, decimal.Zero, fmt.Errorf("lock user balance: %w", err)
	}

	if missingIsSource {
		return decimal.Zero, decimal.Zero, ErrSourceNotFound
	}

	return decimal.Zero, decimal.Zero, ErrDestinationNotFound
}

func orderAsc(first, second uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(first[:], second[:]) <= 0 {
		return first, second
	}

	return second, first
}
