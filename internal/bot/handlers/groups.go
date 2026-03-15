package handlers

import (
	"fmt"

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

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, g := range groups {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🚌 %s", g), fmt.Sprintf("select_group_%s", g)),
		))
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, "Selecione um grupo de ônibus abaixo para rastrear todas as suas linhas:")
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.Send(reply)
	return nil
}
