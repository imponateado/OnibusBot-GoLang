package service

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type BroadcastNotifier interface {
	NotifyMessage(chatID int64, text string, keyboard interface{})
}

type BroadcastService struct {
	filePath    string
	userService *UserService
	prefsRepo   domain.UserPrefsRepository
	optOutUsers map[int64]bool
	notifier    BroadcastNotifier
	mu          sync.Mutex
}

func NewBroadcastService(filePath string, userService *UserService, prefsRepo domain.UserPrefsRepository) *BroadcastService {
	s := &BroadcastService{
		filePath:    filePath,
		userService: userService,
		prefsRepo:   prefsRepo,
		optOutUsers: make(map[int64]bool),
	}

	optOut, err := prefsRepo.LoadBroadcastOptOut()
	if err != nil {
		log.Printf("[BROADCAST] Erro ao carregar preferências de opt-out: %v", err)
	} else {
		s.optOutUsers = optOut
	}

	return s
}

func (b *BroadcastService) SetNotifier(n BroadcastNotifier) {
	b.notifier = n
}

func (b *BroadcastService) ToggleOptOut(chatID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.optOutUsers[chatID] = !b.optOutUsers[chatID]
	current := b.optOutUsers[chatID]
	b.prefsRepo.SaveBroadcastOptOut(b.optOutUsers)
	return current
}

func (b *BroadcastService) OptOut(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.optOutUsers[chatID] = true
	b.prefsRepo.SaveBroadcastOptOut(b.optOutUsers)
}

func (b *BroadcastService) OptIn(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.optOutUsers[chatID] = false
	b.prefsRepo.SaveBroadcastOptOut(b.optOutUsers)
}

func (b *BroadcastService) IsOptedOut(chatID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.optOutUsers[chatID]
}

func (b *BroadcastService) StartLoop() {
	go b.loop()
}

func (b *BroadcastService) loop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		b.check()
	}
}

func (b *BroadcastService) check() {
	if b.notifier == nil {
		return
	}

	data, err := os.ReadFile(b.filePath)
	if err != nil {
		return
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return
	}

	users := b.userService.GetAllUsers()
	if len(users) == 0 {
		log.Printf("[BROADCAST] Nenhum usuário registrado para enviar broadcast")
		return
	}

	log.Printf("[BROADCAST] Enviando mensagem para %d usuários...", len(users))

	sent, skipped := b.send(users, content)

	log.Printf("[BROADCAST] Concluído: %d enviados, %d silenciados", sent, skipped)

	if err := os.Remove(b.filePath); err != nil {
		log.Printf("[BROADCAST] Erro ao remover %s: %v", b.filePath, err)
	}
}

func (b *BroadcastService) send(users []domain.RegisteredUser, content string) (sent, skipped int) {
	b.mu.Lock()
	optOut := make(map[int64]bool, len(b.optOutUsers))
	for k, v := range b.optOutUsers {
		optOut[k] = v
	}
	b.mu.Unlock()

	for _, user := range users {
		if optOut[user.ChatID] {
			skipped++
			continue
		}

		msg := fmt.Sprintf("📢 *Aviso*\n\n%s", content)
		b.notifier.NotifyMessage(user.ChatID, msg, "broadcast_optout_button")
		sent++
		time.Sleep(100 * time.Millisecond)
	}
	return sent, skipped
}
