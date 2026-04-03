package usom

type Response struct {
	TotalCount int     `json:"totalCount"`
	Count      int     `json:"count"`
	Models     []Model `json:"models"`
	Page       int     `json:"page"`
	PageCount  int     `json:"pageCount"`
}

type Model struct {
	ID               int    `json:"id"`
	URL              string `json:"url"`
	Type             string `json:"type"`
	Desc             string `json:"desc"`
	Source           string `json:"source"`
	Date             string `json:"date"`
	CriticalityLevel int    `json:"criticality_level"`
	ConnectionType   string `json:"connectiontype"`
}

type Config struct {
	AddressType      string
	CriticalityLevel int
	DateGTE          string
	DateLTE          string
	Source           string
	Desc             string
	ConnectionType   string
	PerPage          int
}

var AllModels []Model
