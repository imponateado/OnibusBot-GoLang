package handlers

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type LowModeHandler struct {
	Service *service.BusService
}

func (h *LowModeHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	currentMode, err := h.Service.ToggleLowMode(msg.Chat.ID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Você ainda não tem linhas inscritas."))
		return nil
	}

	status := "Ativado (Apenas texto) 🐌"
	if !currentMode {
		status = "Desativado (Mapas habilitados) 🚀"
	}
	bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Modo Econômico: %s", status)))
	return nil
}
