package repository

import (
	"encoding/json"
	"os"
	"sync"
	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type jsonSubscriptionRepository struct {
	path string
	mu   sync.Mutex
}

func NewJSONSubscriptionRepository(path string) domain.SubscriptionRepository {
	return &jsonSubscriptionRepository{path: path}
}

func (r *jsonSubscriptionRepository) Save(subs []domain.UserSubscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(subs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.path, data, 0644)
}

func (r *jsonSubscriptionRepository) Load() ([]domain.UserSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}

	var subs []domain.UserSubscription
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, err
	}

	return subs, nil
}
