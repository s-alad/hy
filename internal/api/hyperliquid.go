package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hy/internal/models"
)

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) GetTicker(symbol string) (*models.Ticker, error) {
	body, _ := json.Marshal(map[string]string{"type": "allMids"})
	req, _ := http.NewRequest("POST", "https://api.hyperliquid.xyz/info", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	priceStr, ok := data[symbol]
	if !ok {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	var price float64
	if _, err := fmt.Sscanf(priceStr, "%f", &price); err != nil {
		return nil, err
	}

	return &models.Ticker{
		Symbol:    symbol,
		LastPrice: price,
		BidPrice:  price,
		AskPrice:  price,
	}, nil
}
