package handlers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type BroadcastOptOutHandler struct {
	BroadcastService *service.BroadcastService
}

func (h *BroadcastOptOutHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	cb := update.CallbackQuery
	if cb == nil {
		return nil
	}

	chatID := cb.Message.Chat.ID
	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	h.BroadcastService.OptOut(chatID)
	bot.Send(tgbotapi.NewMessage(chatID, "🔕 Avisos silenciados. Use /silenciar para reativar."))

	return nil
}
