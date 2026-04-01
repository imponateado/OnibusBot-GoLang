package handlers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type StartHandler struct {
	Service *service.BusService
}

func (h *StartHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	msgStart := "Olá! Digite o número da linha que você deseja acompanhar (ex: 2210) ou o nome de um grupo (ex: EPNB):\n\n"

	reply := tgbotapi.NewMessage(msg.Chat.ID, msgStart)
	reply.ParseMode = "Markdown"
	bot.Send(reply)
	return nil
}
