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
}

func NewAPIClient() *APIClient {
	return &APIClient{
		httpClient: &http.Client{},
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
		log.Printf("[DEBUG] [OSM] Cache HIT para %s", cacheKey)
		return val.(string), nil
	}

	// Rate limiting: apenas UMA requisição por vez à rede, com intervalo
	c.mu.Lock()
	defer c.mu.Unlock()

	// Recomeçar a verificação de cache após o lock, pois outra goroutine pode ter preenchido
	if val, ok := c.geoCache.Load(cacheKey); ok {
		log.Printf("[DEBUG] [OSM] Cache HIT (after lock) para %s", cacheKey)
		return val.(string), nil
	}

	elapsedSinceLast := time.Since(c.lastRequestTime)
	if elapsedSinceLast < 1100*time.Millisecond {
		waitTime := 1100*time.Millisecond - elapsedSinceLast
		log.Printf("[DEBUG] [OSM] Rate limiting em ação: esperando %v...", waitTime)
		time.Sleep(waitTime)
	}

	log.Printf("[DEBUG] [OSM] Cache MISS. Consultando Nominatim para %.4f, %.4f...", lat, lon)
	start := time.Now()
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f", lat, lon)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", fmt.Sprintf("OnibusBot-Go/%s (leoteodoro0@hotmail.com)", version))

	resp, err := c.httpClient.Do(req)
	c.lastRequestTime = time.Now()
	if err != nil {
		log.Printf("[ERROR] [OSM] Falha na consulta Nominatim: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] [OSM] Status inesperado do Nominatim: %d", resp.StatusCode)
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data domain.NominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	var parts []string
	if data.Address.Road != "" {
		parts = append(parts, data.Address.Road)
	}
	if data.Address.Neighbourhood != "" {
		parts = append(parts, data.Address.Neighbourhood)
	}
	if data.Address.Suburb != "" {
		parts = append(parts, data.Address.Suburb)
	}

	address := strings.Join(parts, " ")
	if address == "" {
		address = "Endereço não disponível"
	}

	log.Printf("[INFO] [OSM] Endereço obtido em %v: %s", time.Since(start), address)

	// Salva no cache
	c.geoCache.Store(cacheKey, address)

	return address, nil
}
