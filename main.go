package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const listHeight = 14

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

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
	stepDone
)

type config struct {
	InstancesDir    string `json:"instancesDir"`
	InstanceName    string `json:"instanceName"`
	SelectedVersion string `json:"selectedVersion"`
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

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
					m.choice = fmt.Sprintf("Instance: %s\nVersion: %s", m.selectedInstance, m.selectedVersion)
				}
				m.step = stepDone
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	if m.step == stepPromptPath {
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
	case stepListInstances, stepPickVersion:
		return "\n" + m.list.View()
	case stepDone:
		return quitTextStyle.Render(m.choice)
	}
	return ""
}

func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "gtnh-updater-cli")
	return filepath.Join(appDir, "config.json"), nil
}

func loadConfig() (*config, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func listDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

func fetchAvailableVersions() ([]string, error) {
	resp, err := http.Get("https://downloads.gtnewhorizons.com/Multi_mc_downloads/?raw")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	var results []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "Java_17-21") {
			continue
		}
		results = append(results, trimmed)
	}
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results, nil
}

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
				// preselect previously chosen instance if exists
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
