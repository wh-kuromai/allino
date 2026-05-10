package allino

import (
	"bufio"
	"fmt"
	"os"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
)

type CLIApp struct {
	program *tea.Program

	stdout *os.File
	piper  *os.File
	pipew  *os.File
}

const zLOGLINES = 8

func NewCLIApp() *CLIApp {
	m := model{
		progress: progress.New(progress.WithFillCharacters('━', '━')),

		logs:  make([]string, zLOGLINES),
		logCh: make(chan string),
	}
	pb := &CLIApp{}

	pb.program = tea.NewProgram(m)

	pb.stdout = os.Stdout
	pb.piper, pb.pipew, _ = os.Pipe()
	os.Stdout = pb.pipew

	go pb.program.Run() // 非同期で起動

	go func() {
		scanner := bufio.NewScanner(pb.piper)
		for scanner.Scan() {
			line := scanner.Text()
			m.logCh <- line
		}
	}()

	return pb
}

type progressMsg struct {
	percent float64
	current int
	total   int
}

func (p *CLIApp) Progress(percent float64, current, total int) {
	p.program.Send(progressMsg{
		percent: percent,
		current: current,
		total:   total,
	})
}

func (p *CLIApp) Close() {
	p.program.Send(closeMsg{})
	os.Stdout = p.stdout
}

// --- Bubble Tea 内部 ---------------------

type closeMsg struct{}

type model struct {
	progress progress.Model
	current  int
	total    int

	logidx int
	logs   []string
	logCh  chan string
}

func waitForLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		return logMsg(<-ch)
	}
}

func (m model) Init() tea.Cmd {
	return waitForLog(m.logCh)
}

type logMsg string

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case logMsg:
		m.logs[m.logidx] = string(msg)
		m.logidx++
		if m.logidx >= zLOGLINES {
			m.logidx = 0
		}
		return m, waitForLog(m.logCh)

	case progressMsg:
		m.current = msg.current
		m.total = msg.total
		cmd := m.progress.SetPercent(msg.percent)
		return m, cmd

	case closeMsg:
		return m, tea.Quit

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() tea.View {
	bar := m.progress.View()
	info := fmt.Sprintf(" (%d/%d)",
		m.current,
		m.total,
	)

	log := ""
	for i := 0; i < zLOGLINES; i++ {
		j := i + m.logidx
		if j >= zLOGLINES {
			j -= zLOGLINES
		}
		log += m.logs[j] + "\n"
	}

	return tea.NewView("\n" + bar + info + "\n" + log)
}
