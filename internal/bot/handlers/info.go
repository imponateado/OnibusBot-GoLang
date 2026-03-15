package handlers

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type InfoHandler struct {
	Version string
}

func (h *InfoHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}
	bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Versão do Bot: %s", h.Version)))
	return nil
}
