package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FileItem struct {
	Path      string
	Size      int64
	CompSize  int64
	CRC32     uint32
	Modified  string
	Key       string
	KeyID     string
}

type BrowserModel struct {
	VersionID uint64
	Files     []FileItem
	Filtered  []FileItem
	Cursor    int
	Filter    string
	Quitting  bool
	Width     int
	Height    int
}

func NewBrowserModel(versionID uint64, items []FileItem) BrowserModel {
	return BrowserModel{
		VersionID: versionID,
		Files:     items,
		Filtered:  items,
		Cursor:    0,
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return nil
}

func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			m.Quitting = true
			return m, tea.Quit

		case "q", "esc":
			if m.Filter != "" {
				m.Filter = ""
				m.updateFilter()
				return m, nil
			}
			m.Quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			} else if len(m.Filtered) > 0 {
				m.Cursor = len(m.Filtered) - 1 // wrap around
			}

		case "down", "j":
			if len(m.Filtered) > 0 {
				if m.Cursor < len(m.Filtered)-1 {
					m.Cursor++
				} else {
					m.Cursor = 0 // wrap around
				}
			}

		case "backspace":
			if len(m.Filter) > 0 {
				m.Filter = m.Filter[:len(m.Filter)-1]
				m.updateFilter()
			}

		default:
			// Capture typing for filtering
			if len(msg.String()) == 1 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
				m.Filter += msg.String()
				m.updateFilter()
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	return m, nil
}

func (m *BrowserModel) updateFilter() {
	if m.Filter == "" {
		m.Filtered = m.Files
	} else {
		var filtered []FileItem
		query := strings.ToLower(m.Filter)
		for _, item := range m.Files {
			if strings.Contains(strings.ToLower(item.Path), query) {
				filtered = append(filtered, item)
			}
		}
		m.Filtered = filtered
	}
	// Reset cursor if out of bounds
	if m.Cursor >= len(m.Filtered) {
		m.Cursor = 0
	}
}

func (m BrowserModel) View() string {
	if m.Quitting {
		return ""
	}

	// Title Banner
	title := StyleTitle.Render(fmt.Sprintf(" T-SYNC FILE BROWSER • VERSION: %d ", m.VersionID))

	// Search bar view
	searchBar := ""
	if m.Filter != "" {
		searchBar = fmt.Sprintf("\nFilter: \033[33m%s\033[0m (esc to clear)", m.Filter)
	} else {
		searchBar = "\nFilter: (start typing to search...)"
	}

	// Calculate panel sizes. We split left (file list) and right (selected file details)
	leftLines := make([]string, 0)
	
	// Max files visible is e.g. 15
	maxVisible := 16
	startIdx := 0
	if m.Cursor >= maxVisible {
		startIdx = m.Cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.Filtered) {
		endIdx = len(m.Filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		item := m.Filtered[i]
		row := item.Path
		if len(row) > 48 {
			row = "..." + row[len(row)-45:]
		}

		if i == m.Cursor {
			leftLines = append(leftLines, StyleSelectedRow.Render(fmt.Sprintf(" ▸ %-50s", row)))
		} else {
			leftLines = append(leftLines, StyleNormalRow.Render(fmt.Sprintf("   %-50s", row)))
		}
	}

	if len(m.Filtered) == 0 {
		leftLines = append(leftLines, StyleHelp.Render("   (No files matching filter)"))
	}

	// Fill remaining visible space to keep panel size consistent
	for len(leftLines) < maxVisible {
		leftLines = append(leftLines, "")
	}

	leftPanel := strings.Join(leftLines, "\n")
	leftPanelHeader := StyleHeader.Render("Files List") + "\n"
	leftBox := StyleBorder.Width(56).Height(18).Render(leftPanelHeader + leftPanel)

	// Right Details Panel
	rightPanel := ""
	if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
		item := m.Filtered[m.Cursor]
		savings := 0.0
		if item.Size > 0 {
			savings = 100.0 - (float64(item.CompSize) / float64(item.Size) * 100.0)
		}

		keyDisp := item.Key
		if len(keyDisp) > 28 {
			keyDisp = keyDisp[:28] + "..."
		}

		rightPanel = fmt.Sprintf(
			"Path:     \033[1;37m%s\033[0m\n\n"+
				"Size:     %s (%d B)\n"+
				"Compressed: %s (%d B)\n"+
				"Savings:  %.1f%%\n\n"+
				"CRC32:    0x%08x\n"+
				"Modified: %s\n\n"+
				"Blob Key: %s\n"+
				"Key ID:   %s",
			item.Path,
			FormatBytes(item.Size), item.Size,
			FormatBytes(item.CompSize), item.CompSize,
			savings,
			item.CRC32,
			item.Modified,
			keyDisp,
			item.KeyID,
		)
	} else {
		rightPanel = StyleHelp.Render("No file selected.")
	}

	rightPanelHeader := StyleHeader.Render("File Details") + "\n"
	rightBox := StyleBorder.Width(44).Height(18).Render(rightPanelHeader + rightPanel)

	// Join Panels Side-by-Side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "  ", rightBox)

	// Help footer
	footer := StyleHelp.Render("\n↑/↓: Scroll • Type to Filter • Esc/q: Exit")

	return title + "\n" + searchBar + "\n\n" + panels + footer
}

// FormatBytes formatting helper duplicated here for isolated rendering.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
