package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type APIClient struct {
	httpClient      *http.Client
	geoCache        sync.Map
	mu              sync.Mutex
	lastRequestTime time.Time
	geoapifyKey     string
}

func NewAPIClient(geoapifyKey string) *APIClient {
	return &APIClient{
		httpClient:  &http.Client{},
		geoapifyKey: geoapifyKey,
	}
}

func (c *APIClient) GetLinhasDeOnibus() (*domain.UltimaPosicao, error) {
	url := "https://geoserver.semob.df.gov.br/geoserver/semob/ows?service=WFS&version=1.0.0&request=GetFeature&typeName=semob%3Aultima_posicao&outputFormat=application%2Fjson"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição de linhas: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado (linhas): %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("esperava application/json, recebeu %s", contentType)
	}

	var data domain.UltimaPosicao
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON de linhas: %v", err)
	}

	return &data, nil
}

func (c *APIClient) GetUltimaPosicaoFrota() (*domain.UltimaPosicao, error) {
	url := "https://geoserver.semob.df.gov.br/geoserver/semob/ows?service=WFS&version=1.0.0&request=GetFeature&typeName=semob%3Aultima_posicao&outputFormat=application%2Fjson"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição de frota: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado (frota): %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("esperava application/json, recebeu %s", contentType)
	}

	var data domain.UltimaPosicao
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON de frota: %v", err)
	}

	return &data, nil
}

func (c *APIClient) GetAddressInfo(lat, lon float64, version string) (string, error) {
	// Chave do cache: precisão de 4 casas decimais (~11 metros)
	cacheKey := fmt.Sprintf("%.4f,%.4f", lat, lon)
	if val, ok := c.geoCache.Load(cacheKey); ok {
		log.Printf("[DEBUG] [GEO] Cache HIT para %s", cacheKey)
		return val.(string), nil
	}

	// Rate limiting: apenas UMA requisição por vez à rede, com intervalo
	c.mu.Lock()
	defer c.mu.Unlock()

	// Recomeçar a verificação de cache após o lock, pois outra goroutine pode ter preenchido
	if val, ok := c.geoCache.Load(cacheKey); ok {
		log.Printf("[DEBUG] [GEO] Cache HIT (after lock) para %s", cacheKey)
		return val.(string), nil
	}

	elapsedSinceLast := time.Since(c.lastRequestTime)
	if elapsedSinceLast < 200*time.Millisecond {
		waitTime := 200*time.Millisecond - elapsedSinceLast
		log.Printf("[DEBUG] [GEO] Rate limiting em ação: esperando %v...", waitTime)
		time.Sleep(waitTime)
	}

	log.Printf("[DEBUG] [GEO] Cache MISS. Consultando Geoapify para %.4f, %.4f...", lat, lon)
	start := time.Now()
	url := fmt.Sprintf(
		"https://api.geoapify.com/v1/geocode/reverse?lat=%f&lon=%f&format=json&apiKey=%s",
		lat, lon, c.geoapifyKey,
	)

	resp, err := c.httpClient.Get(url)
	c.lastRequestTime = time.Now()
	if err != nil {
		log.Printf("[ERROR] [GEO] Falha na consulta Geoapify: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] [GEO] Status inesperado do Geoapify: %d", resp.StatusCode)
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data domain.GeoapifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	address := "Endereço não disponível"
	if len(data.Results) > 0 && data.Results[0].Formatted != "" {
		address = data.Results[0].Formatted
	}

	log.Printf("[INFO] [GEO] Endereço obtido em %v: %s", time.Since(start), address)

	// Salva no cache
	c.geoCache.Store(cacheKey, address)

	return address, nil
}
