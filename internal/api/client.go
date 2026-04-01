package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type APIClient struct {
	httpClient *http.Client
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

func (c *APIClient) GetParadasDeOnibus() (*domain.ParadasDeOnibus, error) {
	url := "https://geoserver.semob.df.gov.br/geoserver/semob/ows?service=WFS&version=1.0.0&request=GetFeature&typeName=semob:Paradas%20de%20onibus&outputFormat=application/json&propertyName=geo_ponto_rede_pto,parada,descricao,situacao&srsName=EPSG:4326"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição de paradas: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado (paradas): %d", resp.StatusCode)
	}

	var data domain.ParadasDeOnibus
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON de paradas: %v", err)
	}

	return &data, nil
}
