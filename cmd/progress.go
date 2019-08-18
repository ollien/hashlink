package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ollien/hashlink"
)

const (
	progressBarLength = 20
	progressBarFormat = "[%s] %d%%"
)

// progressBarReporter implements hashlink.ProgressReporter and will print a progress bar to stderr
type progressBarReporter struct{}

// ReportProress will print a progress bar to stderr
func (reporter progressBarReporter) ReportProgress(progress hashlink.Progress) {
	equalsSigns := ""
	lastEqualsSignPosition := int(progressBarLength * float64(progress) / 100)
	for i := 0; i < progressBarLength; i++ {
		if i < lastEqualsSignPosition {
			equalsSigns += "="
		} else {
			equalsSigns += " "
		}
	}

	progressBar := fmt.Sprintf(progressBarFormat, equalsSigns, progress)
	fmt.Fprintf(os.Stderr, "\r%s", progressBar)
}

func (reporter progressBarReporter) finish() {
	fullBar := strings.Repeat("=", progressBarLength)
	progressBar := fmt.Sprintf(progressBarFormat, fullBar, 100)
	fmt.Fprintf(os.Stderr, "\r%s\n", progressBar)
}
