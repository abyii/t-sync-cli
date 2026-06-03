package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SelectorModel struct {
	VersionID uint64
	Files     []FileItem
	Root      *TreeNode
	Selected  map[string]bool // path -> selected
	Cursor    int
	Filter    string
	Quitting  bool
	Submitted bool
	Width     int
	Height    int
}

func NewSelectorModel(versionID uint64, items []FileItem) SelectorModel {
	root := buildTree(items)
	selected := make(map[string]bool)
	for _, item := range items {
		selected[item.Path] = true
	}

	return SelectorModel{
		VersionID: versionID,
		Files:     items,
		Root:      root,
		Selected:  selected,
		Cursor:    0,
	}
}

func (m SelectorModel) Init() tea.Cmd {
	return nil
}

func (m *SelectorModel) getVisibleNodes() []*TreeNode {
	var list []*TreeNode
	query := strings.ToLower(m.Filter)
	
	var traverse func(*TreeNode)
	traverse = func(node *TreeNode) {
		if node != m.Root {
			list = append(list, node)
		}
		
		shouldExpand := node.Expanded || (query != "")
		if node.IsDir && shouldExpand {
			for _, child := range node.Children {
				if query == "" || child.matchesFilter(query) {
					traverse(child)
				}
			}
		}
	}
	
	for _, child := range m.Root.Children {
		if query == "" || child.matchesFilter(query) {
			traverse(child)
		}
	}
	return list
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		visible := m.getVisibleNodes()
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
			if m.Filter == "" {
				m.Quitting = true
				return m, tea.Quit
			}
			m.Filter += "q"
			m.updateFilter()

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			} else if len(visible) > 0 {
				m.Cursor = len(visible) - 1 // wrap
			}

		case "down", "j":
			if len(visible) > 0 {
				if m.Cursor < len(visible)-1 {
					m.Cursor++
				} else {
					m.Cursor = 0 // wrap
				}
			}

		case "left", "h":
			if len(visible) > 0 && m.Cursor < len(visible) {
				node := visible[m.Cursor]
				if node.IsDir && node.Expanded {
					node.Expanded = false
				} else if node.Parent != nil && node.Parent != m.Root {
					for idx, vNode := range visible {
						if vNode == node.Parent {
							m.Cursor = idx
							break
						}
					}
				}
			}

		case "right", "l":
			if len(visible) > 0 && m.Cursor < len(visible) {
				node := visible[m.Cursor]
				if node.IsDir {
					node.Expanded = true
				}
			}

		case " ", "space":
			if len(visible) > 0 && m.Cursor < len(visible) {
				node := visible[m.Cursor]
				toggleNode(node, m.Selected)
			}

		case "ctrl+a":
			allChecked := true
			for _, node := range visible {
				if !node.IsDir {
					if !m.Selected[node.Path] {
						allChecked = false
						break
					}
				}
			}
			targetState := !allChecked
			for _, node := range visible {
				if !node.IsDir {
					m.Selected[node.Path] = targetState
				}
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
	visible := m.getVisibleNodes()
	if m.Cursor >= len(visible) {
		m.Cursor = 0
	}
}

func toggleNode(node *TreeNode, selected map[string]bool) {
	if !node.IsDir {
		selected[node.Path] = !selected[node.Path]
		return
	}

	allChecked := true
	var checkChildren func(*TreeNode)
	checkChildren = func(n *TreeNode) {
		if !n.IsDir {
			if !selected[n.Path] {
				allChecked = false
			}
		} else {
			for _, child := range n.Children {
				checkChildren(child)
			}
		}
	}
	checkChildren(node)

	targetState := !allChecked
	var setChildrenState func(*TreeNode)
	setChildrenState = func(n *TreeNode) {
		if !n.IsDir {
			selected[n.Path] = targetState
		} else {
			for _, child := range n.Children {
				setChildrenState(child)
			}
		}
	}
	setChildrenState(node)
}

func getNodeCheckbox(node *TreeNode, selected map[string]bool) string {
	if !node.IsDir {
		if selected[node.Path] {
			return "[x]"
		}
		return "[ ]"
	}

	var totalFiles int
	var checkedFiles int
	var countChecked func(*TreeNode)
	countChecked = func(n *TreeNode) {
		if !n.IsDir {
			totalFiles++
			if selected[n.Path] {
				checkedFiles++
			}
		} else {
			for _, child := range n.Children {
				countChecked(child)
			}
		}
	}
	countChecked(node)

	if checkedFiles == totalFiles {
		return "[x]"
	} else if checkedFiles > 0 {
		return "[-]"
	} else {
		return "[ ]"
	}
}

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

	selCount := 0
	for _, item := range m.Files {
		if m.Selected[item.Path] {
			selCount++
		}
	}
	stats := StyleHelp.Render(fmt.Sprintf("Selected: %d/%d files", selCount, len(m.Files)))

	visible := m.getVisibleNodes()
	leftLines := make([]string, 0)
	
	maxVisible := 15
	startIdx := 0
	if m.Cursor >= maxVisible {
		startIdx = m.Cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(visible) {
		endIdx = len(visible)
	}

	for i := startIdx; i < endIdx; i++ {
		node := visible[i]
		
		depth := node.depth()
		indent := strings.Repeat("  ", depth)
		
		icon := "📄 "
		if node.IsDir {
			if node.Expanded {
				icon = "▼ 📂 "
			} else {
				icon = "▶ 📁 "
			}
		}
		
		cbStr := getNodeCheckbox(node, m.Selected)
		checkbox := cbStr
		if cbStr == "[x]" {
			checkbox = StyleSelection.Render("[x]")
		} else if cbStr == "[-]" {
			checkbox = StyleSelection.Render("[-]")
		}

		sizeStr := ""
		if !node.IsDir {
			sizeStr = fmt.Sprintf(" (%s)", FormatBytes(node.Item.Size))
		}
		
		rowText := fmt.Sprintf("%s %s%s%s%s", checkbox, indent, icon, node.Name, sizeStr)
		if len(rowText) > 80 {
			rowText = rowText[:77] + "..."
		}

		if i == m.Cursor {
			leftLines = append(leftLines, StyleSelectedRow.Render(fmt.Sprintf(" ▸ %s", rowText)))
		} else {
			leftLines = append(leftLines, StyleNormalRow.Render(fmt.Sprintf("   %s", rowText)))
		}
	}

	if len(visible) == 0 {
		leftLines = append(leftLines, StyleHelp.Render("   (No files matching filter)"))
	}

	for len(leftLines) < maxVisible {
		leftLines = append(leftLines, "")
	}

	leftPanel := strings.Join(leftLines, "\n")
	panelHeader := StyleHeader.Render("Selective Checklist Tree") + "\n"
	leftBox := StyleBorder.Width(84).Height(17).Render(panelHeader + leftPanel)

	// Help footer
	footer := StyleHelp.Render("\n↑/↓: Navigate • Space: Toggle • →: Expand • ←: Collapse • Ctrl+A: Toggle All • Type to Filter • Enter: Submit • Esc/q: Cancel")

	return title + "\n" + searchBar + " | " + stats + "\n\n" + leftBox + footer
}
