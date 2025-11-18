package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

	if width == 0 {
		width = 80
	}

	header := renderHeader(ticker, width)
	mainContent := renderMainContent(ticker)

	// Combine header and main content with exact same spacing
	combined := header + mainContent

	// Apply padding to combined content (maintains exact spacing)
	content := lipgloss.NewStyle().Padding(1, 2).Render(combined)

	// Apply left green column
	return applyLeftColumn(content, 2)
}

// renderHeader renders the header section combining left and right
func renderHeader(ticker *models.Ticker, width int) string {
	headerFg := lipgloss.Color("#dcfe52")
	headerStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Bold(true).
		Width(width).
		Align(lipgloss.Left).
		Padding(0, 1)
	headerLineStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Width(width)

	leftLines := renderHeaderLeft()
	rightLines := renderHeaderRight(ticker)

	var b strings.Builder
	for i, leftLine := range leftLines {
		if i == 0 {
			// First line: combine left and right (binary token)
			spacing := width - len(leftLine) - len(rightLines[0]) - 2
			if spacing < 0 {
				spacing = 1
			}
			lineWithRight := leftLine + strings.Repeat(" ", spacing) + rightLines[0]
			b.WriteString(headerStyle.Render(lineWithRight))
		} else if i == 1 {
			// Second line: combine left and right (contract address)
			spacing := width - len(leftLine) - len(rightLines[1]) - 2
			if spacing < 0 {
				spacing = 1
			}
			lineWithRight := leftLine + strings.Repeat(" ", spacing) + rightLines[1]
			b.WriteString(headerStyle.Render(lineWithRight))
		} else if i >= 2 && i < len(rightLines) {
			// ASCII art lines: align flush to the right
			asciiLine := rightLines[i]
			// Count visual width (rune count) not byte length
			leftRuneCount := utf8.RuneCountInString(leftLine)
			asciiRuneCount := utf8.RuneCountInString(asciiLine)
			// Offset to align properly with the right edge
			spacing := width - leftRuneCount - asciiRuneCount - 5
			if spacing < 0 {
				spacing = 0
			}
			lineWithRight := leftLine + strings.Repeat(" ", spacing) + asciiLine
			b.WriteString(headerStyle.Render(lineWithRight))
		} else {
			// Remaining lines: just left
			b.WriteString(headerStyle.Render(leftLine))
		}
		b.WriteString("\n")
	}

	// Green underline
	b.WriteString(headerLineStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n\n")

	return b.String()
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

	result := []string{binarySymbol, contractAddress}
	result = append(result, asciiArt...)

	return result
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
					if lines[i] != "" {
						lines[i] += " "
					}
					lines[i] += charArt[i]
				}
			}
		}
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

// applyLeftColumn applies the green left column to the content
func applyLeftColumn(content string, columnWidth int) string {
	leftColumnStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#dcfe52")).
		Width(columnWidth)

	contentLines := strings.Split(content, "\n")
	var result strings.Builder
	for _, line := range contentLines {
		result.WriteString(leftColumnStyle.Render(" "))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
