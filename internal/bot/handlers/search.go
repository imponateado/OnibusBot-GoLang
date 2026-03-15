package handlers

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type SearchHandler struct {
	Service *service.BusService
}

func (h *SearchHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	userInput := strings.ToUpper(strings.TrimSpace(msg.Text))

	// 1. Tenta buscar como grupo
	groupLines := h.Service.GetGroup(userInput)
	if len(groupLines) > 0 {
		msgConfig := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Você selecionou o grupo *%s* (%d linhas).\n\nEscolha o sentido para rastrear todas de uma vez:", userInput, len(groupLines)))
		msgConfig.ParseMode = "Markdown"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("IDA (0)", fmt.Sprintf("gsentido_%s_0", userInput)),
				tgbotapi.NewInlineKeyboardButtonData("VOLTA (1)", fmt.Sprintf("gsentido_%s_1", userInput)),
			),
		)
		msgConfig.ReplyMarkup = keyboard
		bot.Send(msgConfig)
		return nil
	}

	// 2. Tenta buscar como linha
	linhasEncontradas := h.Service.GetLinhasDisponiveis(userInput)
	if len(linhasEncontradas) > 0 {
		var rows [][]tgbotapi.InlineKeyboardButton
		maxRows := 10
		if len(linhasEncontradas) < maxRows {
			maxRows = len(linhasEncontradas)
		}
		for i := 0; i < maxRows; i++ {
			l := linhasEncontradas[i]
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(l, l),
			))
		}
		msgConfig := tgbotapi.NewMessage(msg.Chat.ID, "Selecione a linha")
		msgConfig.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		bot.Send(msgConfig)
	} else {
		bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Nenhuma linha encontrada."))
	}

	return nil
}
