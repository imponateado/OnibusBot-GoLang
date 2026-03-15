package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	busGroups         []BusGroup
	version           string
	dbPath            string
}

func NewBotService(token string, version string) (*BotService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	service := &BotService{
		bot:       bot,
		apiClient: NewAPIClient(),
		version:   version,
		dbPath:    "subscriptions.json",
	}

	if err := service.LoadSubscriptions(); err != nil {
		log.Printf("Aviso: Não foi possível carregar inscrições: %v", err)
	}

	if err := service.LoadGroups(); err != nil {
		log.Printf("Aviso: Não foi possível carregar grupos de ônibus: %v", err)
	}

	return service, nil
}

func (s *BotService) LoadGroups() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := "groups.csv"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newGroups []BusGroup
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";")
		if len(parts) >= 2 {
			name := strings.ToUpper(strings.TrimSpace(parts[0]))
			busLines := strings.Split(parts[1], ",")
			for i := range busLines {
				busLines[i] = strings.TrimSpace(busLines[i])
			}
			newGroups = append(newGroups, BusGroup{
				Name:  name,
				Lines: busLines,
			})
		}
	}
	s.busGroups = newGroups
	log.Printf("Carregados %d grupos de ônibus do CSV.", len(s.busGroups))
	return nil
}

func (s *BotService) SaveSubscriptions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.userSubscriptions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.dbPath, data, 0644)
}

func (s *BotService) LoadSubscriptions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.dbPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.dbPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.userSubscriptions)
}

func (s *BotService) Start() {
	log.Printf("Iniciando OnibusBot-Go versão %s...", s.version)

	// Carregar dados iniciais
	if err := s.UpdateData(); err != nil {
		log.Printf("Erro ao carregar dados iniciais: %v", err)
	}

	// Iniciar tickers
	go s.NotificationLoop()
	go s.UnsubscribeButtonLoop()
	go s.DataUpdateLoop()

	for {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60

		updates := s.bot.GetUpdatesChan(u)

		for update := range updates {
			go func(u tgbotapi.Update) {
				if u.Message != nil {
					s.HandleMessage(u.Message)
				} else if u.CallbackQuery != nil {
					s.HandleCallback(u.CallbackQuery)
				}
			}(update)
		}

		log.Printf("Canal de updates fechado ou erro na conexão. Tentando reconectar em 5 segundos...")
		time.Sleep(5 * time.Second)

		// Tenta recriar o bot para garantir uma sessão limpa
		if newBot, err := tgbotapi.NewBotAPI(s.bot.Token); err == nil {
			s.mu.Lock()
			s.bot = newBot
			s.mu.Unlock()
			log.Printf("Reconexão bem-sucedida com o Telegram.")
		} else {
			log.Printf("Falha ao tentar reconectar: %v", err)
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

	if msg.Text == "/lowmode" {
		s.mu.Lock()
		count := 0
		var currentMode bool
		for i := range s.userSubscriptions {
			if s.userSubscriptions[i].ChatID == msg.Chat.ID {
				s.userSubscriptions[i].LowDataMode = !s.userSubscriptions[i].LowDataMode
				currentMode = s.userSubscriptions[i].LowDataMode
				count++
			}
		}
		s.mu.Unlock()

		if count == 0 {
			s.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Você ainda não tem linhas inscritas. Inscreva uma linha primeiro para alternar o modo."))
			return
		}

		s.SaveSubscriptions()

		status := "Ativado (Apenas texto) 🐌"
		if !currentMode {
			status = "Desativado (Mapas habilitados) 🚀"
		}
		s.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Modo Econômico: %s", status)))
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

		msgStart := "Olá! Digite o número da linha que você deseja acompanhar (ex: 2210) ou o nome de um grupo (ex: EPNB):\n\n"
		msgStart += "💡 *Dica:* Se sua internet estiver lenta (32kbps), use o comando /lowmode para economizar dados."
		
		reply := tgbotapi.NewMessage(msg.Chat.ID, msgStart)
		reply.ParseMode = "Markdown"
		s.bot.Send(reply)
		return
	}

	userInput := strings.ToUpper(strings.TrimSpace(msg.Text))

	// Verificar se é um grupo
	var groupLines []string
	s.mu.Lock()
	for _, g := range s.busGroups {
		if g.Name == userInput {
			groupLines = g.Lines
			break
		}
	}
	s.mu.Unlock()

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
		s.bot.Send(msgConfig)
		return
	}

	// Se for um número de linha (ou busca parcial)
	linhaInput := userInput
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

	if strings.HasPrefix(callbackData, "gsentido_") {
		parts := strings.Split(callbackData, "_")
		if len(parts) >= 3 {
			groupName := parts[1]
			sentido := parts[2]
			s.ProcessGroupSubscription(chatID, groupName, sentido)
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

func (s *BotService) ProcessGroupSubscription(chatID int64, groupName, sentido string) {
	s.mu.Lock()
	var lines []string
	for _, g := range s.busGroups {
		if g.Name == groupName {
			lines = g.Lines
			break
		}
	}
	s.mu.Unlock()

	if len(lines) == 0 {
		return
	}

	s.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ Iniciando rastreamento de %d linhas do grupo %s...", len(lines), groupName)))

	for _, linha := range lines {
		s.ProcessAndSendBusStatusExt(chatID, linha, sentido, true)
	}

	msg := tgbotapi.NewMessage(chatID, "✅ Inscrição em lote concluída!\n\nVocê será notificado a cada 2 minutos sobre todas essas linhas.\nPara parar tudo, use o botão abaixo:")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Parar Todas as Notificações", fmt.Sprintf("stop_%d", chatID)),
		),
	)
	msg.ReplyMarkup = keyboard
	s.bot.Send(msg)
}

func (s *BotService) ProcessAndSendBusStatus(chatID int64, linha, sentido string) {
	s.ProcessAndSendBusStatusExt(chatID, linha, sentido, false)
}

func (s *BotService) ProcessAndSendBusStatusExt(chatID int64, linha, sentido string, quiet bool) {
	s.mu.Lock()

	if s.ultimaPosicao == nil {
		s.mu.Unlock()
		if !quiet {
			s.bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Desculpe, o serviço está temporariamente indisponível."))
		}
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
		s.mu.Unlock()
		if !quiet {
			s.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Já foi encontrado uma inscrição pra linha %s!", linha)))
		}
		return
	}

	if count >= 10 {
		s.mu.Unlock()
		if !quiet {
			msg := tgbotapi.NewMessage(chatID, "🚫 Você atingiu o limite máximo de 10 linhas monitoradas!\n\nPara adicionar uma nova linha, primeiro cancele algumas das existentes.")
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("❌ Cancelar todas as inscrições.", fmt.Sprintf("stop_%d", chatID)),
				),
			)
			msg.ReplyMarkup = keyboard
			s.bot.Send(msg)
		}
		return
	}

	s.userSubscriptions = append(s.userSubscriptions, UserSubscription{
		ChatID:  chatID,
		Linha:   linha,
		Sentido: sentido,
	})
	s.mu.Unlock()
	s.SaveSubscriptions()
	s.mu.Lock()

	log.Printf("Nova inscrição: Chat %d, Linha %s, Sentido %s", chatID, linha, sentido)

	var onibus []UltimaFeature
	for _, f := range s.ultimaPosicao.Features {
		if strings.EqualFold(f.Properties.Linha, linha) && f.Properties.Sentido == sentido {
			onibus = append(onibus, f)
		}
	}
	s.mu.Unlock()

	if len(onibus) == 0 {
		if !quiet {
			msg := tgbotapi.NewMessage(chatID, "As localizações dos ônibus serão enviadas quando algum ônibus em curso for encontrado.\n\nClique no botão abaixo quando não quiser ser mais notificado:")
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("stop_%d", chatID)),
				),
			)
			msg.ReplyMarkup = keyboard
			s.bot.Send(msg)
		}
	} else {
		s.EnviarLocalizacoesDosOnibus(chatID, onibus, linha, sentido, !quiet)
	}
}

func (s *BotService) EnviarLocalizacoesDosOnibus(chatID int64, onibus []UltimaFeature, linha, sentido string, primeiraVez bool) {
	s.mu.Lock()
	lowDataMode := false
	for _, sub := range s.userSubscriptions {
		if sub.ChatID == chatID {
			lowDataMode = sub.LowDataMode
			break
		}
	}
	s.mu.Unlock()

	max := len(onibus)
	if max > 10 {
		max = 10
	}

	if lowDataMode {
		// Modo Econômico: UMA única mensagem com todos os endereços
		var text strings.Builder
		text.WriteString(fmt.Sprintf("🚌 *Linha %s (%s)*\n\n", linha, sentido))
		
		for i := 0; i < max; i++ {
			bus := onibus[i]
			lon, lat := bus.Geometry.Coordinates[0], bus.Geometry.Coordinates[1]
			
			address, err := s.apiClient.GetAddressInfo(lat, lon, s.version)
			if err != nil {
				address = "Endereço não disponível"
			}
			
			text.WriteString(fmt.Sprintf("📍 %s\n", address))
		}

		if len(onibus) > 10 {
			text.WriteString(fmt.Sprintf("\n.. e mais %d circulando.", len(onibus)-10))
		}

		msg := tgbotapi.NewMessage(chatID, text.String())
		msg.ParseMode = "Markdown"
		s.bot.Send(msg)
	} else {
		// Modo Padrão: Comportamento original (múltiplas localizações e mensagens)
		for i := 0; i < max; i++ {
			bus := onibus[i]
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
	}

	if primeiraVez {
		text := "Você será notificado de 2 em 2 minutos.\n\n"
		if !lowDataMode {
			text += "💡 *Dica:* Use /lowmode se a internet estiver lenta.\n\n"
		}
		text += "Clique no botão abaixo para parar:"
		
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Parar Notificações", fmt.Sprintf("stop_%d", chatID)),
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
		s.SaveSubscriptions()
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
