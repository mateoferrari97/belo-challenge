package user

import "context"

type repository interface {
	Get(ctx context.Context, in GetInput) (GetOutput, error)
}

type Service struct {
	repository repository
}

func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Get(ctx context.Context, in GetInput) (GetOutput, error) {
	return s.repository.Get(ctx, in)
}
