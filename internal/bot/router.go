package bot

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type Handler interface {
	Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error
}

type Router struct {
	bot      *tgbotapi.BotAPI
	service  *service.BusService
	handlers map[string]Handler
}

func NewRouter(bot *tgbotapi.BotAPI, s *service.BusService) *Router {
	return &Router{
		bot:      bot,
		service:  s,
		handlers: make(map[string]Handler),
	}
}

func (r *Router) Register(command string, h Handler) {
	r.handlers[command] = h
}

func (r *Router) Route(update tgbotapi.Update) {
	if update.Message != nil {
		r.handleMessage(update)
	} else if update.CallbackQuery != nil {
		r.handleCallback(update)
	}
}

func (r *Router) handleMessage(update tgbotapi.Update) {
	text := update.Message.Text
	
	// Tenta encontrar um handler específico para o comando (ex: /start)
	if h, ok := r.handlers[text]; ok {
		if err := h.Handle(r.bot, update); err != nil {
			log.Printf("Erro no handler %s: %v", text, err)
		}
		return
	}

	// Default: Busca de linha ou grupo
	if h, ok := r.handlers["search"]; ok {
		h.Handle(r.bot, update)
	}
}

func (r *Router) handleCallback(update tgbotapi.Update) {
	data := update.CallbackQuery.Data
	
	// 1. Roteamento baseado em prefixos conhecidos
	prefixes := []string{"stop_", "sentido_", "gsentido_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(data, prefix) {
			if h, ok := r.handlers[prefix]; ok {
				h.Handle(r.bot, update)
				return
			}
		}
	}

	// 2. Default: Tratar como seleção de linha simples
	if h, ok := r.handlers["callback_default"]; ok {
		h.Handle(r.bot, update)
	}
}
