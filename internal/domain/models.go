package domain

import "time"

type RegisteredUser struct {
	ChatID   int64     `json:"chat_id"`
	Username string    `json:"username"`
	JoinedAt time.Time `json:"joined_at"`
}

type PointGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type LineStringGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
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
	Geometry     PointGeometry  `json:"geometry"`
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
	Type       string             `json:"type"`
	ID         string             `json:"id"`
	Geometry   LineStringGeometry `json:"geometry"`
	Properties LinhasProperty     `json:"properties"`
}

type LinhasProperty struct {
	ID    int    `json:"id_linha"`
	Linha string `json:"lin_sentido"`
}

type ParadasDeOnibus struct {
	Type     string          `json:"type"`
	Features []ParadaFeature `json:"features"`
}

type ParadaFeature struct {
	Geometry   PointGeometry  `json:"geometry"`
	Properties ParadaProperty `json:"properties"`
}

type ParadaProperty struct {
	Parada    string `json:"parada"`
	Descricao string `json:"descricao"`
	Situacao  string `json:"situacao"`
}

type UserSubscription struct {
	ChatID                    int64
	Linha                     string
	Sentido                   string
	JaRecebeuPrimeiraMensagem bool
	SubscribedAt              time.Time
	ExpiryWarned              bool
}

type BusGroup struct {
	Name   string
	Lines  []string
}
