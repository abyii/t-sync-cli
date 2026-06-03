package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SelectorModel struct {
	VersionID uint64
	Files     []FileItem
	Filtered  []FileItem
	Selected  map[string]bool // path -> selected
	Cursor    int
	Filter    string
	Quitting  bool
	Submitted bool
	Width     int
	Height    int
}

func NewSelectorModel(versionID uint64, items []FileItem) SelectorModel {
	selected := make(map[string]bool)
	// Default: select all initially
	for _, item := range items {
		selected[item.Path] = true
	}

	return SelectorModel{
		VersionID: versionID,
		Files:     items,
		Filtered:  items,
		Selected:  selected,
		Cursor:    0,
	}
}

func (m SelectorModel) Init() tea.Cmd {
	return nil
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			m.Quitting = true
			return m, tea.Quit

		case "esc":
			if m.Filter != "" {
				m.Filter = ""
				m.updateFilter()
				return m, nil
			}
			m.Quitting = true
			return m, tea.Quit

		case "q":
			// If not typing search query, allow 'q' to exit
			if m.Filter == "" {
				m.Quitting = true
				return m, tea.Quit
			}
			m.Filter += "q"
			m.updateFilter()

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			} else if len(m.Filtered) > 0 {
				m.Cursor = len(m.Filtered) - 1 // wrap
			}

		case "down", "j":
			if len(m.Filtered) > 0 {
				if m.Cursor < len(m.Filtered)-1 {
					m.Cursor++
				} else {
					m.Cursor = 0 // wrap
				}
			}

		case " ", "space":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				path := m.Filtered[m.Cursor].Path
				m.Selected[path] = !m.Selected[path]
			}

		case "ctrl+a":
			// Select or deselect all items in the filtered view
			allSelected := true
			for _, item := range m.Filtered {
				if !m.Selected[item.Path] {
					allSelected = false
					break
				}
			}
			// Toggle them
			for _, item := range m.Filtered {
				m.Selected[item.Path] = !allSelected
			}

		case "enter":
			m.Submitted = true
			return m, tea.Quit

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

func (m *SelectorModel) updateFilter() {
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

// GetSelectedPaths returns the list of checked file paths
func (m SelectorModel) GetSelectedPaths() []string {
	var paths []string
	for path, sel := range m.Selected {
		if sel {
			paths = append(paths, path)
		}
	}
	return paths
}

func (m SelectorModel) View() string {
	if m.Quitting {
		return ""
	}

	// Title Banner
	title := StyleTitle.Render(fmt.Sprintf(" SELECTIVE RESTORE • VERSION: %d ", m.VersionID))

	// Search bar view
	searchBar := ""
	if m.Filter != "" {
		searchBar = fmt.Sprintf("\nFilter: \033[33m%s\033[0m (esc to clear)", m.Filter)
	} else {
		searchBar = "\nFilter: (start typing to search...)"
	}

	// Count selected files
	selCount := 0
	for _, item := range m.Files {
		if m.Selected[item.Path] {
			selCount++
		}
	}
	stats := StyleHelp.Render(fmt.Sprintf("Selected: %d/%d files", selCount, len(m.Files)))

	// File selection rows
	leftLines := make([]string, 0)
	maxVisible := 15
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
		isChecked := m.Selected[item.Path]

		checkbox := "[ ]"
		if isChecked {
			checkbox = StyleSelection.Render("[x]")
		}

		rowText := fmt.Sprintf("%s %s (%s)", checkbox, item.Path, FormatBytes(item.Size))
		if len(rowText) > 80 {
			rowText = rowText[:77] + "..."
		}

		if i == m.Cursor {
			leftLines = append(leftLines, StyleSelectedRow.Render(fmt.Sprintf(" ▸ %s", rowText)))
		} else {
			leftLines = append(leftLines, StyleNormalRow.Render(fmt.Sprintf("   %s", rowText)))
		}
	}

	if len(m.Filtered) == 0 {
		leftLines = append(leftLines, StyleHelp.Render("   (No files matching filter)"))
	}

	// Fill space
	for len(leftLines) < maxVisible {
		leftLines = append(leftLines, "")
	}

	leftPanel := strings.Join(leftLines, "\n")
	panelHeader := StyleHeader.Render("Selective Checklist") + "\n"
	leftBox := StyleBorder.Width(84).Height(17).Render(panelHeader + leftPanel)

	// Help footer
	footer := StyleHelp.Render("\n↑/↓: Scroll • Space: Toggle • Ctrl+A: Toggle All • Type to Filter • Enter: Submit • Esc/q: Cancel")

	return title + "\n" + searchBar + " | " + stats + "\n\n" + leftBox + footer
}
