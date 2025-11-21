package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"hy/internal/models"
)

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}}
}

type Meta struct {
	Universe []struct {
		Name string `json:"name"`
	} `json:"universe"`
}

type AssetCtx struct {
	DayNtlVlm    string   `json:"dayNtlVlm"`
	PrevDayPx    string   `json:"prevDayPx"`
	MarkPx       string   `json:"markPx"`
	MidPx        *string  `json:"midPx"`
	ImpactPxs    []string `json:"impactPxs"`
	Funding      string   `json:"funding"`
	OpenInterest string   `json:"openInterest"`
}

func (c *Client) GetTicker(symbol string) (*models.Ticker, error) {
	body, _ := json.Marshal(map[string]string{"type": "metaAndAssetCtxs"})
	req, _ := http.NewRequest("POST", "https://api.hyperliquid.xyz/info", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response) < 2 {
		return nil, fmt.Errorf("unexpected response format")
	}

	var meta Meta
	if err := json.Unmarshal(response[0], &meta); err != nil {
		return nil, err
	}

	var assetCtxs []AssetCtx
	if err := json.Unmarshal(response[1], &assetCtxs); err != nil {
		return nil, err
	}

	var index = -1
	for i, u := range meta.Universe {
		if u.Name == symbol {
			index = i
			break
		}
	}

	if index == -1 {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	if index >= len(assetCtxs) {
		return nil, fmt.Errorf("asset context not found for symbol %s", symbol)
	}

	ctx := assetCtxs[index]

	markPx, err := strconv.ParseFloat(ctx.MarkPx, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing markPx: %v", err)
	}

	prevDayPx, err := strconv.ParseFloat(ctx.PrevDayPx, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing prevDayPx: %v", err)
	}

	volume24h, err := strconv.ParseFloat(ctx.DayNtlVlm, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing dayNtlVlm: %v", err)
	}

	change24h := 0.0
	if prevDayPx != 0 {
		change24h = ((markPx - prevDayPx) / prevDayPx) * 100
	}

	bidPrice := markPx
	askPrice := markPx
	if len(ctx.ImpactPxs) >= 2 {
		if bp, err := strconv.ParseFloat(ctx.ImpactPxs[0], 64); err == nil {
			bidPrice = bp
		}
		if ap, err := strconv.ParseFloat(ctx.ImpactPxs[1], 64); err == nil {
			askPrice = ap
		}
	}

	fundingRate, err := strconv.ParseFloat(ctx.Funding, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing funding: %v", err)
	}

	openInterest, err := strconv.ParseFloat(ctx.OpenInterest, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing openInterest: %v", err)
	}

	return &models.Ticker{
		Symbol:       symbol,
		LastPrice:    markPx,
		BidPrice:     bidPrice,
		AskPrice:     askPrice,
		Volume24h:    volume24h,
		Change24h:    change24h,
		FundingRate:  fundingRate,
		OpenInterest: openInterest,
	}, nil
}

type CandleRequest struct {
	Coin      string `json:"coin"`
	Interval  string `json:"interval"`
	StartTime int64  `json:"startTime,omitempty"`
	EndTime   int64  `json:"endTime,omitempty"`
}

type rawCandle struct {
	Time   int64  `json:"t"`
	Open   string `json:"o"`
	High   string `json:"h"`
	Low    string `json:"l"`
	Close  string `json:"c"`
	Volume string `json:"v"`
}

func getDuration(interval string) int64 {
	switch interval {
	case "1m":
		return 60 * 1000
	case "5m":
		return 5 * 60 * 1000
	case "15m":
		return 15 * 60 * 1000
	case "1h":
		return 60 * 60 * 1000
	case "4h":
		return 4 * 60 * 60 * 1000
	case "1d":
		return 24 * 60 * 60 * 1000
	default:
		return 60 * 60 * 1000 // Default to 1h
	}
}

func (c *Client) GetCandles(symbol string, interval string) ([]models.Candle, error) {
	endTime := time.Now().UnixMilli()
	duration := getDuration(interval)
	// Fetch 100 candles
	startTime := endTime - (100 * duration)

	reqBody := map[string]interface{}{
		"type": "candleSnapshot",
		"req": CandleRequest{
			Coin:      symbol,
			Interval:  interval,
			StartTime: startTime,
			EndTime:   endTime,
		},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.hyperliquid.xyz/info", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rawCandles []rawCandle
	if err := json.NewDecoder(resp.Body).Decode(&rawCandles); err != nil {
		return nil, err
	}

	var candles []models.Candle
	for _, rc := range rawCandles {
		open, _ := strconv.ParseFloat(rc.Open, 64)
		high, _ := strconv.ParseFloat(rc.High, 64)
		low, _ := strconv.ParseFloat(rc.Low, 64)
		close, _ := strconv.ParseFloat(rc.Close, 64)
		volume, _ := strconv.ParseFloat(rc.Volume, 64)

		candles = append(candles, models.Candle{
			Time:   rc.Time,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
		})
	}

	return candles, nil
}
