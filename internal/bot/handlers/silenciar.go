package handlers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type SilenciarHandler struct {
	BroadcastService *service.BroadcastService
}

func (h *SilenciarHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	chatID := msg.Chat.ID

	if h.BroadcastService.IsOptedOut(chatID) {
		h.BroadcastService.OptIn(chatID)
		bot.Send(tgbotapi.NewMessage(chatID, "🔔 Avisos reativados! Você voltará a receber mensagens de broadcast."))
	} else {
		bot.Send(tgbotapi.NewMessage(chatID, "✅ Você já está recebendo avisos normalmente."))
	}

	return nil
}
