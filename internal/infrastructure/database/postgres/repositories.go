package postgres

import "context"

type RepositoryFactory struct{}

func NewRepositoryFactory() *RepositoryFactory {
	return &RepositoryFactory{}
}

func (f *RepositoryFactory) Ping(_ context.Context) error {
	return nil
}
