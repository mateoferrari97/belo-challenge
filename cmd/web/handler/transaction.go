package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/mateoferrari97/belo-challenge/internal/platform/web"
	"github.com/mateoferrari97/belo-challenge/internal/transaction"
	"github.com/mateoferrari97/belo-challenge/internal/user"
)

type service interface {
	CreateTransaction(ctx context.Context, in transaction.CreateInput) (transaction.CreateOutput, error)
	GetUserTransactions(ctx context.Context, in transaction.ListInput) (transaction.ListOutput, error)
	ApproveTransaction(ctx context.Context, in transaction.ApproveInput) (transaction.ApproveOutput, error)
	RejectTransaction(ctx context.Context, in transaction.RejectInput) (transaction.RejectOutput, error)
}

type Transaction struct {
	service service
}

func NewTransaction(service service) *Transaction {
	return &Transaction{service: service}
}

// @Summary     Create a transaction
// @Description Create a new transfer between two users. Amounts above 50,000 return with status `pending` and require PATCH approval. Smaller amounts return `approved` with balances already updated.
// @Tags        transactions
// @Accept      json
// @Produce     json
// @Param       body  body      createRequestBody        true  "Transaction to create"
// @Success     201   {object}  createResponseBody
// @Failure     400   {object}  errCreateBadRequest
// @Failure     404   {object}  errCreateNotFound
// @Failure     409   {object}  errInsufficientBalance
// @Failure     500   {object}  errInternalServer
// @Router      /transactions [post]
func (t *Transaction) CreateTransaction() web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		var requestBody createRequestBody
		if err := web.Decode(r, &requestBody); err != nil {
			return web.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		out, err := t.service.CreateTransaction(r.Context(), transaction.CreateInput{
			SourceID:      requestBody.SourceID,
			DestinationID: requestBody.DestinationID,
			Amount:        requestBody.Amount,
		})
		if err != nil {
			return mapServiceError(err)
		}

		return web.Respond(w, toCreateResponseBody(out.Transaction), http.StatusCreated)
	}
}

// @Summary     List transactions for a user
// @Description Returns transactions where the given user is source or destination, newest first. Pagination via opaque cursor from a prior response.
// @Tags        transactions
// @Produce     json
// @Param       userId  query     string                true   "User ID (UUID)"                               example(550e8400-e29b-41d4-a716-446655440000)
// @Param       cursor  query     string                false  "Opaque pagination cursor from a prior response"
// @Success     200     {object}  listResponseBody
// @Failure     400     {object}  errListBadRequest
// @Failure     404     {object}  errUserNotFound
// @Failure     500     {object}  errInternalServer
// @Router      /transactions [get]
func (t *Transaction) GetUserTransactions() web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		query := r.URL.Query()

		userID, err := uuid.Parse(query.Get("userId"))
		if err != nil {
			return web.NewHTTPError(http.StatusBadRequest, "invalid user_id")
		}

		var after *transaction.Cursor
		if rawCursor := query.Get("cursor"); rawCursor != "" {
			var cursor transaction.Cursor
			cursor, err = decodeCursor(rawCursor)
			if err != nil {
				return web.NewHTTPError(http.StatusBadRequest, "invalid cursor")
			}
			after = &cursor
		}

		out, err := t.service.GetUserTransactions(r.Context(), transaction.ListInput{
			UserID: userID,
			After:  after,
		})
		if err != nil {
			return mapServiceError(err)
		}

		responseBody, err := toListResponseBody(out)
		if err != nil {
			return fmt.Errorf("build list response: %w", err)
		}

		return web.Respond(w, responseBody, http.StatusOK)
	}
}

// @Summary     Approve a pending transaction
// @Description Move a `pending` transaction to `approved`, debit source, credit destination. Fails if the transaction is not pending.
// @Tags        transactions
// @Produce     json
// @Param       id   path      string                    true  "Transaction ID (UUID)"   example(550e8400-e29b-41d4-a716-446655440002)
// @Success     200  {object}  createResponseBody
// @Failure     400  {object}  errTransitionBadRequest
// @Failure     404  {object}  errTransactionNotFound
// @Failure     409  {object}  errTransactionNotPending
// @Failure     500  {object}  errInternalServer
// @Router      /transactions/{id}/approve [patch]
func (t *Transaction) ApproveTransaction() web.Handler {
	return t.transitionByID(func(ctx context.Context, id uuid.UUID) (transaction.Transaction, error) {
		out, err := t.service.ApproveTransaction(ctx, transaction.ApproveInput{ID: id})
		return out.Transaction, err
	})
}

// @Summary     Reject a pending transaction
// @Description Move a `pending` transaction to `rejected`. Terminal state; no further transitions. Balances are not touched.
// @Tags        transactions
// @Produce     json
// @Param       id   path      string                    true  "Transaction ID (UUID)"   example(550e8400-e29b-41d4-a716-446655440002)
// @Success     200  {object}  createResponseBody
// @Failure     400  {object}  errTransitionBadRequest
// @Failure     404  {object}  errTransactionNotFound
// @Failure     409  {object}  errTransactionNotPending
// @Failure     500  {object}  errInternalServer
// @Router      /transactions/{id}/reject [patch]
func (t *Transaction) RejectTransaction() web.Handler {
	return t.transitionByID(func(ctx context.Context, id uuid.UUID) (transaction.Transaction, error) {
		out, err := t.service.RejectTransaction(ctx, transaction.RejectInput{ID: id})
		return out.Transaction, err
	})
}

func (t *Transaction) transitionByID(
	action func(ctx context.Context, id uuid.UUID) (transaction.Transaction, error),
) web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := uuid.Parse(web.Param(r, "id"))
		if err != nil {
			return web.NewHTTPError(http.StatusBadRequest, "invalid transaction id")
		}

		updated, err := action(r.Context(), id)
		if err != nil {
			return mapServiceError(err)
		}

		return web.Respond(w, toCreateResponseBody(updated), http.StatusOK)
	}
}

func mapServiceError(err error) error {
	var validationErr *transaction.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return web.NewHTTPError(http.StatusBadRequest, validationErr.Message)
	case errors.Is(err, user.ErrNotFound):
		return web.NewHTTPError(http.StatusNotFound, "user not found")
	case errors.Is(err, transaction.ErrNotFound):
		return web.NewHTTPError(http.StatusNotFound, "transaction not found")
	case errors.Is(err, transaction.ErrNotPending):
		return web.NewHTTPError(http.StatusConflict, "transaction is not pending")
	case errors.Is(err, transaction.ErrSourceNotFound):
		return web.NewHTTPError(http.StatusNotFound, "source user not found")
	case errors.Is(err, transaction.ErrDestinationNotFound):
		return web.NewHTTPError(http.StatusNotFound, "destination user not found")
	case errors.Is(err, transaction.ErrInsufficientBalance):
		return web.NewHTTPError(http.StatusConflict, "insufficient balance")
	default:
		log.Printf("transaction handler: %v", err)
		return web.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
