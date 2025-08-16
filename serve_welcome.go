package allino

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (s *Server) showWelcome(bindURL string) {
	// スタイル定義
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF79C6")).
		//Background(lipgloss.Color("#1E1E1E")).
		Padding(0, 1).
		Bold(true)

	urlStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#dededeff")).
		//Background(lipgloss.Color("#1E1E1E")).
		Padding(0, 1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8a8a8aff")).
		Padding(1, 2).
		Align(lipgloss.Center)

		// コンテンツを組み立て
	title := s.Config.AppName
	if s.Config.Description != "" {
		title += " - " + s.Config.Description
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(title),
		urlStyle.Render(""),
		urlStyle.Render("Running at: "+bindURL),
	)

	// 全体をボックスで囲む
	final := boxStyle.Render(content)

	// 出力
	fmt.Println(final)
}
