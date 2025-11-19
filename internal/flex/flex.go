package flex

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Direction int

const (
	Row Direction = iota
	Column
)

type Justify int

const (
	JustifyStart Justify = iota
	JustifyCenter
	JustifyEnd
	JustifySpaceBetween
	JustifySpaceAround
)

type Align int

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

type Node struct {
	direction Direction
	justify   Justify
	align     Align

	children []*Node
	content  string // Leaf node content

	flex   int // Flex grow factor
	width  int // Fixed width override
	height int // Fixed height override

	style lipgloss.Style
}

func New(d Direction) *Node {
	return &Node{
		direction: d,
		style:     lipgloss.NewStyle(),
	}
}

func NewContent(content string) *Node {
	return &Node{
		content: content,
		style:   lipgloss.NewStyle(),
	}
}

func (n *Node) Add(children ...*Node) *Node {
	n.children = append(n.children, children...)
	return n
}

func (n *Node) WithStyle(s lipgloss.Style) *Node {
	n.style = s
	return n
}

func (n *Node) WithFlex(f int) *Node {
	n.flex = f
	return n
}

func (n *Node) WithWidth(w int) *Node {
	n.width = w
	return n
}

func (n *Node) WithHeight(h int) *Node {
	n.height = h
	return n
}

func (n *Node) Justify(j Justify) *Node {
	n.justify = j
	return n
}

func (n *Node) Align(a Align) *Node {
	n.align = a
	return n
}

// Render renders the node and its children within the given constraints.
func (n *Node) Render(maxWidth, maxHeight int) string {
	// Leaf node
	if len(n.children) == 0 && n.content != "" {
		s := n.style
		if n.width > 0 {
			s = s.Width(n.width)
		} else if n.flex > 0 && maxWidth > 0 {
			// If flex > 0, we want to fill the allocated space
			s = s.Width(maxWidth)
		}

		if n.height > 0 {
			s = s.Height(n.height)
		} else if n.flex > 0 && maxHeight > 0 {
			// If flex > 0, we want to fill the allocated space
			s = s.Height(maxHeight)
		}

		return s.Render(n.content)
	}

	// Container node
	if len(n.children) == 0 {
		return n.style.Render("")
	}

	// Determine available space
	availableWidth := maxWidth
	availableHeight := maxHeight
	if n.width > 0 {
		availableWidth = n.width
	}
	if n.height > 0 {
		availableHeight = n.height
	}

	isRow := n.direction == Row

	// 1. Measure fixed children
	totalFixedSize := 0
	totalFlex := 0

	childContents := make([]string, len(n.children))
	childSizes := make([]int, len(n.children)) // Main axis size

	for i, child := range n.children {
		if child.flex > 0 {
			totalFlex += child.flex
			continue
		}

		// Measure fixed child
		// Cross axis constraint
		cw, ch := 0, 0
		if isRow {
			ch = availableHeight
		} else {
			cw = availableWidth
		}

		rendered := child.Render(cw, ch)
		childContents[i] = rendered
		w, h := lipgloss.Size(rendered)

		if isRow {
			childSizes[i] = w
			totalFixedSize += w
		} else {
			childSizes[i] = h
			totalFixedSize += h
		}
	}

	// 2. Measure flex children
	remainingSpace := 0
	if isRow {
		remainingSpace = availableWidth - totalFixedSize
	} else {
		remainingSpace = availableHeight - totalFixedSize
	}
	if remainingSpace < 0 {
		remainingSpace = 0
	}

	if totalFlex > 0 {
		unitSpace := float64(remainingSpace) / float64(totalFlex)
		usedFlexSpace := 0

		for i, child := range n.children {
			if child.flex > 0 {
				flexSize := int(float64(child.flex) * unitSpace)

				cw, ch := 0, 0
				if isRow {
					cw = flexSize
					ch = availableHeight
				} else {
					cw = availableWidth
					ch = flexSize
				}

				// For flex children, we want to force the size if possible,
				// or at least pass it as constraint.
				// We can use lipgloss.Place to force size if the child renders smaller.

				rendered := child.Render(cw, ch)

				// Force size to flexSize?
				// If content is smaller, we might want to expand it?
				// Flexbox usually expands the item to fill the flex space.
				w, h := lipgloss.Size(rendered)

				if isRow {
					if w < flexSize {
						// Expand width
						rendered = lipgloss.PlaceHorizontal(flexSize, lipgloss.Left, rendered)
						w = flexSize
					}
					childSizes[i] = w
					usedFlexSpace += w
				} else {
					if h < flexSize {
						// Expand height
						rendered = lipgloss.PlaceVertical(flexSize, lipgloss.Top, rendered)
						h = flexSize
					}
					childSizes[i] = h
					usedFlexSpace += h
				}
				childContents[i] = rendered
			}
		}
		totalFixedSize += usedFlexSpace
	}

	// 3. Distribute space (Justify)
	totalSpace := 0
	if isRow {
		totalSpace = availableWidth
	} else {
		totalSpace = availableHeight
	}

	freeSpace := totalSpace - totalFixedSize
	if freeSpace < 0 {
		freeSpace = 0
	}

	// Calculate gaps
	startGap := 0
	gaps := make([]int, len(n.children)) // gap after each child

	childCount := len(n.children)
	if childCount > 0 {
		switch n.justify {
		case JustifyStart:
			// All space at end
		case JustifyEnd:
			startGap = freeSpace
		case JustifyCenter:
			startGap = freeSpace / 2
		case JustifySpaceBetween:
			if childCount > 1 {
				avgGap := freeSpace / (childCount - 1)
				remainder := freeSpace % (childCount - 1)
				for i := 0; i < childCount-1; i++ {
					gaps[i] = avgGap
					if i < remainder {
						gaps[i]++
					}
				}
			}
		case JustifySpaceAround:
			if childCount > 0 {
				// 2 units per item (one each side) -> 2 * count spaces?
				// No, space around is: half-space item space item half-space
				// Total units = 2 * count
				// Actually simpler: space / count.
				// Each item gets space/count distributed around it.
				// So gap before first = (space/count)/2
				// gap between = space/count
				// gap after last = (space/count)/2

				// Let's stick to the requested features first.
				// User didn't ask for SpaceAround.
				// But I'll implement it simply if needed.
				// For now, let's focus on Start, End, Center, SpaceBetween.
			}
		}
	}

	// 4. Assemble
	var parts []string

	// Add start gap
	if startGap > 0 {
		if isRow {
			parts = append(parts, strings.Repeat(" ", startGap))
		} else {
			parts = append(parts, strings.Repeat("\n", startGap))
		}
	}

	for i, content := range childContents {
		// Align (Cross Axis)
		if isRow {
			// Vertical alignment
			if availableHeight > 0 {
				h := lipgloss.Height(content)
				if h < availableHeight {
					pos := lipgloss.Top
					switch n.align {
					case AlignCenter:
						pos = lipgloss.Center
					case AlignEnd:
						pos = lipgloss.Bottom
					}
					content = lipgloss.PlaceVertical(availableHeight, pos, content)
				}
			}
		} else {
			// Horizontal alignment
			if availableWidth > 0 {
				w := lipgloss.Width(content)
				if w < availableWidth {
					pos := lipgloss.Left
					switch n.align {
					case AlignCenter:
						pos = lipgloss.Center
					case AlignEnd:
						pos = lipgloss.Right
					}
					content = lipgloss.PlaceHorizontal(availableWidth, pos, content)
				}
			}
		}

		parts = append(parts, content)

		// Add gap after child
		if i < len(gaps) && gaps[i] > 0 {
			if isRow {
				parts = append(parts, strings.Repeat(" ", gaps[i]))
			} else {
				parts = append(parts, strings.Repeat("\n", gaps[i]))
			}
		}
	}

	var result string
	if isRow {
		result = lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	} else {
		result = lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	return n.style.Render(result)
}
