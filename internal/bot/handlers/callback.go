package handlers

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type CallbackHandler struct {
	Service *service.BusService
}

func (h *CallbackHandler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	cb := update.CallbackQuery
	data := cb.Data
	chatID := cb.Message.Chat.ID

	// Answer callback to remove loading state
	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	switch {
	case strings.HasPrefix(data, "stop_"):
		removidos, err := h.Service.UnsubscribeAll(chatID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "Erro ao cancelar inscrições."))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Todas as notificações foram canceladas! (%d remoções)", removidos)))
		}

	case strings.HasPrefix(data, "sentido_"):
		parts := strings.Split(data, "_")
		if len(parts) >= 3 {
			h.ProcessSubscription(bot, chatID, parts[1], parts[2], false)
		}

	case strings.HasPrefix(data, "gsentido_"):
		parts := strings.Split(data, "_")
		if len(parts) >= 3 {
			h.ProcessGroupSubscription(bot, chatID, parts[1], parts[2])
		}

	default:
		// Se for apenas o nome da linha
		if h.Service.IsLinhaValida(data) {
			msgConfig := tgbotapi.NewMessage(chatID, "Escolha o sentido (0=Ida, 1=Volta):")
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("IDA", fmt.Sprintf("sentido_%s_0", data)),
					tgbotapi.NewInlineKeyboardButtonData("VOLTA", fmt.Sprintf("sentido_%s_1", data)),
				),
			)
			msgConfig.ReplyMarkup = keyboard
			bot.Send(msgConfig)
		}
	}

	return nil
}

func (h *CallbackHandler) ProcessSubscription(bot *tgbotapi.BotAPI, chatID int64, linha, sentido string, quiet bool) {
	err := h.Service.Subscribe(chatID, linha, sentido)
	if err != nil {
		if !quiet {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Erro na linha %s: %v", linha, err)))
		}
		return
	}

	onibus, lowMode := h.Service.GetBusStatus(chatID, linha, sentido)
	if len(onibus) == 0 {
		if !quiet {
			msg := tgbotapi.NewMessage(chatID, "Vigilância iniciada! Você será avisado quando um ônibus for encontrado.")
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("❌ Parar", fmt.Sprintf("stop_%d", chatID)),
				),
			)
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
		}
	} else {
		h.Service.NotifyBuses(chatID, onibus, linha, sentido, lowMode)
		if !quiet {
			h.sendInstructionMessage(bot, chatID)
		}
		h.Service.SetJaRecebeuPrimeiraMensagem(chatID, linha, sentido)
	}
}

func (h *CallbackHandler) ProcessGroupSubscription(bot *tgbotapi.BotAPI, chatID int64, groupName, sentido string) {
	lines := h.Service.GetGroup(groupName)
	if len(lines) == 0 {
		return
	}

	bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ Rastreando %d linhas do grupo %s...", len(lines), groupName)))

	var inscritas []string
	var falhas []string

	for _, l := range lines {
		if err := h.Service.Subscribe(chatID, l, sentido); err == nil {
			inscritas = append(inscritas, l)
			// Envia localização inicial se houver
			onibus, lowMode := h.Service.GetBusStatus(chatID, l, sentido)
			if len(onibus) > 0 {
				h.Service.NotifyBuses(chatID, onibus, l, sentido, lowMode)
				h.Service.SetJaRecebeuPrimeiraMensagem(chatID, l, sentido)
			}
		} else {
			falhas = append(falhas, l)
		}
	}

	var text strings.Builder
	text.WriteString(fmt.Sprintf("✅ *Grupo %s Concluído*\n\n", groupName))
	text.WriteString(fmt.Sprintf("📝 *Linhas:* %s\n", strings.Join(inscritas, ", ")))
	if len(falhas) > 0 {
		text.WriteString(fmt.Sprintf("\n⚠️ *Falhas:* %s\n", strings.Join(falhas, ", ")))
	}
	
	msg := tgbotapi.NewMessage(chatID, text.String())
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func (h *CallbackHandler) sendInstructionMessage(bot *tgbotapi.BotAPI, chatID int64) {
	text := "Você será notificado a cada 2 minutos.\nClique abaixo para parar:"
	msg := tgbotapi.NewMessage(chatID, text)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Parar Notificações", fmt.Sprintf("stop_%d", chatID)),
		),
	)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
