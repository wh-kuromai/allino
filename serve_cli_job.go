package allino

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	runStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

func styleStatus(status string) string {
	switch status {
	case "done":
		return okStyle.Render(status)
	case "error":
		return errStyle.Render(status)
	case "leased":
		return runStyle.Render(status)
	default: // queued
		return mutedStyle.Render(status)
	}
}

func PrintJobList(jobs []JobInfo) {
	if len(jobs) == 0 {
		fmt.Println(mutedStyle.Render("no jobs"))
		return
	}

	fmt.Printf(
		"%s  %s  %s  %s  %s\n",
		headerStyle.Render("JOB"),
		headerStyle.Render("HANDLER"),
		headerStyle.Render("STATUS"),
		headerStyle.Render("UPDATED"),
		headerStyle.Render("LEASE"),
	)

	for _, j := range jobs {
		lease := "-"
		if j.LeasedUntil != nil {
			lease = j.LeasedUntil.Format("15:04:05")
		}

		fmt.Printf(
			"%d  %s  %s  %s  %s\n",
			jobIDtoJobNum(j.JobID),
			j.Handler,
			styleStatus(j.Meta.Status),
			j.UpdatedAt.Format("15:04:05"),
			lease,
		)
	}
}

var lastjobID = 0
var jobIDMap map[string]int

func jobIDtoJobNum(id string) int {
	num, ok := jobIDMap[id]
	if ok {
		return num
	}

	lastjobID++
	jobIDMap[id] = lastjobID
	return lastjobID
}
