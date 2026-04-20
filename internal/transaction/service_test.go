package transaction_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mateoferrari97/belo-challenge/internal/transaction"
	"github.com/mateoferrari97/belo-challenge/internal/user"
)

type repositoryMock struct {
	mock.Mock
}

func (r *repositoryMock) CreateTransaction(ctx context.Context, in transaction.SaveInput) (transaction.SaveOutput, error) {
	args := r.Called(ctx, in)
	return args.Get(0).(transaction.SaveOutput), args.Error(1)
}

func (r *repositoryMock) GetUserTransactions(ctx context.Context, in transaction.ListInput) (transaction.ListOutput, error) {
	args := r.Called(ctx, in)
	return args.Get(0).(transaction.ListOutput), args.Error(1)
}

func (r *repositoryMock) ApproveTransaction(ctx context.Context, in transaction.ApproveInput) (transaction.ApproveOutput, error) {
	args := r.Called(ctx, in)
	return args.Get(0).(transaction.ApproveOutput), args.Error(1)
}

func (r *repositoryMock) RejectTransaction(ctx context.Context, in transaction.RejectInput) (transaction.RejectOutput, error) {
	args := r.Called(ctx, in)
	return args.Get(0).(transaction.RejectOutput), args.Error(1)
}

type usersMock struct {
	mock.Mock
}

func (u *usersMock) Get(ctx context.Context, in user.GetInput) (user.GetOutput, error) {
	args := u.Called(ctx, in)
	return args.Get(0).(user.GetOutput), args.Error(1)
}

func TestService_CreateTransaction(t *testing.T) {
	// Given
	ctx := context.Background()
	sourceID := uuid.New()
	destinationID := uuid.New()
	amount := decimal.NewFromInt(1500)

	persisted := transaction.Transaction{
		SourceID:      sourceID,
		DestinationID: destinationID,
		Amount:        amount,
		Status:        transaction.StatusApproved,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	repository := &repositoryMock{}
	repository.On("CreateTransaction", ctx, mock.MatchedBy(func(in transaction.SaveInput) bool {
		return in.Transaction.ID != uuid.Nil &&
			in.Transaction.SourceID == sourceID &&
			in.Transaction.DestinationID == destinationID &&
			in.Transaction.Amount.Equal(amount) &&
			in.Transaction.Status == transaction.StatusApproved
	})).Return(transaction.SaveOutput{Transaction: persisted}, nil)

	service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

	// When
	out, err := service.CreateTransaction(ctx, transaction.CreateInput{
		SourceID:      sourceID,
		DestinationID: destinationID,
		Amount:        amount,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, transaction.StatusApproved, out.Transaction.Status)
	require.True(t, out.Transaction.Amount.Equal(amount))
	mock.AssertExpectationsForObjects(t, repository)
}

func TestService_CreateTransaction_FlagsPendingAboveThreshold(t *testing.T) {
	// Given
	ctx := context.Background()
	sourceID := uuid.New()
	destinationID := uuid.New()
	amount := decimal.NewFromInt(50001)

	repository := &repositoryMock{}
	repository.On("CreateTransaction", ctx, mock.MatchedBy(func(in transaction.SaveInput) bool {
		return in.Transaction.Status == transaction.StatusPending
	})).Return(transaction.SaveOutput{Transaction: transaction.Transaction{Status: transaction.StatusPending}}, nil)

	service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

	// When
	out, err := service.CreateTransaction(ctx, transaction.CreateInput{
		SourceID:      sourceID,
		DestinationID: destinationID,
		Amount:        amount,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, transaction.StatusPending, out.Transaction.Status)
	mock.AssertExpectationsForObjects(t, repository)
}

func TestService_CreateTransaction_RejectsInvalidInput(t *testing.T) {
	sourceID := uuid.New()
	destinationID := uuid.New()

	cases := []struct {
		name string
		in   transaction.CreateInput
	}{
		{"missing source", transaction.CreateInput{DestinationID: destinationID, Amount: decimal.NewFromInt(100)}},
		{"missing destination", transaction.CreateInput{SourceID: sourceID, Amount: decimal.NewFromInt(100)}},
		{"same source and destination", transaction.CreateInput{SourceID: sourceID, DestinationID: sourceID, Amount: decimal.NewFromInt(100)}},
		{"zero amount", transaction.CreateInput{SourceID: sourceID, DestinationID: destinationID, Amount: decimal.Zero}},
		{"negative amount", transaction.CreateInput{SourceID: sourceID, DestinationID: destinationID, Amount: decimal.NewFromInt(-1)}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			repository := &repositoryMock{}
			service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

			// When
			_, err := service.CreateTransaction(context.Background(), testCase.in)

			// Then
			require.ErrorIs(t, err, transaction.ErrValidation)
			repository.AssertNotCalled(t, "CreateTransaction")
		})
	}
}

func TestService_CreateTransaction_PropagatesRepositoryError(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"source not found", transaction.ErrSourceNotFound},
		{"destination not found", transaction.ErrDestinationNotFound},
		{"insufficient balance", transaction.ErrInsufficientBalance},
		{"generic error", errors.New("boom")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			ctx := context.Background()
			repository := &repositoryMock{}
			repository.On("CreateTransaction", ctx, mock.Anything).Return(transaction.SaveOutput{}, testCase.err)

			service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

			// When
			_, err := service.CreateTransaction(ctx, transaction.CreateInput{
				SourceID:      uuid.New(),
				DestinationID: uuid.New(),
				Amount:        decimal.NewFromInt(100),
			})

			// Then
			require.ErrorIs(t, err, testCase.err)
			mock.AssertExpectationsForObjects(t, repository)
		})
	}
}

func TestService_GetUserTransactions(t *testing.T) {
	// Given
	ctx := context.Background()
	userID := uuid.New()
	in := transaction.ListInput{UserID: userID}

	output := transaction.ListOutput{
		Transactions: []transaction.Transaction{
			{ID: uuid.New(), SourceID: userID, Amount: decimal.NewFromInt(10), Status: transaction.StatusApproved},
			{ID: uuid.New(), DestinationID: userID, Amount: decimal.NewFromInt(20), Status: transaction.StatusApproved},
		},
		NextCursor: &transaction.Cursor{CreatedAt: time.Now(), ID: uuid.New()},
	}

	users := &usersMock{}
	users.On("Get", ctx, user.GetInput{ID: userID}).Return(user.GetOutput{}, nil)

	repository := &repositoryMock{}
	repository.On("GetUserTransactions", ctx, in).Return(output, nil)

	service := transaction.NewService(repository, users, decimal.NewFromInt(50000))

	// When
	out, err := service.GetUserTransactions(ctx, in)

	// Then
	require.NoError(t, err)
	require.Len(t, out.Transactions, 2)
	require.NotNil(t, out.NextCursor)
	mock.AssertExpectationsForObjects(t, repository, users)
}

func TestService_GetUserTransactions_RejectsInvalidUserID(t *testing.T) {
	// Given
	repository := &repositoryMock{}
	users := &usersMock{}
	service := transaction.NewService(repository, users, decimal.NewFromInt(50000))

	// When
	_, err := service.GetUserTransactions(context.Background(), transaction.ListInput{UserID: uuid.Nil})

	// Then
	require.ErrorIs(t, err, transaction.ErrValidation)
	users.AssertNotCalled(t, "Get")
	repository.AssertNotCalled(t, "GetUserTransactions")
}

func TestService_GetUserTransactions_PropagatesUserNotFound(t *testing.T) {
	// Given
	ctx := context.Background()
	userID := uuid.New()

	users := &usersMock{}
	users.On("Get", ctx, user.GetInput{ID: userID}).Return(user.GetOutput{}, user.ErrNotFound)

	repository := &repositoryMock{}
	service := transaction.NewService(repository, users, decimal.NewFromInt(50000))

	// When
	_, err := service.GetUserTransactions(ctx, transaction.ListInput{UserID: userID})

	// Then
	require.ErrorIs(t, err, user.ErrNotFound)
	repository.AssertNotCalled(t, "GetUserTransactions")
	mock.AssertExpectationsForObjects(t, users)
}

func TestService_GetUserTransactions_PropagatesRepositoryError(t *testing.T) {
	// Given
	ctx := context.Background()
	userID := uuid.New()
	boom := errors.New("boom")

	users := &usersMock{}
	users.On("Get", ctx, user.GetInput{ID: userID}).Return(user.GetOutput{}, nil)

	repository := &repositoryMock{}
	repository.On("GetUserTransactions", ctx, transaction.ListInput{UserID: userID}).
		Return(transaction.ListOutput{}, boom)

	service := transaction.NewService(repository, users, decimal.NewFromInt(50000))

	// When
	_, err := service.GetUserTransactions(ctx, transaction.ListInput{UserID: userID})

	// Then
	require.ErrorIs(t, err, boom)
	mock.AssertExpectationsForObjects(t, repository, users)
}

func TestService_ApproveTransaction(t *testing.T) {
	// Given
	ctx := context.Background()
	id := uuid.New()
	output := transaction.ApproveOutput{
		Transaction: transaction.Transaction{ID: id, Status: transaction.StatusApproved},
	}

	repository := &repositoryMock{}
	repository.On("ApproveTransaction", ctx, transaction.ApproveInput{ID: id}).Return(output, nil)

	service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

	// When
	out, err := service.ApproveTransaction(ctx, transaction.ApproveInput{ID: id})

	// Then
	require.NoError(t, err)
	require.Equal(t, id, out.Transaction.ID)
	require.Equal(t, transaction.StatusApproved, out.Transaction.Status)
	mock.AssertExpectationsForObjects(t, repository)
}

func TestService_ApproveTransaction_RejectsInvalidID(t *testing.T) {
	// Given
	repository := &repositoryMock{}
	service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

	// When
	_, err := service.ApproveTransaction(context.Background(), transaction.ApproveInput{ID: uuid.Nil})

	// Then
	require.ErrorIs(t, err, transaction.ErrValidation)
	repository.AssertNotCalled(t, "ApproveTransaction")
}

func TestService_ApproveTransaction_PropagatesRepositoryError(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"transaction not found", transaction.ErrNotFound},
		{"transaction not pending", transaction.ErrNotPending},
		{"insufficient balance", transaction.ErrInsufficientBalance},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			ctx := context.Background()
			id := uuid.New()

			repository := &repositoryMock{}
			repository.On("ApproveTransaction", ctx, transaction.ApproveInput{ID: id}).
				Return(transaction.ApproveOutput{}, testCase.err)

			service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

			// When
			_, err := service.ApproveTransaction(ctx, transaction.ApproveInput{ID: id})

			// Then
			require.ErrorIs(t, err, testCase.err)
			mock.AssertExpectationsForObjects(t, repository)
		})
	}
}

func TestService_RejectTransaction(t *testing.T) {
	// Given
	ctx := context.Background()
	id := uuid.New()
	output := transaction.RejectOutput{
		Transaction: transaction.Transaction{ID: id, Status: transaction.StatusRejected},
	}

	repository := &repositoryMock{}
	repository.On("RejectTransaction", ctx, transaction.RejectInput{ID: id}).Return(output, nil)

	service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

	// When
	out, err := service.RejectTransaction(ctx, transaction.RejectInput{ID: id})

	// Then
	require.NoError(t, err)
	require.Equal(t, transaction.StatusRejected, out.Transaction.Status)
	mock.AssertExpectationsForObjects(t, repository)
}

func TestService_RejectTransaction_RejectsInvalidID(t *testing.T) {
	// Given
	repository := &repositoryMock{}
	service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

	// When
	_, err := service.RejectTransaction(context.Background(), transaction.RejectInput{ID: uuid.Nil})

	// Then
	require.ErrorIs(t, err, transaction.ErrValidation)
	repository.AssertNotCalled(t, "RejectTransaction")
}

func TestService_RejectTransaction_PropagatesRepositoryError(t *testing.T) {
	cases := []error{
		transaction.ErrNotFound,
		transaction.ErrNotPending,
	}

	for _, target := range cases {
		t.Run(target.Error(), func(t *testing.T) {
			// Given
			ctx := context.Background()
			id := uuid.New()

			repository := &repositoryMock{}
			repository.On("RejectTransaction", ctx, transaction.RejectInput{ID: id}).
				Return(transaction.RejectOutput{}, target)

			service := transaction.NewService(repository, &usersMock{}, decimal.NewFromInt(50000))

			// When
			_, err := service.RejectTransaction(ctx, transaction.RejectInput{ID: id})

			// Then
			require.ErrorIs(t, err, target)
			mock.AssertExpectationsForObjects(t, repository)
		})
	}
}
