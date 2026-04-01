package service

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/leoteodoro/onibus-bot-go/internal/domain"
	"github.com/leoteodoro/onibus-bot-go/pkg/utils"
)

type BusProvider interface {
	GetLinhasDeOnibus() (*domain.UltimaPosicao, error)
	GetParadasDeOnibus() (*domain.ParadasDeOnibus, error)
}

type Notifier interface {
	NotifyLocation(chatID int64, lat, lon float64, text string)
	NotifyMessage(chatID int64, text string, keyboard interface{})
}

type BusService struct {
	provider          BusProvider
	subsRepo          domain.SubscriptionRepository
	groupsRepo        domain.GroupRepository
	prefsRepo         domain.UserPrefsRepository
	userSubscriptions []domain.UserSubscription
	busGroups         []domain.BusGroup
	lowModeUsers      map[int64]bool
	linhasDisponiveis []string
	ultimaPosicao     *domain.UltimaPosicao
	paradas           []domain.ParadaFeature
	mu                sync.Mutex
	version           string
	notifier          Notifier
}

func NewBusService(version string, provider BusProvider, subsRepo domain.SubscriptionRepository, groupsRepo domain.GroupRepository, prefsRepo domain.UserPrefsRepository) *BusService {
	s := &BusService{
		version:      version,
		provider:     provider,
		subsRepo:     subsRepo,
		groupsRepo:   groupsRepo,
		prefsRepo:    prefsRepo,
		lowModeUsers: make(map[int64]bool),
	}
	s.LoadData()
	return s
}

func (s *BusService) SetNotifier(n Notifier) {
	s.notifier = n
}

func (s *BusService) LoadData() {
	subs, err := s.subsRepo.Load()
	if err != nil {
		log.Printf("Erro ao carregar inscrições: %v", err)
	}
	s.userSubscriptions = subs

	groups, err := s.groupsRepo.Load()
	if err != nil {
		log.Printf("Erro ao carregar grupos: %v", err)
	}
	s.busGroups = groups

	lowMode, err := s.prefsRepo.LoadLowMode()
	if err != nil {
		log.Printf("Erro ao carregar preferências: %v", err)
	} else {
		s.lowModeUsers = lowMode
	}

	paradas, err := s.provider.GetParadasDeOnibus()
	if err != nil {
		log.Printf("Erro ao carregar paradas: %v", err)
	} else {
		var ativas []domain.ParadaFeature
		for _, p := range paradas.Features {
			if p.Properties.Situacao == "ATIVA" && len(p.Geometry.Coordinates) >= 2 {
				ativas = append(ativas, p)
			}
		}
		s.paradas = ativas
		log.Printf("[INFO] %d paradas ativas carregadas", len(ativas))
	}
}

func (s *BusService) StartLoops() {
	if err := s.UpdateData(); err != nil {
		log.Printf("Erro ao carregar dados iniciais: %v", err)
	}

	go s.notificationLoop()
	go s.unsubscribeButtonLoop()
	go s.expiryCheckLoop()
	go s.dataUpdateLoop()
}

func (s *BusService) UpdateData() error {
	posicao, err := s.provider.GetLinhasDeOnibus()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.linhasDisponiveis = nil
	seen := make(map[string]bool)
	var cleanFeatures []domain.UltimaFeature
	for _, f := range posicao.Features {
		l := f.Properties.Linha
		if l != "" {
			if !seen[l] {
				s.linhasDisponiveis = append(s.linhasDisponiveis, l)
				seen[l] = true
			}
			cleanFeatures = append(cleanFeatures, f)
		}
	}
	posicao.Features = cleanFeatures
	s.ultimaPosicao = posicao

	return nil
}

func (s *BusService) GetLinhasDisponiveis(query string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []string
	query = strings.ToUpper(query)
	for _, l := range s.linhasDisponiveis {
		if strings.Contains(l, query) {
			res = append(res, l)
		}
	}
	return res
}

func (s *BusService) IsLinhaValida(linha string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.linhasDisponiveis {
		if l == linha {
			return true
		}
	}
	return false
}

func (s *BusService) GetActiveDirections(linha string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ultimaPosicao == nil {
		return nil
	}

	seen := make(map[string]bool)
	var res []string
	for _, f := range s.ultimaPosicao.Features {
		if strings.EqualFold(f.Properties.Linha, linha) {
			snt := f.Properties.Sentido
			if snt != "" && !seen[snt] {
				seen[snt] = true
				res = append(res, snt)
			}
		}
	}
	return res
}

func (s *BusService) GetGroupsList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []string
	for _, g := range s.busGroups {
		res = append(res, g.Name)
	}
	return res
}

func (s *BusService) GetGroup(name string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.ToUpper(name)
	for _, g := range s.busGroups {
		if g.Name == name {
			return g.Lines
		}
	}
	return nil
}

func (s *BusService) Subscribe(chatID int64, linha, sentido string) error {
	s.mu.Lock()
	count := 0
	for _, sub := range s.userSubscriptions {
		if sub.ChatID == chatID {
			count++
			if sub.Linha == linha && sub.Sentido == sentido {
				s.mu.Unlock()
				return fmt.Errorf("já inscrito")
			}
		}
	}

	if count >= 10 {
		s.mu.Unlock()
		return fmt.Errorf("limite atingido")
	}

	s.userSubscriptions = append(s.userSubscriptions, domain.UserSubscription{
		ChatID:       chatID,
		Linha:        linha,
		Sentido:      sentido,
		SubscribedAt: time.Now(),
	})
	subs := s.userSubscriptions
	s.mu.Unlock()

	return s.subsRepo.Save(subs)
}

func (s *BusService) UnsubscribeAll(chatID int64) (int, error) {
	s.mu.Lock()
	var newSubs []domain.UserSubscription
	removidos := 0
	for _, sub := range s.userSubscriptions {
		if sub.ChatID == chatID {
			removidos++
		} else {
			newSubs = append(newSubs, sub)
		}
	}
	s.userSubscriptions = newSubs
	subs := s.userSubscriptions
	s.mu.Unlock()

	return removidos, s.subsRepo.Save(subs)
}

func (s *BusService) ToggleLowMode(chatID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := true
	if v, ok := s.lowModeUsers[chatID]; ok {
		current = v
	}
	s.lowModeUsers[chatID] = !current
	s.prefsRepo.SaveLowMode(s.lowModeUsers)
	return !current
}

func (s *BusService) GetBusStatus(chatID int64, linha, sentido string) ([]domain.UltimaFeature, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lowDataMode := true
	if v, ok := s.lowModeUsers[chatID]; ok {
		lowDataMode = v
	}

	if s.ultimaPosicao == nil {
		return nil, lowDataMode
	}

	var onibus []domain.UltimaFeature
	for _, f := range s.ultimaPosicao.Features {
		if strings.EqualFold(f.Properties.Linha, linha) && f.Properties.Sentido == sentido {
			onibus = append(onibus, f)
		}
	}
	return onibus, lowDataMode
}

func (s *BusService) GetAddress(lat, lon float64) (string, error) {
	s.mu.Lock()
	paradas := s.paradas
	s.mu.Unlock()

	if len(paradas) == 0 {
		return "Parada não disponível", nil
	}

	minDist := math.MaxFloat64
	var nearest domain.ParadaFeature
	for _, p := range paradas {
		// Coordinates: [lon, lat] (GeoJSON format)
		pLon := p.Geometry.Coordinates[0]
		pLat := p.Geometry.Coordinates[1]
		dist := utils.Haversine(lat, lon, pLat, pLon)
		if dist < minDist {
			minDist = dist
			nearest = p
		}
	}

	return nearest.Properties.Descricao, nil
}

// Loops internos
func (s *BusService) notificationLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		subs := make([]domain.UserSubscription, len(s.userSubscriptions))
		copy(subs, s.userSubscriptions)
		s.mu.Unlock()

		if len(subs) > 0 {
			log.Printf("[INFO] Iniciando ciclo de notificação para %d inscrições...", len(subs))
		}

		for _, sub := range subs {
			onibus, lowMode := s.GetBusStatus(sub.ChatID, sub.Linha, sub.Sentido)
			if len(onibus) > 0 {
				s.NotifyBuses(sub.ChatID, onibus, sub.Linha, sub.Sentido, lowMode)
			}
		}
	}
}

func (s *BusService) NotifyBuses(chatID int64, onibus []domain.UltimaFeature, linha, sentido string, lowMode bool) {
	if s.notifier == nil {
		return
	}

	max := len(onibus)
	if max > 10 {
		max = 10
	}

	type ClusterBus struct {
		Lat      float64
		Lon      float64
		Prefixos []string
	}
	clusters := make(map[string]*ClusterBus)
	var clusterKeys []string

	for i := 0; i < max; i++ {
		bus := onibus[i]
		lat, lon := bus.Geometry.Coordinates[1], bus.Geometry.Coordinates[0]
		key := fmt.Sprintf("%.4f,%.4f", lat, lon)
		if c, ok := clusters[key]; ok {
			c.Prefixos = append(c.Prefixos, bus.Properties.Prefixo)
		} else {
			clusters[key] = &ClusterBus{
				Lat:      lat,
				Lon:      lon,
				Prefixos: []string{bus.Properties.Prefixo},
			}
			clusterKeys = append(clusterKeys, key)
		}
	}

	if len(onibus) > len(clusterKeys) {
		log.Printf("[DEBUG] Clustering para chat %d: %d ônibus reduzidos para %d localizações", chatID, len(onibus), len(clusterKeys))
	}

	if lowMode {
		var text strings.Builder
		text.WriteString(fmt.Sprintf("🚌 *Linha %s (%s)*\n\n", linha, sentido))
		for _, key := range clusterKeys {
			c := clusters[key]
			address, _ := s.GetAddress(c.Lat, c.Lon)
			text.WriteString(fmt.Sprintf("Próximo à parada %s\n   (Ônibus: %s)\n\n", address, strings.Join(c.Prefixos, ", ")))
		}
		if len(onibus) > 10 {
			text.WriteString(fmt.Sprintf("\n.. e mais %d circulando.", len(onibus)-10))
		}
		s.notifier.NotifyMessage(chatID, text.String(), nil)
	} else {
		for _, key := range clusterKeys {
			c := clusters[key]
			address, _ := s.GetAddress(c.Lat, c.Lon)
			msg := fmt.Sprintf("%s (%s) :: %s", linha, strings.Join(c.Prefixos, ", "), address)
			s.notifier.NotifyLocation(chatID, c.Lat, c.Lon, msg)
		}
		if len(onibus) > 10 {
			s.notifier.NotifyMessage(chatID, fmt.Sprintf(".. e mais %d circulando.", len(onibus)-10), nil)
		}
	}
}

func (s *BusService) expiryCheckLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.checkExpiredSubscriptions()
	}
}

func (s *BusService) checkExpiredSubscriptions() {
	if s.notifier == nil {
		return
	}

	now := time.Now()
	s.mu.Lock()

	var toWarn []domain.UserSubscription
	var toRemove []domain.UserSubscription
	var remaining []domain.UserSubscription

	for i := range s.userSubscriptions {
		sub := s.userSubscriptions[i]
		age := now.Sub(sub.SubscribedAt)

		if age >= 2*time.Hour {
			toRemove = append(toRemove, sub)
		} else if age >= 1*time.Hour && !sub.ExpiryWarned {
			s.userSubscriptions[i].ExpiryWarned = true
			toWarn = append(toWarn, s.userSubscriptions[i])
			remaining = append(remaining, s.userSubscriptions[i])
		} else {
			remaining = append(remaining, sub)
		}
	}

	s.userSubscriptions = remaining
	subs := s.userSubscriptions
	s.mu.Unlock()

	if len(toWarn) > 0 || len(toRemove) > 0 {
		s.subsRepo.Save(subs)
	}

	// Avisar quem está há 1 hora
	warnedChats := make(map[int64]bool)
	for _, sub := range toWarn {
		if !warnedChats[sub.ChatID] {
			warnedChats[sub.ChatID] = true
			s.notifier.NotifyMessage(sub.ChatID,
				"⚠️ Você está rastreando ônibus há mais de *1 hora*.\n\nO rastreamento será encerrado automaticamente em mais 1 hora.\nClique abaixo para parar agora:",
				"stop_button")
		}
	}

	// Remover quem está há 2 horas
	removedChats := make(map[int64]int)
	for _, sub := range toRemove {
		removedChats[sub.ChatID]++
	}
	for chatID, count := range removedChats {
		log.Printf("[INFO] Auto-removendo %d inscrições expiradas do chat %d", count, chatID)
		s.notifier.NotifyMessage(chatID,
			fmt.Sprintf("⏰ Rastreamento encerrado automaticamente após 2 horas. (%d linhas removidas)\n\nVocê pode se inscrever novamente a qualquer momento!", count),
			nil)
	}
}

func (s *BusService) unsubscribeButtonLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		chatIDs := make(map[int64]bool)
		for _, sub := range s.userSubscriptions {
			chatIDs[sub.ChatID] = true
		}
		s.mu.Unlock()

		for chatID := range chatIDs {
			if s.notifier != nil {
				s.notifier.NotifyMessage(chatID, "Clique no botão abaixo para parar de receber notificações de todas as linhas:", "stop_button")
			}
		}
	}
}

func (s *BusService) dataUpdateLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		hasSubs := len(s.userSubscriptions) > 0
		s.mu.Unlock()
		if hasSubs {
			s.UpdateData()
		}
	}
}

func (s *BusService) SetJaRecebeuPrimeiraMensagem(chatID int64, linha, sentido string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.userSubscriptions {
		if s.userSubscriptions[i].ChatID == chatID && s.userSubscriptions[i].Linha == linha && s.userSubscriptions[i].Sentido == sentido {
			s.userSubscriptions[i].JaRecebeuPrimeiraMensagem = true
			s.subsRepo.Save(s.userSubscriptions)
			break
		}
	}
}
