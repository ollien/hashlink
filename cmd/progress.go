package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/ollien/hashlink"
)

const (
	progressBarLength = 20
	progressBarFormat = "[%s] %d%%"
)

// progressBarReporter implements hashlink.ProgressReporter and will print a progress bar to stderr
type progressBarReporter struct{}

// progressReporterAggregator will send aggregate progress to a base reporter.
type progressReporterAggregator struct {
	// progressLock will be held when progress is being reported
	// cannot use a sync.Map due to needing to ensure we calculate progress at reporting time, rather than between reports
	progressLock sync.Mutex
	// reportedProgresses will store all of the progresses received from other progress reporters
	reportedProgresses map[uuid.UUID]hashlink.Progress
	// baseReporter represents the reporter we will be sending our progress to
	baseReporter hashlink.ProgressReporter
	// expectedLength represents the number of elements we're expecting in reportedProgresses.
	expectedLength int
}

// subAgregateProgressReporter will report any proress to its parent progressReportAggregator
type subAggregateProgressReporter struct {
	id     uuid.UUID
	parent *progressReporterAggregator
}

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

// when abort is called, we will give a carriage return and overwrite the progress bar, as there was an error
func (reporter progressBarReporter) abort() {
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", progressBarLength))
}

// newProgressReporterAggregator will make an aggregate proress reporter for the given reporter and length
func newProgressReporterAggregator(baseReporter hashlink.ProgressReporter, expectedLength int) *progressReporterAggregator {
	return &progressReporterAggregator{
		expectedLength:     expectedLength,
		baseReporter:       baseReporter,
		reportedProgresses: make(map[uuid.UUID]hashlink.Progress, expectedLength),
	}
}

// reportSubProgress will take a progress and report the normalized progress to the base reporter
func (aggregator *progressReporterAggregator) reportSubProgress(id uuid.UUID, subprogress hashlink.Progress) {
	aggregator.progressLock.Lock()
	defer aggregator.progressLock.Unlock()

	aggregator.reportedProgresses[id] = subprogress
	totalProgress := hashlink.Progress(0)
	for _, progress := range aggregator.reportedProgresses {
		totalProgress += progress
	}

	length := len(aggregator.reportedProgresses)
	if length < aggregator.expectedLength {
		length = aggregator.expectedLength
	}

	// make sure we don't divide by zero with our length, so set 0% as the default
	normalizedProgress := hashlink.Progress(0)
	if length > 0 {
		normalizedProgress = hashlink.Progress(int(totalProgress) / length)
	}

	aggregator.baseReporter.ReportProgress(normalizedProgress)
}

func newSubAggregateProgressReporter(aggregator *progressReporterAggregator) subAggregateProgressReporter {
	return subAggregateProgressReporter{
		id:     uuid.New(),
		parent: aggregator,
	}
}

// ReportProgress wil report the given progress to the parent aggregator
func (reporter subAggregateProgressReporter) ReportProgress(progress hashlink.Progress) {
	reporter.parent.reportSubProgress(reporter.id, progress)
}
