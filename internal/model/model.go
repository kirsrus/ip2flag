// Package model содержит структуры данных приложения.
package model

// GeoIP представляет собой ответ сервиса ipinfo.io
// https://ipinfo.io/missingauth
type GeoIP struct {
	IP       string `json:"ip"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
	Readme   string `json:"readme"`
}
