package service

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type BusProvider interface {
	GetLinhasDeOnibus() (*domain.UltimaPosicao, error)
	GetUltimaPosicaoFrota() (*domain.UltimaPosicao, error)
	GetAddressInfo(lat, lon float64, version string) (string, error)
}

type Notifier interface {
	NotifyLocation(chatID int64, lat, lon float64, text string)
	NotifyMessage(chatID int64, text string, keyboard interface{})
}

type BusService struct {
	provider          BusProvider
	subsRepo          domain.SubscriptionRepository
	groupsRepo        domain.GroupRepository
	userSubscriptions []domain.UserSubscription
	busGroups         []domain.BusGroup
	linhasDisponiveis []string
	ultimaPosicao     *domain.UltimaPosicao
	mu                sync.Mutex
	version           string
	notifier          Notifier
}

func NewBusService(version string, provider BusProvider, subsRepo domain.SubscriptionRepository, groupsRepo domain.GroupRepository) *BusService {
	s := &BusService{
		version:    version,
		provider:   provider,
		subsRepo:   subsRepo,
		groupsRepo: groupsRepo,
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
}

func (s *BusService) StartLoops() {
	if err := s.UpdateData(); err != nil {
		log.Printf("Erro ao carregar dados iniciais: %v", err)
	}

	go s.notificationLoop()
	go s.unsubscribeButtonLoop()
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
		ChatID:  chatID,
		Linha:   linha,
		Sentido: sentido,
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

func (s *BusService) ToggleLowMode(chatID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	var currentMode bool
	for i := range s.userSubscriptions {
		if s.userSubscriptions[i].ChatID == chatID {
			s.userSubscriptions[i].LowDataMode = !s.userSubscriptions[i].LowDataMode
			currentMode = s.userSubscriptions[i].LowDataMode
			found = true
		}
	}
	if !found {
		return false, fmt.Errorf("sem inscrições")
	}
	return currentMode, s.subsRepo.Save(s.userSubscriptions)
}

func (s *BusService) GetBusStatus(chatID int64, linha, sentido string) ([]domain.UltimaFeature, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	lowDataMode := false
	for _, sub := range s.userSubscriptions {
		if sub.ChatID == chatID {
			lowDataMode = sub.LowDataMode
			break
		}
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
	return s.provider.GetAddressInfo(lat, lon, s.version)
}

// Loops internos
func (s *BusService) notificationLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		subs := make([]domain.UserSubscription, len(s.userSubscriptions))
		copy(subs, s.userSubscriptions)
		s.mu.Unlock()

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

	if lowMode {
		var text strings.Builder
		text.WriteString(fmt.Sprintf("🚌 *Linha %s (%s)*\n\n", linha, sentido))
		for _, key := range clusterKeys {
			c := clusters[key]
			address, _ := s.GetAddress(c.Lat, c.Lon)
			text.WriteString(fmt.Sprintf("📍 %s\n   (Ônibus: %s)\n", address, strings.Join(c.Prefixos, ", ")))
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
