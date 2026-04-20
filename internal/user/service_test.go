package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mateoferrari97/belo-challenge/internal/user"
)

type repositoryMock struct {
	mock.Mock
}

func (r *repositoryMock) Get(ctx context.Context, in user.GetInput) (user.GetOutput, error) {
	args := r.Called(ctx, in)
	return args.Get(0).(user.GetOutput), args.Error(1)
}

func TestService_Get(t *testing.T) {
	// Given
	ctx := context.Background()
	id := uuid.New()
	want := user.GetOutput{User: user.User{ID: id, Name: "Alice", Email: "alice@belo.app"}}

	repository := &repositoryMock{}
	repository.On("Get", ctx, user.GetInput{ID: id}).Return(want, nil)

	service := user.NewService(repository)

	// When
	out, err := service.Get(ctx, user.GetInput{ID: id})

	// Then
	require.NoError(t, err)
	require.Equal(t, want, out)
	mock.AssertExpectationsForObjects(t, repository)
}

func TestService_Get_PropagatesRepositoryError(t *testing.T) {
	// Given
	ctx := context.Background()
	id := uuid.New()

	repository := &repositoryMock{}
	repository.On("Get", ctx, user.GetInput{ID: id}).Return(user.GetOutput{}, user.ErrNotFound)

	service := user.NewService(repository)

	// When
	_, err := service.Get(ctx, user.GetInput{ID: id})

	// Then
	require.ErrorIs(t, err, user.ErrNotFound)
	mock.AssertExpectationsForObjects(t, repository)
}

func TestService_Get_PropagatesGenericError(t *testing.T) {
	// Given
	ctx := context.Background()
	id := uuid.New()
	boom := errors.New("boom")

	repository := &repositoryMock{}
	repository.On("Get", ctx, user.GetInput{ID: id}).Return(user.GetOutput{}, boom)

	service := user.NewService(repository)

	// When
	_, err := service.Get(ctx, user.GetInput{ID: id})

	// Then
	require.ErrorIs(t, err, boom)
	mock.AssertExpectationsForObjects(t, repository)
}
