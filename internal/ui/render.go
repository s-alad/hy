package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"hy/internal/flex"
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

	if width == 0 {
		width = 80
	}

	// Calculate content width (width - green bar width)
	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Header
	header := renderHeaderNode(ticker, contentWidth)

	// Main Content
	mainContent := flex.NewContent(renderMainContent(ticker))

	// Combine Header and Main Content
	contentCol := flex.New(flex.Column).
		Add(header).
		Add(mainContent).
		WithStyle(lipgloss.NewStyle().Padding(1, 2))

	// Render content to get height
	renderedContent := contentCol.Render(contentWidth, 0)
	contentHeight := lipgloss.Height(renderedContent)

	// Render green bar
	greenBar := lipgloss.NewStyle().
		Background(lipgloss.Color("#dcfe52")).
		Width(2).
		Height(contentHeight).
		Render("")

	// Combine
	return lipgloss.JoinHorizontal(lipgloss.Top, greenBar, renderedContent)
}

func renderHeaderNode(ticker *models.Ticker, width int) *flex.Node {
	headerFg := lipgloss.Color("#dcfe52")
	headerStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Bold(true)

	leftLines := renderHeaderLeft()
	leftContent := strings.Join(leftLines, "\n")

	rightLines := renderHeaderRight(ticker)
	rightContent := strings.Join(rightLines, "\n")

	// Header Row
	headerRow := flex.New(flex.Row).
		Justify(flex.JustifySpaceBetween).
		Add(
			flex.NewContent(leftContent).WithStyle(headerStyle),
			flex.NewContent(rightContent).WithStyle(headerStyle.Copy().Align(lipgloss.Right)),
		).
		WithWidth(width - 4) // Adjust for padding (left 2, right 2? No, padding is applied to container)
	// The container has Padding(1, 2). So horizontal padding is 2+2=4.
	// We passed contentWidth to renderHeaderNode.
	// contentWidth is (TotalWidth - 2).
	// The contentCol has Padding(1, 2).
	// So available width for headerRow is contentWidth - 4.

	// Green underline
	// We want the underline to span the full width of the content area (excluding padding?)
	// The original code had: width(width).
	// But here we are inside padding.
	// Let's make it span the available width.
	underline := flex.NewContent(strings.Repeat("─", width-4)).
		WithStyle(lipgloss.NewStyle().Foreground(headerFg))

	return flex.New(flex.Column).
		Add(headerRow).
		Add(underline).
		Add(flex.NewContent("\n"))
}

// renderHeaderLeft returns the left side of the header (HY ASCII art)
func renderHeaderLeft() []string {
	hyASCII := `
   __       
  / /  __ __
 / _ \/ // /
/_//_/\_, / 
     /___/  
	 v1.0.0
                `
	hyLines := strings.Split(hyASCII, "\n")
	cleanHyLines := []string{}
	for _, line := range hyLines {
		if strings.TrimSpace(line) != "" {
			cleanHyLines = append(cleanHyLines, strings.TrimRight(line, " \t"))
		}
	}
	return cleanHyLines
}

// renderHeaderRight returns the right side of the header (binary + address + ASCII art)
func renderHeaderRight(ticker *models.Ticker) []string {
	// Binary token symbol
	binarySymbol := "$ "
	for _, char := range ticker.Symbol {
		binarySymbol += fmt.Sprintf("%08b ", byte(char))
	}
	binarySymbol = strings.TrimSpace(binarySymbol)

	// Contract address
	contractAddress := "0x0d01dc56dcaaca66ad901c959b4011ec"

	// ASCII art of ticker symbol
	asciiArt := renderTickerASCII(ticker.Symbol)

	result := []string{contractAddress, ""}
	result = append(result, asciiArt...)

	// Prepend binary symbol? Original code had it.
	// Original code:
	// Line 0: binary
	// Line 1: address
	// Line 2+: ascii

	finalResult := []string{binarySymbol, contractAddress, ""}
	finalResult = append(finalResult, asciiArt...)

	return finalResult
}

// renderTickerASCII converts ticker symbol to ASCII art
func renderTickerASCII(symbol string) []string {
	asciiMap := map[rune][]string{
		'H': {"░█░█", "░█▀█", "░▀░▀"},
		'Y': {"░█░█", "░░█░", "░░▀░"},
		'P': {"░█▀█", "░█▀▀", "░▀░░"},
		'E': {"░█▀▀", "░█▀▀", "░▀▀▀"},
	}

	// Default to 3 lines
	lines := make([]string, 3)
	for i := range lines {
		lines[i] = ""
	}

	// Build ASCII art character by character
	for _, char := range symbol {
		if charArt, ok := asciiMap[char]; ok {
			for i := 0; i < 3; i++ {
				if i < len(charArt) {
					lines[i] += charArt[i]
				}
			}
		}
	}

	// Add USDC ASCII to the right
	usdc := []string{
		"░░░█░█░█░█▀▀░█▀▄░█▀▀",
		"░▄▀░░█░█░▀▀█░█░█░█░░",
		"░▀░░░▀▀▀░▀▀▀░▀▀░░▀▀▀",
	}
	for i := 0; i < 3; i++ {
		lines[i] += usdc[i]
	}

	// Add vertical bar dollar sign prefix to each line
	prefixes := []string{"| ", "$ ", "| "}
	for i := range lines {
		if i < len(prefixes) {
			lines[i] = prefixes[i] + lines[i]
		}
	}

	return lines
}

// renderMainContent renders the main content section
func renderMainContent(ticker *models.Ticker) string {
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	green := lipgloss.Color("2")

	var b strings.Builder
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

	return b.String()
}
