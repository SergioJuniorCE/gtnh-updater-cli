package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	const defaultWidth = 40

	l := list.New([]list.Item{}, itemDelegate{}, defaultWidth, listHeight)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	ti := textinput.New()
	ti.Placeholder = "C:\\Users\\<you>\\MultiMC\\instances"
	ti.CharLimit = 0
	ti.Width = 60

	initialStep := stepPromptPath
	if cfg, err := loadConfig(); err == nil && cfg != nil && cfg.InstancesDir != "" {
		if _, statErr := os.Stat(cfg.InstancesDir); statErr == nil {
			dirs, derr := listDirectories(cfg.InstancesDir)
			if derr == nil {
				items := make([]list.Item, 0, len(dirs))
				for _, d := range dirs {
					items = append(items, item(d))
				}
				l.SetItems(items)
				l.Title = "Select instance folder"
				initialStep = stepListInstances
				ti.SetValue(cfg.InstancesDir)
				if cfg.InstanceName != "" {
					for idx, it := range l.Items() {
						if string(it.(item)) == cfg.InstanceName {
							l.Select(idx)
							break
						}
					}
				}
			}
		}
	}

	if initialStep == stepPromptPath {
		ti.Focus()
	}

	m := model{list: l, text: ti, step: initialStep}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
