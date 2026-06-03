package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FileItem struct {
	Path     string
	Size     int64
	CompSize int64
	CRC32    uint32
	Modified string
	Key      string
	KeyID    string
}

type TreeNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*TreeNode
	Parent   *TreeNode
	Expanded bool
	Item     *FileItem
}

type BrowserModel struct {
	VersionID uint64
	Files     []FileItem
	Root      *TreeNode
	Cursor    int
	Filter    string
	Quitting  bool
	Width     int
	Height    int
}

func (node *TreeNode) depth() int {
	d := 0
	curr := node.Parent
	for curr != nil && curr.Parent != nil {
		d++
		curr = curr.Parent
	}
	return d
}

func (node *TreeNode) matchesFilter(query string) bool {
	if query == "" {
		return true
	}
	if !node.IsDir {
		return strings.Contains(strings.ToLower(node.Path), query)
	}
	for _, child := range node.Children {
		if child.matchesFilter(query) {
			return true
		}
	}
	return false
}

func buildTree(items []FileItem) *TreeNode {
	root := &TreeNode{
		Name:  "root",
		IsDir: true,
	}

	for i := range items {
		item := &items[i]
		parts := strings.Split(filepath.ToSlash(item.Path), "/")
		
		current := root
		var currentPath []string
		for idx, part := range parts {
			if part == "" {
				continue
			}
			currentPath = append(currentPath, part)
			fullPath := strings.Join(currentPath, "/")
			
			isLast := idx == len(parts)-1
			
			var found *TreeNode
			for _, child := range current.Children {
				if child.Name == part && child.IsDir == !isLast {
					found = child
					break
				}
			}
			
			if found == nil {
				newNode := &TreeNode{
					Name:   part,
					Path:   fullPath,
					IsDir:  !isLast,
					Parent: current,
				}
				if isLast {
					newNode.Item = item
				}
				current.Children = append(current.Children, newNode)
				sortChildren(current.Children)
				current = newNode
			} else {
				current = found
			}
		}
	}
	
	return root
}

func sortChildren(children []*TreeNode) {
	sort.Slice(children, func(i, j int) bool {
		if children[i].IsDir != children[j].IsDir {
			return children[i].IsDir
		}
		return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
	})
}

func NewBrowserModel(versionID uint64, items []FileItem) BrowserModel {
	root := buildTree(items)
	return BrowserModel{
		VersionID: versionID,
		Files:     items,
		Root:      root,
		Cursor:    0,
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return nil
}

func (m *BrowserModel) getVisibleNodes() []*TreeNode {
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

func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		visible := m.getVisibleNodes()
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
			} else if len(visible) > 0 {
				m.Cursor = len(visible) - 1 // wrap around
			}

		case "down", "j":
			if len(visible) > 0 {
				if m.Cursor < len(visible)-1 {
					m.Cursor++
				} else {
					m.Cursor = 0 // wrap around
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

		case "right", "l", "enter", " ":
			if len(visible) > 0 && m.Cursor < len(visible) {
				node := visible[m.Cursor]
				if node.IsDir {
					node.Expanded = !node.Expanded
				}
			}

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

func (m *BrowserModel) updateFilter() {
	visible := m.getVisibleNodes()
	if m.Cursor >= len(visible) {
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

	visible := m.getVisibleNodes()
	leftLines := make([]string, 0)
	
	maxVisible := 16
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
		
		row := indent + icon + node.Name
		if len(row) > 48 {
			row = row[:45] + "..."
		}

		if i == m.Cursor {
			leftLines = append(leftLines, StyleSelectedRow.Render(fmt.Sprintf(" ▸ %-50s", row)))
		} else {
			leftLines = append(leftLines, StyleNormalRow.Render(fmt.Sprintf("   %-50s", row)))
		}
	}

	if len(visible) == 0 {
		leftLines = append(leftLines, StyleHelp.Render("   (No files matching filter)"))
	}

	for len(leftLines) < maxVisible {
		leftLines = append(leftLines, "")
	}

	leftPanel := strings.Join(leftLines, "\n")
	leftPanelHeader := StyleHeader.Render("Files Tree") + "\n"
	leftBox := StyleBorder.Width(56).Height(18).Render(leftPanelHeader + leftPanel)

	// Right Details Panel
	rightPanel := ""
	if len(visible) > 0 && m.Cursor < len(visible) {
		node := visible[m.Cursor]
		if !node.IsDir {
			item := node.Item
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
			var fileCount int
			var totalSize int64
			var totalCompSize int64
			
			var countStats func(*TreeNode)
			countStats = func(n *TreeNode) {
				if !n.IsDir {
					fileCount++
					totalSize += n.Item.Size
					totalCompSize += n.Item.CompSize
				} else {
					for _, child := range n.Children {
						countStats(child)
					}
				}
			}
			countStats(node)
			
			savings := 0.0
			if totalSize > 0 {
				savings = 100.0 - (float64(totalCompSize) / float64(totalSize) * 100.0)
			}

			rightPanel = fmt.Sprintf(
				"Directory:\033[1;37m%s/\033[0m\n\n"+
					"Files:    %d\n"+
					"Size:     %s (%d B)\n"+
					"Compressed: %s (%d B)\n"+
					"Savings:  %.1f%%",
				node.Path,
				fileCount,
				FormatBytes(totalSize), totalSize,
				FormatBytes(totalCompSize), totalCompSize,
				savings,
			)
		}
	} else {
		rightPanel = StyleHelp.Render("No node selected.")
	}

	rightPanelHeader := StyleHeader.Render("Details") + "\n"
	rightBox := StyleBorder.Width(44).Height(18).Render(rightPanelHeader + rightPanel)

	// Join Panels Side-by-Side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "  ", rightBox)

	// Help footer
	footer := StyleHelp.Render("\n↑/↓: Navigate • Enter/Space/→: Expand/Collapse • Left: Go to Parent • Type to Filter • Esc/q: Exit")

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
