package ui

import (
	"time"

	"hy/internal/models"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChartModel struct {
	Chart  tslc.Model
	Width  int
	Height int
}

func NewChartModel(width, height int) ChartModel {
	return ChartModel{
		Chart:  tslc.New(width, height),
		Width:  width,
		Height: height,
	}
}

func (m ChartModel) Init() tea.Cmd {
	return nil
}

func (m ChartModel) Update(msg tea.Msg) (ChartModel, tea.Cmd) {
	var cmd tea.Cmd
	m.Chart, cmd = m.Chart.Update(msg)
	return m, cmd
}

func (m ChartModel) View() string {
	return m.Chart.View()
}

func (m *ChartModel) SetCandles(candles []models.Candle) {
	if len(candles) == 0 {
		return
	}

	// Calculate min/max Y and Time
	minTime := time.Unix(candles[0].Time/1000, 0)
	maxTime := time.Unix(candles[len(candles)-1].Time/1000, 0)
	minY := candles[0].Low
	maxY := candles[0].High

	for _, c := range candles {
		if c.Low < minY {
			minY = c.Low
		}
		if c.High > maxY {
			maxY = c.High
		}
	}

	// Add some padding to Y range
	padding := (maxY - minY) * 0.1
	minY -= padding
	maxY += padding

	// Re-initialize chart with new range
	m.Chart = tslc.New(m.Width, m.Height,
		tslc.WithTimeRange(minTime, maxTime),
		tslc.WithYRange(minY, maxY),
	)

	// Push data
	for _, c := range candles {
		t := time.Unix(c.Time/1000, 0)
		m.Chart.PushDataSet("open", tslc.TimePoint{Time: t, Value: c.Open})
		m.Chart.PushDataSet("high", tslc.TimePoint{Time: t, Value: c.High})
		m.Chart.PushDataSet("low", tslc.TimePoint{Time: t, Value: c.Low})
		m.Chart.PushDataSet("close", tslc.TimePoint{Time: t, Value: c.Close})
	}

	m.draw()
}

func (m *ChartModel) Resize(width, height int) {
	m.Width = width
	m.Height = height
	m.Chart.Resize(width, height)
	m.draw()
}

func (m *ChartModel) draw() {
	// Draw Candles
	m.Chart.DrawCandle("open", "high", "low", "close",
		lipgloss.NewStyle().Foreground(lipgloss.Color("2")), // Bullish (Green)
		lipgloss.NewStyle().Foreground(lipgloss.Color("1")), // Bearish (Red)
	)
}
