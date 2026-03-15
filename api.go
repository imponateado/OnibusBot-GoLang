package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type APIClient struct {
	httpClient *http.Client
	geoCache   sync.Map
}

func NewAPIClient() *APIClient {
	return &APIClient{
		httpClient: &http.Client{},
	}
}

func (c *APIClient) GetLinhasDeOnibus() (*UltimaPosicao, error) {
	// Usamos o mesmo endpoint da frota para pegar as linhas que estão ATIVAS no momento (que têm ônibus circulando)
	// ou poderíamos usar "Horários das Linhas", mas a frota é mais garantido de ter cd_linha
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

	var data UltimaPosicao
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON de linhas: %v", err)
	}

	return &data, nil
}

func (c *APIClient) GetUltimaPosicaoFrota() (*UltimaPosicao, error) {
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

	var data UltimaPosicao
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON de frota: %v", err)
	}

	return &data, nil
}

func (c *APIClient) GetAddressInfo(lat, lon float64, version string) (string, error) {
	// Chave do cache: precisão de 4 casas decimais (~11 metros)
	cacheKey := fmt.Sprintf("%.4f,%.4f", lat, lon)
	if val, ok := c.geoCache.Load(cacheKey); ok {
		return val.(string), nil
	}

	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f", lat, lon)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", fmt.Sprintf("OnibusBot-Go/%s (leoteodoro0@hotmail.com)", version))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data NominatimResponse
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

	// Salva no cache
	c.geoCache.Store(cacheKey, address)

	return address, nil
}
