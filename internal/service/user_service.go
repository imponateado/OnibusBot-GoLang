package service

import (
	"log"
	"sync"
	"time"

	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type UserService struct {
	repo  domain.UserRepository
	users []domain.RegisteredUser
	cache sync.Map // Map[int64]bool
	mu    sync.Mutex
}

func NewUserService(repo domain.UserRepository) *UserService {
	s := &UserService{
		repo: repo,
	}
	s.load()
	return s
}

func (s *UserService) load() {
	users, err := s.repo.Load()
	if err != nil {
		log.Printf("Erro ao carregar usuários: %v", err)
		return
	}
	s.users = users
	for _, u := range users {
		s.cache.Store(u.ChatID, true)
	}
}

func (s *UserService) Register(chatID int64, username string) {
	if _, ok := s.cache.Load(chatID); ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check
	if _, ok := s.cache.Load(chatID); ok {
		return
	}

	newUser := domain.RegisteredUser{
		ChatID:   chatID,
		Username: username,
		JoinedAt: time.Now(),
	}

	s.users = append(s.users, newUser)
	s.cache.Store(chatID, true)

	if err := s.repo.Save(s.users); err != nil {
		log.Printf("Erro ao salvar novos usuários: %v", err)
	} else {
		log.Printf("Novo usuário registrado: %s (%d)", username, chatID)
	}
}

func (s *UserService) GetAllUsers() []domain.RegisteredUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.users
}
