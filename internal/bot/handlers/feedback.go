package handlers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type FeedbackHandler struct{}

func (h *FeedbackHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	text := "💬 Tem uma sugestão, bug ou feedback?\n\nClique no botão abaixo para abrir uma issue no GitHub:"
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📝 Enviar Feedback", "https://github.com/imponateado/OnibusBot-GoLang/issues/new"),
		),
	)
	bot.Send(reply)
	return nil
}
