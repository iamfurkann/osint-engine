package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iamfurkann/osint-engine/internal/ipc"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58a6ff")).MarginBottom(1)
	itemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9"))
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#da3633"))
)

type tickMsg time.Time

type model struct {
	ipcClient      *ipc.Client
	investigations []map[string]interface{}
	err            error
	width          int
	height         int
}

// StartTUI, bubbletea arayüzünü başlatır.
func StartTUI(ipcSocketPath string) error {
	m := &model{
		ipcClient: ipc.NewClient(ipcSocketPath),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.fetchData(), tickCmd())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			return m, m.fetchData()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tea.Batch(m.fetchData(), tickCmd())

	case dataMsg:
		m.investigations = msg.data
		m.err = nil

	case errMsg:
		m.err = msg.err
	}

	return m, nil
}

func (m *model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Hata: %v\n\n(Çıkmak için 'q' tuşuna basın)", m.err))
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("OSINT-Engine Canlı İzleme (TUI)"))
	b.WriteString("\n")

	if len(m.investigations) == 0 {
		b.WriteString(infoStyle.Render("Aktif veya tamamlanmış araştırma bulunmuyor.\n"))
	} else {
		// Tablo başlığı
		b.WriteString(fmt.Sprintf("%-15s %-25s %-15s %-10s\n", "ID", "HEDEF", "DURUM", "İLERLEME"))
		b.WriteString(strings.Repeat("─", 70) + "\n")

		for _, inv := range m.investigations {
			id := inv["id"].(string)
			if len(id) > 12 {
				id = id[:12] + "..."
			}

			target := inv["target"].(string)
			if len(target) > 22 {
				target = target[:22] + "..."
			}

			status := inv["status"].(string)
			progress := inv["progress"].(float64)

			statusColor := "#8b949e"
			switch status {
			case "running":
				statusColor = "#58a6ff"
			case "completed":
				statusColor = "#238636"
			case "failed", "cancelled":
				statusColor = "#da3633"
			}

			renderedStatus := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(fmt.Sprintf("%-15s", status))

			line := fmt.Sprintf("%-15s %-25s %s %5.1f%%\n", id, target, renderedStatus, progress)
			b.WriteString(itemStyle.Render(line))
		}
	}

	b.WriteString("\n\n" + infoStyle.Render("Yenile: 'r' • Çıkış: 'q'"))
	return b.String()
}

// --- Komutlar ve Mesajlar ---

type dataMsg struct {
	data []map[string]interface{}
}

type errMsg struct {
	err error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) fetchData() tea.Cmd {
	return func() tea.Msg {
		if !m.ipcClient.IsRunning() {
			return errMsg{fmt.Errorf("daemon çalışmıyor. osintd start ile başlatın")}
		}

		res, err := m.ipcClient.Call("investigation.list", nil)
		if err != nil {
			return errMsg{err}
		}

		var list []map[string]interface{}
		if err := json.Unmarshal(res, &list); err != nil {
			return errMsg{err}
		}

		return dataMsg{data: list}
	}
}
