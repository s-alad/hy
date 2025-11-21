package models

// Ticker represents market ticker data from Hyperliquid
type Ticker struct {
	Symbol       string  `json:"symbol"`
	LastPrice    float64 `json:"lastPrice"`
	BidPrice     float64 `json:"bidPrice"`
	AskPrice     float64 `json:"askPrice"`
	Volume24h    float64 `json:"volume24h"`
	Change24h    float64 `json:"change24h"`
	FundingRate  float64 `json:"fundingRate"`
	OpenInterest float64 `json:"openInterest"`
}

// Candle represents a single OHLC candle
type Candle struct {
	Time   int64   `json:"t"` // Start time in milliseconds
	Open   float64 `json:"o"`
	High   float64 `json:"h"`
	Low    float64 `json:"l"`
	Close  float64 `json:"c"`
	Volume float64 `json:"v"`
}

// MarketData represents comprehensive market data
type MarketData struct {
	Ticker  *Ticker
	Candles []Candle
	Error   error
}
