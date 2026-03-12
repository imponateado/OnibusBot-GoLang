package main

type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type UltimaPosicao struct {
	Type           string          `json:"type"`
	Features       []UltimaFeature `json:"features"`
	TotalFeatures  int             `json:"totalFeatures"`
	NumberMatched  int             `json:"numberMatched"`
	NumberReturned int             `json:"numberReturned"`
	TimeStamp      string          `json:"timeStamp"`
}

type UltimaFeature struct {
	Type         string         `json:"type"`
	ID           string         `json:"id"`
	Geometry     Geometry       `json:"geometry"`
	GeometryName string         `json:"geometry_name"`
	Properties   UltimaProperty `json:"properties"`
}

type UltimaProperty struct {
	IdOperadora  int     `json:"id_operadora"`
	Prefixo      string  `json:"prefixo"`
	DataLocal    string  `json:"datalocal"`
	Velocidade   string  `json:"velocidade"`
	Linha        string  `json:"cd_linha"`
	Direcao      string  `json:"direcao"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	DataRegistro string  `json:"dataregistro"`
	IMEI         string  `json:"imei"`
	Sentido      string  `json:"sentido"`
}

type LinhasDeOnibus struct {
	Type     string          `json:"type"`
	Features []LinhasFeature `json:"features"`
}

type LinhasFeature struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Geometry   Geometry       `json:"geometry"`
	Properties LinhasProperty `json:"properties"`
}

type LinhasProperty struct {
	Linha        string `json:"cd_linha"`
	Nome         string `json:"no_linha"`
	IdOperadora  int    `json:"id_operadora"`
	Cor          string `json:"cor_linha"`
}

type NominatimResponse struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		Road          string `json:"road"`
		Neighbourhood string `json:"neighbourhood"`
		Suburb        string `json:"suburb"`
		City          string `json:"city"`
		State         string `json:"state"`
		Postcode      string `json:"postcode"`
		Country       string `json:"country"`
		CountryCode   string `json:"country_code"`
	} `json:"address"`
}

type UserSubscription struct {
	ChatID                   int64
	Linha                    string
	Sentido                  string
	JaRecebeuPrimeiraMensagem bool
}
