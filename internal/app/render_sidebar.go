package app

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Gaurav-Gosain/tuios/internal/config"
)

// SidebarWidthPercent is the percentage of screen width for sidebar (20%)
const SidebarWidthPercent = 0.20

// SidebarMinWidth is the minimum sidebar width in columns
const SidebarMinWidth = 25

// SidebarMaxWidth is the maximum sidebar width in columns
const SidebarMaxWidth = 50

// ZIndexSidebar is the z-index for the sidebar overlay
const ZIndexSidebar = 1003

// SidebarItemPosition stores clickable region for a sidebar item
type SidebarItemPosition struct {
	WindowIndex int // Index in m.Windows
	StartY      int // Y position start (inclusive)
	EndY        int // Y position end (exclusive)
}

// SidebarLayout stores calculated sidebar layout for hit detection
type SidebarLayout struct {
	Width         int                   // Sidebar width in columns
	ItemPositions []SidebarItemPosition // Clickable regions
	WorkspaceY    map[int]int           // Y position of each workspace header
}

// GetSidebarWidth calculates the sidebar width based on screen size
func (m *OS) GetSidebarWidth() int {
	width := int(float64(m.GetRenderWidth()) * SidebarWidthPercent)
	if width < SidebarMinWidth {
		width = SidebarMinWidth
	}
	if width > SidebarMaxWidth {
		width = SidebarMaxWidth
	}
	return width
}

// CalculateSidebarLayout calculates sidebar layout for rendering and hit detection
// Must match exactly how renderSidebar builds lines
func (m *OS) CalculateSidebarLayout() SidebarLayout {
	layout := SidebarLayout{
		Width:         m.GetSidebarWidth(),
		ItemPositions: make([]SidebarItemPosition, 0),
		WorkspaceY:    make(map[int]int),
	}

	// Group windows by workspace
	workspaceWindows := make(map[int][]int) // workspace -> window indices
	for i, w := range m.Windows {
		workspaceWindows[w.Workspace] = append(workspaceWindows[w.Workspace], i)
	}

	// Get sorted workspace numbers
	workspaces := make([]int, 0, len(workspaceWindows))
	for ws := range workspaceWindows {
		workspaces = append(workspaces, ws)
	}
	sort.Ints(workspaces)

	// Match renderSidebar exactly:
	// Line 0: Title "Windows"
	// Line 1: blank
	// +1 for border
	currentY := 3

	for wsIdx, ws := range workspaces {
		indices := workspaceWindows[ws]

		// Workspace header line
		layout.WorkspaceY[ws] = currentY
		currentY++

		// Window items in this workspace
		for _, idx := range indices {
			layout.ItemPositions = append(layout.ItemPositions, SidebarItemPosition{
				WindowIndex: idx,
				StartY:      currentY,
				EndY:        currentY + 1,
			})
			currentY++
		}

		// Gap between workspaces (except after last)
		if wsIdx < len(workspaces)-1 {
			currentY++
		}
	}

	return layout
}

// renderSidebar renders the browser-style sidebar with window list
// Uses same design language as dock and help overlays
func (m *OS) renderSidebar() *lipgloss.Layer {
	if !m.SidebarVisible {
		return nil
	}

	sidebarWidth := m.GetSidebarWidth()
	sidebarHeight := m.GetRenderHeight()
	topMargin := m.GetTopMargin()

	// Use project's standard colors
	bgColor := lipgloss.Color("#1a1a2e")     // Same as time overlay, which-key
	borderColor := lipgloss.Color("#303040") // Same as dock separator
	titleColor := lipgloss.Color("14")       // Cyan - same as help titles
	mutedColor := lipgloss.Color("#808090")  // Same as dock muted

	// Sidebar container - rounded border like help overlay
	containerStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(sidebarHeight - topMargin).
		Background(bgColor).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	// Title style - matches help overlay title
	titleStyle := lipgloss.NewStyle().
		Width(sidebarWidth-4).
		Foreground(titleColor).
		Bold(true).
		Padding(0, 1)

	// Workspace header - muted like dock sections
	workspaceStyle := lipgloss.NewStyle().
		Width(sidebarWidth-4).
		Foreground(mutedColor).
		Padding(0, 1)

	// Build content
	var lines []string

	// Title
	lines = append(lines, titleStyle.Render("Windows"))
	lines = append(lines, "")

	// Group windows by workspace
	workspaceWindows := make(map[int][]int)
	for i, w := range m.Windows {
		workspaceWindows[w.Workspace] = append(workspaceWindows[w.Workspace], i)
	}

	// Sort workspaces
	workspaces := make([]int, 0, len(workspaceWindows))
	for ws := range workspaceWindows {
		workspaces = append(workspaces, ws)
	}
	sort.Ints(workspaces)

	// Pill characters from config (same as dock)
	leftPill := config.GetDockPillLeftChar()
	rightPill := config.GetDockPillRightChar()

	// Render each workspace
	for wsIdx, ws := range workspaces {
		indices := workspaceWindows[ws]

		// Workspace header
		wsMarker := ""
		if ws == m.CurrentWorkspace {
			wsMarker = "*"
		}
		wsHeader := fmt.Sprintf(" Workspace %d %s", ws, wsMarker)
		lines = append(lines, workspaceStyle.Render(wsHeader))

		// Window items
		for _, idx := range indices {
			w := m.Windows[idx]

			// Get display name
			displayName := w.CustomName
			if displayName == "" {
				displayName = w.Title
			}
			if displayName == "" {
				displayName = "terminal"
			}

			// Truncate if needed
			maxLen := sidebarWidth - 12
			if len(displayName) > maxLen {
				displayName = displayName[:maxLen-1] + "…"
			}

			// Determine colors based on state
			var pillBg, pillFg, textFg string
			isBold := false

			if m.SidebarFocused && idx == m.SidebarSelectedIndex {
				// Selected - blue pill (like dock focused)
				pillBg = "#4865f2"
				pillFg = "#ffffff"
				textFg = "#ffffff"
				isBold = true
			} else if idx == m.FocusedWindow && w.Workspace == m.CurrentWorkspace {
				// Focused but not selected - highlight bg
				pillBg = "#2a2a3e"
				pillFg = "#a0a0b0"
				textFg = "#a0a0b0"
			} else if w.Minimized {
				// Minimized - muted
				pillBg = "#1a1a2e"
				pillFg = "#808090"
				textFg = "#808090"
			} else {
				// Normal
				pillBg = "#1a1a2e"
				pillFg = "#a0a0b0"
				textFg = "#a0a0b0"
			}

			// Build pill-style item (like dock items)
			numStr := fmt.Sprintf(" %d ", idx+1)

			leftCircle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(pillBg)).
				Render(leftPill)

			numLabel := lipgloss.NewStyle().
				Background(lipgloss.Color(pillBg)).
				Foreground(lipgloss.Color(pillFg)).
				Bold(isBold).
				Render(numStr)

			rightCircle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(pillBg)).
				Render(rightPill)

			nameStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(textFg)).
				Bold(isBold)

			// Add minimized marker
			prefix := ""
			if w.Minimized {
				prefix = "[m] "
			}

			itemLine := fmt.Sprintf(" %s%s%s %s%s",
				leftCircle, numLabel, rightCircle,
				prefix, nameStyle.Render(displayName))

			lines = append(lines, itemLine)
		}

		// Spacing between workspaces
		if wsIdx < len(workspaces)-1 {
			lines = append(lines, "")
		}
	}

	// Empty state
	if len(m.Windows) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Width(sidebarWidth-4).
			Foreground(mutedColor).
			Italic(true).
			Padding(0, 1).
			Align(lipgloss.Center)
		lines = append(lines, "")
		lines = append(lines, emptyStyle.Render("No windows"))
		lines = append(lines, emptyStyle.Render("Press 'n' to create"))
	}

	// Footer hint - like help overlay
	lines = append(lines, "")
	footerStyle := lipgloss.NewStyle().
		Width(sidebarWidth-4).
		Foreground(mutedColor).
		Italic(true).
		Padding(0, 1)
	lines = append(lines, footerStyle.Render("j/k:nav  Enter:select  q:close"))

	content := strings.Join(lines, "\n")
	sidebar := containerStyle.Render(content)

	// Position
	yPos := topMargin
	if config.DockbarPosition == "top" {
		yPos = config.DockHeight
	}

	return lipgloss.NewLayer(sidebar).X(0).Y(yPos).Z(ZIndexSidebar).ID("sidebar")
}

// ToggleSidebar toggles the sidebar visibility
func (m *OS) ToggleSidebar() {
	m.SidebarVisible = !m.SidebarVisible
	if m.SidebarVisible {
		m.SidebarFocused = true
		// Select current focused window in sidebar
		m.SidebarSelectedIndex = m.FocusedWindow
		m.ShowNotification("Sidebar: ↑↓ navigate, Enter select, Esc close", "info", config.NotificationDuration)
	} else {
		m.SidebarFocused = false
		m.SidebarSelectedIndex = -1
	}
}

// SidebarSelectNext moves sidebar selection down
func (m *OS) SidebarSelectNext() {
	if len(m.Windows) == 0 {
		return
	}
	m.SidebarSelectedIndex++
	if m.SidebarSelectedIndex >= len(m.Windows) {
		m.SidebarSelectedIndex = 0 // wrap around
	}
}

// SidebarSelectPrev moves sidebar selection up
func (m *OS) SidebarSelectPrev() {
	if len(m.Windows) == 0 {
		return
	}
	m.SidebarSelectedIndex--
	if m.SidebarSelectedIndex < 0 {
		m.SidebarSelectedIndex = len(m.Windows) - 1 // wrap around
	}
}

// SidebarConfirmSelection switches to the selected window
func (m *OS) SidebarConfirmSelection() {
	if m.SidebarSelectedIndex < 0 || m.SidebarSelectedIndex >= len(m.Windows) {
		return
	}

	selectedWindow := m.Windows[m.SidebarSelectedIndex]

	// Switch workspace if needed
	if selectedWindow.Workspace != m.CurrentWorkspace {
		m.SwitchToWorkspace(selectedWindow.Workspace)
	}

	// Restore if minimized
	if selectedWindow.Minimized {
		m.RestoreWindow(m.SidebarSelectedIndex)
		if m.AutoTiling {
			m.TileAllWindows()
		}
	}

	// Focus the window
	m.FocusWindow(m.SidebarSelectedIndex)

	// Close sidebar and enter terminal mode
	m.SidebarVisible = false
	m.SidebarFocused = false
	m.Mode = TerminalMode
}

// CloseSidebar closes the sidebar
func (m *OS) CloseSidebar() {
	m.SidebarVisible = false
	m.SidebarFocused = false
	m.SidebarSelectedIndex = -1
}

// FindSidebarItemClicked returns the window index if a sidebar item was clicked, -1 otherwise
func (m *OS) FindSidebarItemClicked(x, y int) int {
	if !m.SidebarVisible || len(m.Windows) == 0 {
		return -1
	}

	sidebarWidth := m.GetSidebarWidth()

	// Check if click is within sidebar bounds
	if x >= sidebarWidth {
		return -1
	}

	// Simple approach: just check all windows and return based on click Y
	// The sidebar starts after top margin, has 1 border, title, blank, then items
	topMargin := m.GetTopMargin()
	if config.DockbarPosition == "top" {
		topMargin = config.DockHeight
	}

	// Sidebar content starts at: topMargin + 1 (border) + 1 (title) + 1 (blank) = topMargin + 3
	// Then workspace headers and items
	// For simplicity, just iterate and find by position

	// Group windows by workspace (same as render)
	workspaceWindows := make(map[int][]int)
	for i, w := range m.Windows {
		workspaceWindows[w.Workspace] = append(workspaceWindows[w.Workspace], i)
	}

	workspaces := make([]int, 0, len(workspaceWindows))
	for ws := range workspaceWindows {
		workspaces = append(workspaces, ws)
	}
	sort.Ints(workspaces)

	// Calculate Y positions matching render
	currentY := topMargin + 3 // border + title + blank

	for wsIdx, ws := range workspaces {
		indices := workspaceWindows[ws]
		currentY++ // workspace header

		for _, idx := range indices {
			if y == currentY {
				return idx
			}
			currentY++
		}

		if wsIdx < len(workspaces)-1 {
			currentY++ // gap
		}
	}

	return -1
}

// SidebarHoverZoneWidth is the width of the hover trigger zone on the left edge
const SidebarHoverZoneWidth = 5

// IsSidebarHoverZone checks if coordinates are in the left edge hover zone
func (m *OS) IsSidebarHoverZone(x, _ int) bool {
	// Hover zone is the leftmost N columns when sidebar is hidden
	return !m.SidebarVisible && x < SidebarHoverZoneWidth
}
