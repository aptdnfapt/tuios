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
	// Line 0: "WINDOWS" header
	// Line 1: blank
	currentY := 2

	for wsIdx, ws := range workspaces {
		indices := workspaceWindows[ws]

		// Workspace header line (e.g., "--- Workspace 1 ---")
		layout.WorkspaceY[ws] = currentY
		currentY++

		// Window items in this workspace (no blank line after workspace header)
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
func (m *OS) renderSidebar() *lipgloss.Layer {
	if !m.SidebarVisible {
		return nil
	}

	sidebarWidth := m.GetSidebarWidth()
	sidebarHeight := m.GetRenderHeight()
	topMargin := m.GetTopMargin()

	// Sidebar container style
	containerStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(sidebarHeight - topMargin).
		Background(lipgloss.Color("#1a1a2e")).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#3a3a5e"))

	// Header style
	headerStyle := lipgloss.NewStyle().
		Width(sidebarWidth-2).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#2a2a4e")).
		Bold(true).
		Padding(0, 1).
		Align(lipgloss.Center)

	// Workspace header style
	workspaceStyle := lipgloss.NewStyle().
		Width(sidebarWidth-2).
		Foreground(lipgloss.Color("#808090")).
		Bold(true).
		Padding(0, 1)

	// Window item base style
	itemBaseStyle := lipgloss.NewStyle().
		Width(sidebarWidth-2).
		Padding(0, 1)

	// Selected item style (keyboard nav highlight)
	selectedStyle := itemBaseStyle.
		Background(lipgloss.Color("#4865f2")).
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	// Focused window style (not selected but is the focused window)
	focusedStyle := itemBaseStyle.
		Background(lipgloss.Color("#3a3a5e")).
		Foreground(lipgloss.Color("#ffffff"))

	// Normal item style
	normalStyle := itemBaseStyle.
		Foreground(lipgloss.Color("#a0a0b0"))

	// Minimized item style
	minimizedStyle := itemBaseStyle.
		Foreground(lipgloss.Color("#606070")).
		Italic(true)

	// Build sidebar content
	var lines []string

	// Header
	lines = append(lines, headerStyle.Render("WINDOWS"))
	lines = append(lines, "") // blank line

	// Group windows by workspace
	workspaceWindows := make(map[int][]int)
	for i, w := range m.Windows {
		workspaceWindows[w.Workspace] = append(workspaceWindows[w.Workspace], i)
	}

	// Get sorted workspace numbers
	workspaces := make([]int, 0, len(workspaceWindows))
	for ws := range workspaceWindows {
		workspaces = append(workspaces, ws)
	}
	sort.Ints(workspaces)

	// Render each workspace section
	for wsIdx, ws := range workspaces {
		indices := workspaceWindows[ws]

		// Workspace header
		wsIndicator := ""
		if ws == m.CurrentWorkspace {
			wsIndicator = " *" // current workspace marker
		}
		wsHeader := fmt.Sprintf("--- Workspace %d%s ---", ws, wsIndicator)
		lines = append(lines, workspaceStyle.Render(wsHeader))

		// Window items in this workspace
		for _, idx := range indices {
			w := m.Windows[idx]

			// Get display name (custom name or title, truncated)
			displayName := w.CustomName
			if displayName == "" {
				displayName = w.Title
			}
			maxNameLen := sidebarWidth - 8 // leave room for number and padding
			if len(displayName) > maxNameLen {
				displayName = displayName[:maxNameLen-3] + "..."
			}

			// Format: "N  title" where N is window number
			windowNum := idx + 1
			itemText := fmt.Sprintf("%d  %s", windowNum, displayName)

			// Add minimized indicator
			if w.Minimized {
				itemText = fmt.Sprintf("%d  [m] %s", windowNum, displayName)
			}

			// Choose style based on state
			var styledItem string
			if m.SidebarFocused && idx == m.SidebarSelectedIndex {
				// Keyboard-selected item (highest priority)
				styledItem = selectedStyle.Render(itemText)
			} else if idx == m.FocusedWindow && w.Workspace == m.CurrentWorkspace {
				// Focused window in current workspace
				styledItem = focusedStyle.Render(itemText)
			} else if w.Minimized {
				// Minimized window
				styledItem = minimizedStyle.Render(itemText)
			} else {
				// Normal window
				styledItem = normalStyle.Render(itemText)
			}

			lines = append(lines, styledItem)
		}

		// Add spacing between workspaces (except last)
		if wsIdx < len(workspaces)-1 {
			lines = append(lines, "")
		}
	}

	// If no windows, show placeholder
	if len(m.Windows) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Width(sidebarWidth-2).
			Foreground(lipgloss.Color("#606070")).
			Italic(true).
			Padding(0, 1).
			Align(lipgloss.Center)
		lines = append(lines, emptyStyle.Render("No windows"))
		lines = append(lines, emptyStyle.Render("Press 'n' to create"))
	}

	// Join all lines
	content := strings.Join(lines, "\n")

	// Apply container style
	sidebar := containerStyle.Render(content)

	// Position sidebar on the left
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
	if !m.SidebarVisible {
		return -1
	}

	sidebarWidth := m.GetSidebarWidth()

	// Check if click is within sidebar bounds
	if x >= sidebarWidth {
		return -1
	}

	// Account for top margin in Y coordinate
	topMargin := m.GetTopMargin()
	if config.DockbarPosition == "top" {
		topMargin = config.DockHeight
	}

	// Adjust Y to be relative to sidebar content
	relativeY := y - topMargin

	layout := m.CalculateSidebarLayout()

	// Check each item position
	for _, item := range layout.ItemPositions {
		if relativeY >= item.StartY && relativeY < item.EndY {
			return item.WindowIndex
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
