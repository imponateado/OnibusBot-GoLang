package repository

import (
	"encoding/json"
	"os"
	"sync"
	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type jsonUserRepository struct {
	path string
	mu   sync.Mutex
}

func NewJSONUserRepository(path string) domain.UserRepository {
	return &jsonUserRepository{path: path}
}

func (r *jsonUserRepository) Save(users []domain.RegisteredUser) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.path, data, 0644)
}

func (r *jsonUserRepository) Load() ([]domain.RegisteredUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}

	var users []domain.RegisteredUser
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}

	return users, nil
}
