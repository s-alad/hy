package models

// Ticker represents market ticker data from Hyperliquid
type Ticker struct {
	Symbol    string  `json:"symbol"`
	LastPrice float64 `json:"lastPrice"`
	BidPrice  float64 `json:"bidPrice"`
	AskPrice  float64 `json:"askPrice"`
	Volume24h float64 `json:"volume24h"`
	Change24h float64 `json:"change24h"`
}

// MarketData represents comprehensive market data
type MarketData struct {
	Ticker *Ticker
	Error  error
}
