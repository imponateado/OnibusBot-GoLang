package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotService struct {
	bot               *tgbotapi.BotAPI
	apiClient         *APIClient
	userSubscriptions []UserSubscription
	mu                sync.Mutex
	ultimaPosicao     *UltimaPosicao
	linhasDisponiveis []string
	version           string
}

func NewBotService(token string, version string) (*BotService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &BotService{
		bot:       bot,
		apiClient: NewAPIClient(),
		version:   version,
	}, nil
}

func (s *BotService) Start() {
	log.Printf("Iniciando OnibusBot-Go versão %s...", s.version)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := s.bot.GetUpdatesChan(u)

	// Carregar dados iniciais
	if err := s.UpdateData(); err != nil {
		log.Printf("Erro ao carregar dados iniciais: %v", err)
	}

	// Iniciar tickers
	go s.NotificationLoop()
	go s.UnsubscribeButtonLoop()
	go s.DataUpdateLoop()

	for update := range updates {
		if update.Message != nil {
			s.HandleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			s.HandleCallback(update.CallbackQuery)
		}
	}
}

func (s *BotService) UpdateData() error {
	posicao, err := s.apiClient.GetLinhasDeOnibus()
	if err != nil {
		log.Printf("Erro ao buscar linhas: %v", err)
		return err
	}
	
	s.mu.Lock()
	s.linhasDisponiveis = nil
	seen := make(map[string]bool)
	for _, f := range posicao.Features {
		l := f.Properties.Linha // cd_linha no UltimaPosicao
		if l != "" && !seen[l] {
			s.linhasDisponiveis = append(s.linhasDisponiveis, l)
			seen[l] = true
		}
	}
	s.ultimaPosicao = s.CleanUltimaPosicao(posicao)
	s.mu.Unlock()

	return nil
}

func (s *BotService) CleanUltimaPosicao(posicao *UltimaPosicao) *UltimaPosicao {
	var cleanFeatures []UltimaFeature
	for _, f := range posicao.Features {
		if f.Properties.Linha != "" {
			cleanFeatures = append(cleanFeatures, f)
		}
	}
	posicao.Features = cleanFeatures
	return posicao
}

func (s *BotService) HandleMessage(msg *tgbotapi.Message) {
	if msg.Text == "/info" {
		s.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Versão do Bot: %s", s.version)))
		return
	}

	if msg.Text == "/start" || strings.ToLower(msg.Text) == "oi" {
		s.mu.Lock()
		up := s.ultimaPosicao
		s.mu.Unlock()

		if up == nil {
			s.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "⚠️ Desculpe, o serviço está temporariamente indisponível (dados não carregados)."))
			return
		}

		s.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Olá! Digite o número da linha que você deseja acompanhar (ex: 2210):"))
		return
	}

	// Se for um número de linha
	linhaInput := strings.ToUpper(msg.Text)
	var linhasEncontradas []string
	s.mu.Lock()
	for _, l := range s.linhasDisponiveis {
		if strings.Contains(l, linhaInput) {
			linhasEncontradas = append(linhasEncontradas, l)
		}
	}
	s.mu.Unlock()

	if len(linhasEncontradas) > 0 {
		var rows [][]tgbotapi.InlineKeyboardButton
		maxRows := 10 // Limitar para não estourar o Telegram
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
		s.bot.Send(msgConfig)
	} else {
		s.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Nenhuma linha encontrada."))
	}
}

func (s *BotService) HandleCallback(cb *tgbotapi.CallbackQuery) {
	callbackData := cb.Data
	chatID := cb.Message.Chat.ID

	callback := tgbotapi.NewCallback(cb.ID, "")
	s.bot.Request(callback)

	if strings.HasPrefix(callbackData, "stop_") {
		s.mu.Lock()
		var newSubs []UserSubscription
		removidos := 0
		for _, sub := range s.userSubscriptions {
			if sub.ChatID == chatID {
				removidos++
			} else {
				newSubs = append(newSubs, sub)
			}
		}
		s.userSubscriptions = newSubs
		s.mu.Unlock()

		s.bot.Send(tgbotapi.NewMessage(chatID, "✅ Todas as notificações foram canceladas!"))
		log.Printf("Notificações canceladas para chat %d, %d inscrições removidas", chatID, removidos)
		return
	}

	if strings.HasPrefix(callbackData, "sentido_") {
		parts := strings.Split(callbackData, "_")
		if len(parts) >= 3 {
			linha := parts[1]
			sentido := parts[2]
			s.ProcessAndSendBusStatus(chatID, linha, sentido)
		}
		return
	}

	// Se for apenas o nome da linha (selecionada no menu de linhas encontradas)
	s.mu.Lock()
	isLinha := false
	for _, l := range s.linhasDisponiveis {
		if l == callbackData {
			isLinha = true
			break
		}
	}
	s.mu.Unlock()

	if isLinha {
		msgConfig := tgbotapi.NewMessage(chatID, "Escolha o sentido (0=Ida, 1=Volta):")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("IDA", fmt.Sprintf("sentido_%s_0", callbackData)),
				tgbotapi.NewInlineKeyboardButtonData("VOLTA", fmt.Sprintf("sentido_%s_1", callbackData)),
			),
		)
		msgConfig.ReplyMarkup = keyboard
		s.bot.Send(msgConfig)
		return
	}
}

func (s *BotService) ProcessAndSendBusStatus(chatID int64, linha, sentido string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ultimaPosicao == nil {
		s.bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Desculpe, o serviço está temporariamente indisponível."))
		return
	}

	count := 0
	jaExiste := false
	for _, sub := range s.userSubscriptions {
		if sub.ChatID == chatID {
			count++
			if sub.Linha == linha && sub.Sentido == sentido {
				jaExiste = true
			}
		}
	}

	if jaExiste {
		s.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Já foi encontrado uma inscrição pra linha %s!", linha)))
		return
	}

	if count >= 10 {
		msg := tgbotapi.NewMessage(chatID, "🚫 Você atingiu o limite máximo de 10 linhas monitoradas!\n\nPara adicionar uma nova linha, primeiro cancele algumas das existentes.")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancelar todas as inscrições.", fmt.Sprintf("stop_%d", chatID)),
			),
		)
		msg.ReplyMarkup = keyboard
		s.bot.Send(msg)
		return
	}

	s.userSubscriptions = append(s.userSubscriptions, UserSubscription{
		ChatID:  chatID,
		Linha:   linha,
		Sentido: sentido,
	})

	log.Printf("Nova inscrição: Chat %d, Linha %s, Sentido %s", chatID, linha, sentido)

	var onibus []UltimaFeature
	for _, f := range s.ultimaPosicao.Features {
		if strings.EqualFold(f.Properties.Linha, linha) && f.Properties.Sentido == sentido {
			onibus = append(onibus, f)
		}
	}

	if len(onibus) == 0 {
		msg := tgbotapi.NewMessage(chatID, "As localizações dos ônibus serão enviadas quando algum ônibus em curso for encontrado.\n\nClique no botão abaixo quando não quiser ser mais notificado:")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("stop_%d", chatID)),
			),
		)
		msg.ReplyMarkup = keyboard
		s.bot.Send(msg)
	} else {
		s.EnviarLocalizacoesDosOnibus(chatID, onibus, linha, sentido, true)
	}
}

func (s *BotService) EnviarLocalizacoesDosOnibus(chatID int64, onibus []UltimaFeature, linha, sentido string, primeiraVez bool) {
	max := len(onibus)
	if max > 10 {
		max = 10
	}

	for i := 0; i < max; i++ {
		bus := onibus[i]
		// O Geoserver já retornou WGS84 (lat/lon)
		lon, lat := bus.Geometry.Coordinates[0], bus.Geometry.Coordinates[1]

		s.bot.Send(tgbotapi.NewLocation(chatID, lat, lon))
		
		address, err := s.apiClient.GetAddressInfo(lat, lon, s.version)
		if err != nil {
			address = "Endereço não encontrado"
		}

		s.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("%s :: %s", linha, address)))
		time.Sleep(1 * time.Second)
	}

	if len(onibus) > 10 {
		s.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf(".. e mais %d circulando.", len(onibus)-10)))
	}

	if primeiraVez {
		msg := tgbotapi.NewMessage(chatID, "Você será notificado de 2 em 2 minutos.\n\nClique no botão abaixo quando não quiser ser mais notificado:")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("stop_%d", chatID)),
			),
		)
		msg.ReplyMarkup = keyboard
		s.bot.Send(msg)

		s.mu.Lock()
		for i := range s.userSubscriptions {
			if s.userSubscriptions[i].ChatID == chatID && s.userSubscriptions[i].Linha == linha && s.userSubscriptions[i].Sentido == sentido {
				s.userSubscriptions[i].JaRecebeuPrimeiraMensagem = true
			}
		}
		s.mu.Unlock()
	}
}

func (s *BotService) NotificationLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		s.EnviarNotificacoesPeriodicas()
	}
}

func (s *BotService) EnviarNotificacoesPeriodicas() {
	s.mu.Lock()
	subs := make([]UserSubscription, len(s.userSubscriptions))
	copy(subs, s.userSubscriptions)
	up := s.ultimaPosicao
	s.mu.Unlock()

	if up == nil {
		return
	}

	for _, sub := range subs {
		var onibus []UltimaFeature
		for _, f := range up.Features {
			if strings.EqualFold(f.Properties.Linha, sub.Linha) && f.Properties.Sentido == sub.Sentido {
				onibus = append(onibus, f)
			}
		}

		if len(onibus) > 0 {
			s.EnviarLocalizacoesDosOnibus(sub.ChatID, onibus, sub.Linha, sub.Sentido, false)
		}
	}
}

func (s *BotService) UnsubscribeButtonLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		s.EnviarBotaoDeDesinscrever()
	}
}

func (s *BotService) EnviarBotaoDeDesinscrever() {
	s.mu.Lock()
	chatIDs := make(map[int64]bool)
	for _, sub := range s.userSubscriptions {
		chatIDs[sub.ChatID] = true
	}
	s.mu.Unlock()

	for chatID := range chatIDs {
		msg := tgbotapi.NewMessage(chatID, "Clique no botão abaixo para parar de receber notificações de todas as linhas:")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Parar tudo", fmt.Sprintf("stop_%d", chatID)),
			),
		)
		msg.ReplyMarkup = keyboard
		s.bot.Send(msg)
	}
}

func (s *BotService) DataUpdateLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		hasSubs := len(s.userSubscriptions) > 0
		s.mu.Unlock()

		if hasSubs {
			if err := s.UpdateData(); err != nil {
				log.Printf("Erro ao atualizar dados da frota: %v", err)
			}
		}
	}
}
