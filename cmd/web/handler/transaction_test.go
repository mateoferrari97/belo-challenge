package handler_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mateoferrari97/belo-challenge/cmd/web/handler"
	"github.com/mateoferrari97/belo-challenge/internal/transaction"
	"github.com/mateoferrari97/belo-challenge/internal/user"
)

func encodeCursor(t *testing.T, cursor transaction.Cursor) string {
	t.Helper()
	payload, err := json.Marshal(cursor)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCursor(t *testing.T, encoded string) *transaction.Cursor {
	t.Helper()
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	var cursor transaction.Cursor
	require.NoError(t, json.Unmarshal(payload, &cursor))
	return &cursor
}

type serviceMock struct {
	mock.Mock
}

func (s *serviceMock) CreateTransaction(ctx context.Context, in transaction.CreateInput) (transaction.CreateOutput, error) {
	args := s.Called(ctx, in)
	return args.Get(0).(transaction.CreateOutput), args.Error(1)
}

func (s *serviceMock) GetUserTransactions(ctx context.Context, in transaction.ListInput) (transaction.ListOutput, error) {
	args := s.Called(ctx, in)
	return args.Get(0).(transaction.ListOutput), args.Error(1)
}

func (s *serviceMock) ApproveTransaction(ctx context.Context, in transaction.ApproveInput) (transaction.ApproveOutput, error) {
	args := s.Called(ctx, in)
	return args.Get(0).(transaction.ApproveOutput), args.Error(1)
}

func (s *serviceMock) RejectTransaction(ctx context.Context, in transaction.RejectInput) (transaction.RejectOutput, error) {
	args := s.Called(ctx, in)
	return args.Get(0).(transaction.RejectOutput), args.Error(1)
}

func TestTransaction_CreateTransaction(t *testing.T) {
	// Given
	sourceID := uuid.New()
	destinationID := uuid.New()
	transactionID := uuid.New()
	amount := decimal.NewFromInt(100)
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	service := &serviceMock{}
	service.On("CreateTransaction", mock.Anything, transaction.CreateInput{
		SourceID:      sourceID,
		DestinationID: destinationID,
		Amount:        amount,
	}).Return(transaction.CreateOutput{
		Transaction: transaction.Transaction{
			ID:            transactionID,
			SourceID:      sourceID,
			DestinationID: destinationID,
			Amount:        amount,
			Status:        transaction.StatusApproved,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}, nil)

	body := fmt.Sprintf(`{"source_id":%q,"destination_id":%q,"amount":"100"}`, sourceID, destinationID)
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.CreateTransaction().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusCreated, rr.Code)
	require.JSONEq(t, fmt.Sprintf(
		`{"id":%q,"source_id":%q,"destination_id":%q,"amount":"100","status":"approved","created_at":%q,"updated_at":%q}`,
		transactionID, sourceID, destinationID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	), rr.Body.String())
	mock.AssertExpectationsForObjects(t, service)
}

func TestTransaction_CreateTransaction_InvalidJSON(t *testing.T) {
	// Given
	service := &serviceMock{}
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.CreateTransaction().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "CreateTransaction")
}

func TestTransaction_CreateTransaction_MalformedUUID(t *testing.T) {
	// Given
	service := &serviceMock{}
	body := `{"source_id":"not-a-uuid","destination_id":"018f3a7b-0000-7000-8000-000000000002","amount":"100"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.CreateTransaction().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "CreateTransaction")
}

func TestTransaction_CreateTransaction_PropagatesServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"validation", &transaction.ValidationError{Message: "invalid input"}, http.StatusBadRequest},
		{"source not found", transaction.ErrSourceNotFound, http.StatusNotFound},
		{"destination not found", transaction.ErrDestinationNotFound, http.StatusNotFound},
		{"insufficient balance", transaction.ErrInsufficientBalance, http.StatusConflict},
		{"generic error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			service := &serviceMock{}
			service.On("CreateTransaction", mock.Anything, mock.Anything).
				Return(transaction.CreateOutput{}, testCase.err)

			body := fmt.Sprintf(`{"source_id":%q,"destination_id":%q,"amount":"100"}`, uuid.New(), uuid.New())
			req := httptest.NewRequest(http.MethodPost, "/v1/transactions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			transactionHandler := handler.NewTransaction(service)

			// When
			transactionHandler.CreateTransaction().ServeHTTP(rr, req)

			// Then
			require.Equal(t, testCase.wantStatus, rr.Code)
			mock.AssertExpectationsForObjects(t, service)
		})
	}
}

func TestTransaction_GetUserTransactions(t *testing.T) {
	// Given
	userID := uuid.New()
	transactionID1 := uuid.New()
	transactionID2 := uuid.New()
	peer1 := uuid.New()
	peer2 := uuid.New()
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	service := &serviceMock{}
	service.On("GetUserTransactions", mock.Anything, transaction.ListInput{UserID: userID}).
		Return(transaction.ListOutput{
			Transactions: []transaction.Transaction{
				{ID: transactionID1, SourceID: userID, DestinationID: peer1, Amount: decimal.NewFromInt(10), Status: transaction.StatusApproved, CreatedAt: now, UpdatedAt: now},
				{ID: transactionID2, SourceID: peer2, DestinationID: userID, Amount: decimal.NewFromInt(20), Status: transaction.StatusApproved, CreatedAt: now, UpdatedAt: now},
			},
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/transactions?userId="+userID.String(), nil)
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.GetUserTransactions().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, fmt.Sprintf(
		`{"data":[
			{"id":%q,"source_id":%q,"destination_id":%q,"amount":"10","status":"approved","created_at":%q,"updated_at":%q},
			{"id":%q,"source_id":%q,"destination_id":%q,"amount":"20","status":"approved","created_at":%q,"updated_at":%q}
		],"next_cursor":null}`,
		transactionID1, userID, peer1, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
		transactionID2, peer2, userID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	), rr.Body.String())
	mock.AssertExpectationsForObjects(t, service)
}

func TestTransaction_GetUserTransactions_WithNextCursor(t *testing.T) {
	// Given
	userID := uuid.New()
	nextCursor := &transaction.Cursor{
		CreatedAt: time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC),
		ID:        uuid.New(),
	}

	service := &serviceMock{}
	service.On("GetUserTransactions", mock.Anything, transaction.ListInput{UserID: userID}).
		Return(transaction.ListOutput{
			Transactions: []transaction.Transaction{},
			NextCursor:   nextCursor,
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/transactions?userId="+userID.String(), nil)
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.GetUserTransactions().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusOK, rr.Code)

	encoded := encodeCursor(t, *nextCursor)
	require.JSONEq(t, fmt.Sprintf(`{"data":[],"next_cursor":%q}`, encoded), rr.Body.String())
	mock.AssertExpectationsForObjects(t, service)
}

func TestTransaction_GetUserTransactions_MissingUserID(t *testing.T) {
	// Given
	service := &serviceMock{}
	req := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.GetUserTransactions().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "GetUserTransactions")
}

func TestTransaction_GetUserTransactions_MalformedUserID(t *testing.T) {
	// Given
	service := &serviceMock{}
	req := httptest.NewRequest(http.MethodGet, "/v1/transactions?userId=nope", nil)
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.GetUserTransactions().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "GetUserTransactions")
}

func TestTransaction_GetUserTransactions_MalformedCursor(t *testing.T) {
	// Given
	service := &serviceMock{}
	req := httptest.NewRequest(http.MethodGet, "/v1/transactions?userId="+uuid.New().String()+"&cursor=not-base64!!", nil)
	rr := httptest.NewRecorder()

	transactionHandler := handler.NewTransaction(service)

	// When
	transactionHandler.GetUserTransactions().ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "GetUserTransactions")
}

func TestTransaction_GetUserTransactions_PropagatesServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"user not found", user.ErrNotFound, http.StatusNotFound},
		{"validation", &transaction.ValidationError{Message: "invalid input"}, http.StatusBadRequest},
		{"generic error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			service := &serviceMock{}
			service.On("GetUserTransactions", mock.Anything, mock.Anything).
				Return(transaction.ListOutput{}, testCase.err)

			req := httptest.NewRequest(http.MethodGet, "/v1/transactions?userId="+uuid.New().String(), nil)
			rr := httptest.NewRecorder()

			transactionHandler := handler.NewTransaction(service)

			// When
			transactionHandler.GetUserTransactions().ServeHTTP(rr, req)

			// Then
			require.Equal(t, testCase.wantStatus, rr.Code)
			mock.AssertExpectationsForObjects(t, service)
		})
	}
}

func TestTransaction_CursorRoundTrip(t *testing.T) {
	// Given
	original := transaction.Cursor{
		CreatedAt: time.Date(2026, 4, 19, 13, 42, 11, 0, time.UTC),
		ID:        uuid.New(),
	}

	// When
	encoded := encodeCursor(t, original)
	decoded := decodeCursor(t, encoded)

	// Then
	require.NotNil(t, decoded)
	require.True(t, decoded.CreatedAt.Equal(original.CreatedAt))
	require.Equal(t, original.ID, decoded.ID)
}

func TestTransaction_ApproveTransaction(t *testing.T) {
	// Given
	id := uuid.New()
	sourceID := uuid.New()
	destinationID := uuid.New()
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	service := &serviceMock{}
	service.On("ApproveTransaction", mock.Anything, transaction.ApproveInput{ID: id}).
		Return(transaction.ApproveOutput{
			Transaction: transaction.Transaction{
				ID:            id,
				SourceID:      sourceID,
				DestinationID: destinationID,
				Amount:        decimal.NewFromInt(100),
				Status:        transaction.StatusApproved,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		}, nil)

	router := chi.NewRouter()
	transactionHandler := handler.NewTransaction(service)
	router.Method(http.MethodPatch, "/v1/transactions/{id}/approve", transactionHandler.ApproveTransaction())

	req := httptest.NewRequest(http.MethodPatch, "/v1/transactions/"+id.String()+"/approve", nil)
	rr := httptest.NewRecorder()

	// When
	router.ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, fmt.Sprintf(
		`{"id":%q,"source_id":%q,"destination_id":%q,"amount":"100","status":"approved","created_at":%q,"updated_at":%q}`,
		id, sourceID, destinationID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	), rr.Body.String())
	mock.AssertExpectationsForObjects(t, service)
}

func TestTransaction_ApproveTransaction_MalformedID(t *testing.T) {
	// Given
	service := &serviceMock{}

	router := chi.NewRouter()
	transactionHandler := handler.NewTransaction(service)
	router.Method(http.MethodPatch, "/v1/transactions/{id}/approve", transactionHandler.ApproveTransaction())

	req := httptest.NewRequest(http.MethodPatch, "/v1/transactions/nope/approve", nil)
	rr := httptest.NewRecorder()

	// When
	router.ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "ApproveTransaction")
}

func TestTransaction_ApproveTransaction_PropagatesServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"transaction not found", transaction.ErrNotFound, http.StatusNotFound},
		{"transaction not pending", transaction.ErrNotPending, http.StatusConflict},
		{"insufficient balance", transaction.ErrInsufficientBalance, http.StatusConflict},
		{"generic error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			service := &serviceMock{}
			service.On("ApproveTransaction", mock.Anything, mock.Anything).
				Return(transaction.ApproveOutput{}, testCase.err)

			router := chi.NewRouter()
			transactionHandler := handler.NewTransaction(service)
			router.Method(http.MethodPatch, "/v1/transactions/{id}/approve", transactionHandler.ApproveTransaction())

			req := httptest.NewRequest(http.MethodPatch, "/v1/transactions/"+uuid.New().String()+"/approve", nil)
			rr := httptest.NewRecorder()

			// When
			router.ServeHTTP(rr, req)

			// Then
			require.Equal(t, testCase.wantStatus, rr.Code)
			mock.AssertExpectationsForObjects(t, service)
		})
	}
}

func TestTransaction_RejectTransaction(t *testing.T) {
	// Given
	id := uuid.New()
	sourceID := uuid.New()
	destinationID := uuid.New()
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	service := &serviceMock{}
	service.On("RejectTransaction", mock.Anything, transaction.RejectInput{ID: id}).
		Return(transaction.RejectOutput{
			Transaction: transaction.Transaction{
				ID:            id,
				SourceID:      sourceID,
				DestinationID: destinationID,
				Amount:        decimal.NewFromInt(100),
				Status:        transaction.StatusRejected,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		}, nil)

	router := chi.NewRouter()
	transactionHandler := handler.NewTransaction(service)
	router.Method(http.MethodPatch, "/v1/transactions/{id}/reject", transactionHandler.RejectTransaction())

	req := httptest.NewRequest(http.MethodPatch, "/v1/transactions/"+id.String()+"/reject", nil)
	rr := httptest.NewRecorder()

	// When
	router.ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, fmt.Sprintf(
		`{"id":%q,"source_id":%q,"destination_id":%q,"amount":"100","status":"rejected","created_at":%q,"updated_at":%q}`,
		id, sourceID, destinationID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	), rr.Body.String())
	mock.AssertExpectationsForObjects(t, service)
}

func TestTransaction_RejectTransaction_MalformedID(t *testing.T) {
	// Given
	service := &serviceMock{}

	router := chi.NewRouter()
	transactionHandler := handler.NewTransaction(service)
	router.Method(http.MethodPatch, "/v1/transactions/{id}/reject", transactionHandler.RejectTransaction())

	req := httptest.NewRequest(http.MethodPatch, "/v1/transactions/nope/reject", nil)
	rr := httptest.NewRecorder()

	// When
	router.ServeHTTP(rr, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rr.Code)
	service.AssertNotCalled(t, "RejectTransaction")
}

func TestTransaction_RejectTransaction_PropagatesServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"transaction not found", transaction.ErrNotFound, http.StatusNotFound},
		{"transaction not pending", transaction.ErrNotPending, http.StatusConflict},
		{"generic error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			service := &serviceMock{}
			service.On("RejectTransaction", mock.Anything, mock.Anything).
				Return(transaction.RejectOutput{}, testCase.err)

			router := chi.NewRouter()
			transactionHandler := handler.NewTransaction(service)
			router.Method(http.MethodPatch, "/v1/transactions/{id}/reject", transactionHandler.RejectTransaction())

			req := httptest.NewRequest(http.MethodPatch, "/v1/transactions/"+uuid.New().String()+"/reject", nil)
			rr := httptest.NewRecorder()

			// When
			router.ServeHTTP(rr, req)

			// Then
			require.Equal(t, testCase.wantStatus, rr.Code)
			mock.AssertExpectationsForObjects(t, service)
		})
	}
}
