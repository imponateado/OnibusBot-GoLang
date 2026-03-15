package handlers

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type GroupsHandler struct {
	Service *service.BusService
}

func (h *GroupsHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	groups := h.Service.GetGroupsList()
	if len(groups) == 0 {
		bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Nenhum grupo cadastrado no momento."))
		return nil
	}

	var text strings.Builder
	text.WriteString("🚌 *Grupos de Ônibus Disponíveis:*\n\n")
	for _, g := range groups {
		text.WriteString(fmt.Sprintf("• %s\n", g))
	}
	text.WriteString("\n💡 Digite o nome de qualquer grupo acima para rastrear suas linhas.")

	reply := tgbotapi.NewMessage(msg.Chat.ID, text.String())
	reply.ParseMode = "Markdown"
	bot.Send(reply)
	return nil
}
