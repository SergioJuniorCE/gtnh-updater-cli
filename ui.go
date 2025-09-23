package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	list             list.Model
	text             textinput.Model
	choice           string
	selectedInstance string
	selectedVersion  string
	quitting         bool
	step             int
}

const (
	stepPromptPath = iota
	stepListInstances
	stepPickVersion
	stepPromptDest
	stepDone
)

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		switch m.step {
		case stepPromptPath:
			if msg.String() == "enter" {
				path := strings.TrimSpace(m.text.Value())
				if path == "" {
					break
				}
				info, err := os.Stat(path)
				if err != nil || !info.IsDir() {
					m.text.SetValue("")
					m.text.Placeholder = "Invalid path. Try again."
					break
				}
				cfg := &config{InstancesDir: path}
				_ = saveConfig(cfg)
				dirs, err := listDirectories(path)
				if err != nil {
					m.text.Placeholder = "Failed to read directory. Try another."
					break
				}
				items := make([]list.Item, 0, len(dirs))
				for _, d := range dirs {
					items = append(items, item(d))
				}
				m.list.SetItems(items)
				m.list.Title = "Select instance folder"
				m.step = stepListInstances
				return m, nil
			}
		case stepListInstances:
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				i, ok := m.list.SelectedItem().(item)
				if !ok {
					break
				}
				m.selectedInstance = string(i)
				if cfg, err := loadConfig(); err == nil && cfg != nil {
					cfg.InstanceName = m.selectedInstance
					_ = saveConfig(cfg)
				} else {
					_ = saveConfig(&config{InstancesDir: m.text.Value(), InstanceName: m.selectedInstance})
				}
				versions, err := fetchAvailableVersions()
				if err != nil || len(versions) == 0 {
					m.list.SetItems([]list.Item{item("No versions available")})
					m.list.Title = "Pick GTNH version"
					m.step = stepPickVersion
					return m, nil
				}
				vitems := make([]list.Item, 0, len(versions))
				for _, v := range versions {
					vitems = append(vitems, item(v))
				}
				m.list.SetItems(vitems)
				m.list.Title = "Pick GTNH version"
				m.step = stepPickVersion
				return m, nil
			}
		case stepPickVersion:
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				i, ok := m.list.SelectedItem().(item)
				if ok {
					m.selectedVersion = string(i)
					if cfg, err := loadConfig(); err == nil && cfg != nil {
						cfg.SelectedVersion = m.selectedVersion
						_ = saveConfig(cfg)
					} else {
						_ = saveConfig(&config{InstancesDir: m.text.Value(), InstanceName: m.selectedInstance, SelectedVersion: m.selectedVersion})
					}
					// proceed to destination prompt (will backup automatically before migrating)
					m.text.SetValue("")
					m.text.Placeholder = "Name for NEW GTNH instance (folder under instances dir)"
					m.step = stepPromptDest
					return m, m.text.Focus()
				}
				return m, nil
			}
		case stepPromptDest:
			if msg.String() == "enter" {
				name := strings.TrimSpace(m.text.Value())
				if name == "" {
					break
				}
				if strings.ContainsAny(name, "\\/:*?\"<>|") {
					m.text.SetValue("")
					m.text.Placeholder = "Invalid name. Avoid path separators/special chars."
					break
				}
				cfg, _ := loadConfig()
				base := ""
				if cfg != nil && cfg.InstancesDir != "" {
					base = cfg.InstancesDir
				} else {
					m.text.SetValue("")
					m.text.Placeholder = "Instances directory not set. Restart."
					break
				}
				dest := filepath.Join(base, name)
				created := false
				info, err := os.Stat(dest)
				if os.IsNotExist(err) {
					if err := os.MkdirAll(dest, 0o755); err != nil {
						m.text.SetValue("")
						m.text.Placeholder = "Failed to create folder. Try another name."
						break
					}
					created = true
				} else if err != nil {
					m.text.SetValue("")
					m.text.Placeholder = "Cannot access destination folder. Try another name."
					break
				} else if !info.IsDir() {
					m.text.SetValue("")
					m.text.Placeholder = "A file with that name exists. Try another name."
					break
				}
				source := filepath.Join(base, m.selectedInstance)
				// backup only if destination already existed
				if !created {
					if _, err := backupInstanceDir(dest); err != nil {
						m.choice = fmt.Sprintf("Backup failed: %v", err)
						m.step = stepDone
						return m, tea.Quit
					}
				}
				if err := migrateInstance(source, dest); err != nil {
					m.choice = fmt.Sprintf("Migration failed: %v", err)
					m.step = stepDone
					return m, tea.Quit
				}
				_ = writeMigrationTips(dest)
				m.choice = fmt.Sprintf("Migrated '%s' to\n%s\nSelected version: %s", m.selectedInstance, dest, m.selectedVersion)
				m.step = stepDone
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	if m.step == stepPromptPath || m.step == stepPromptDest {
		m.text, cmd = m.text.Update(msg)
		return m, cmd
	}
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	switch m.step {
	case stepPromptPath:
		return "\n" + titleStyle.Render("Enter your instances folder path:") + "\n\n  " + m.text.View() + "\n\n  Press Enter to continue"
	case stepPromptDest:
		return "\n" + titleStyle.Render("Enter destination instance path:") + "\n\n  " + m.text.View() + "\n\n  Press Enter to migrate"
	case stepListInstances, stepPickVersion:
		return "\n" + m.list.View()
	case stepDone:
		return quitTextStyle.Render(m.choice)
	}
	return ""
}
