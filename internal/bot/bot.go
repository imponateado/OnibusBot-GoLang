package bot

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/leoteodoro/onibus-bot-go/internal/service"
)

type TelegramBot struct {
	bot     *tgbotapi.BotAPI
	router  *Router
	service *service.BusService
	token   string
}

func NewTelegramBot(token string, s *service.BusService, r *Router) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	t := &TelegramBot{
		bot:     bot,
		router:  r,
		service: s,
		token:   token,
	}
	s.SetNotifier(t)
	t.setBotCommands()
	return t, nil
}

func (t *TelegramBot) setBotCommands() {
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Iniciar o bot e ver instruções"},
		{Command: "grupos", Description: "Listar grupos de ônibus disponíveis"},
		{Command: "lowmode", Description: "Alternar modo de economia de dados (apenas texto)"},
		{Command: "info", Description: "Ver versão e informações do bot"},
	}

	config := tgbotapi.NewSetMyCommands(commands...)
	if _, err := t.bot.Request(config); err != nil {
		log.Printf("Erro ao configurar comandos do bot: %v", err)
	} else {
		log.Printf("Comandos do bot registrados com sucesso no Telegram.")
	}
}

func (t *TelegramBot) Start() {
	t.service.StartLoops()

	for {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates := t.bot.GetUpdatesChan(u)

		for update := range updates {
			go t.router.Route(update)
		}

		log.Printf("Conexão perdida. Reconectando em 5 segundos...")
		time.Sleep(5 * time.Second)
		if newBot, err := tgbotapi.NewBotAPI(t.token); err == nil {
			t.bot = newBot
			log.Printf("Reconectado com sucesso.")
		}
	}
}

// Implementação de service.Notifier
func (t *TelegramBot) NotifyLocation(chatID int64, lat, lon float64, text string) {
	t.bot.Send(tgbotapi.NewLocation(chatID, lat, lon))
	t.bot.Send(tgbotapi.NewMessage(chatID, text))
}

func (t *TelegramBot) NotifyMessage(chatID int64, text string, keyboard interface{}) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	if k, ok := keyboard.(string); ok && k == "stop_button" {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Parar tudo", fmt.Sprintf("stop_%d", chatID)),
			),
		)
	} else if k, ok := keyboard.(tgbotapi.InlineKeyboardMarkup); ok {
		msg.ReplyMarkup = k
	}

	t.bot.Send(msg)
}
