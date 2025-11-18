package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"hy/internal/models"
)

func Render(ticker *models.Ticker, err error, width int) string {
	if err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).
			Render(fmt.Sprintf("Error: %v", err))
	}
	if ticker == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).
			Render("Loading...")
	}

	green := lipgloss.Color("2")
	headerFg := lipgloss.Color("#dcfe52")
	if width == 0 {
		width = 80
	}
	headerStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Bold(true).
		Width(width).
		Align(lipgloss.Left).
		Padding(0, 1)
	headerLineStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Width(width)
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	var b strings.Builder
	bigHY := `
   __       
  / /  __ __
 / _ \/ // /
/_//_/\_, / 
     /___/  
	 v1.0.0
                `
	headerLines := strings.Split(bigHY, "\n")
	for _, line := range headerLines {
		if strings.TrimSpace(line) != "" {
			b.WriteString(headerStyle.Render(line))
			b.WriteString("\n")
		}
	}
	b.WriteString(headerLineStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("%s/USDC\n\n", ticker.Symbol))
	b.WriteString(fmt.Sprintf("%s $%.6f\n", label.Render("PRICE:"), ticker.LastPrice))
	b.WriteString(fmt.Sprintf("%s $%.6f\n", label.Render("BID:"), ticker.BidPrice))
	b.WriteString(fmt.Sprintf("%s $%.6f\n", label.Render("ASK:"), ticker.AskPrice))
	b.WriteString(fmt.Sprintf("%s $%.2f\n", label.Render("VOLUME:"), ticker.Volume24h))

	change := fmt.Sprintf("%.2f%%", ticker.Change24h)
	changeColor := green
	if ticker.Change24h < 0 {
		changeColor = lipgloss.Color("1")
	}
	b.WriteString(fmt.Sprintf("%s %s\n", label.Render("24H CHANGE:"),
		lipgloss.NewStyle().Foreground(changeColor).Render(change)))

	content := lipgloss.NewStyle().Padding(1, 2).Render(b.String())

	// Add left green column
	leftColumnWidth := 2
	leftColumnStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#dcfe52")).
		Width(leftColumnWidth)

	// Split content into lines and add left column to each
	contentLines := strings.Split(content, "\n")
	var result strings.Builder
	for _, line := range contentLines {
		result.WriteString(leftColumnStyle.Render(" "))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
