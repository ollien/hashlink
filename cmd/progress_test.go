package main

import (
	"testing"

	"github.com/ollien/hashlink"
	"github.com/stretchr/testify/assert"
)

// staticReporter will hold the last progress that has been reported to it
type staticReporter struct {
	lastReportedProgress hashlink.Progress
}

func (reporter *staticReporter) ReportProgress(progress hashlink.Progress) {
	reporter.lastReportedProgress = progress
}

func TestProgressReporterAggregator(t *testing.T) {
	reporter := &staticReporter{}
	aggregator := newProgressReporterAggregator(reporter, 4)
	subreporters := make([]subAggregateProgressReporter, 4)
	for i := range subreporters {
		subreporters[i] = newSubAggregateProgressReporter(aggregator)
	}

	subreporters[0].ReportProgress(16)
	// Because we have 4 expected reporters, we expect that the reported progress should be 4 (16/4 = 4).
	assert.Equal(t, hashlink.Progress(4), reporter.lastReportedProgress)

	subreporters[1].ReportProgress(16)
	// Because we have 4 expected reporters, we expect that the reported progress should be 8 (32/4 = 8).
	assert.Equal(t, hashlink.Progress(8), reporter.lastReportedProgress)

	subreporters[2].ReportProgress(16)
	// Because we have 4 expected reporters, we expect that the reported progress should be 12 (48/4 = 12).
	assert.Equal(t, hashlink.Progress(12), reporter.lastReportedProgress)

	subreporters[3].ReportProgress(16)
	// Because we have 4 expected reporters, we expect that the reported progress should be 16 (64/4 = 16).
	assert.Equal(t, hashlink.Progress(16), reporter.lastReportedProgress)

	// Now that we're adding another subreporter into the mix, we expect that the entire result should be divided by 5, rather than 4.
	subreporters = append(subreporters, newSubAggregateProgressReporter(aggregator))
	subreporters[4].ReportProgress(6)
	// Because we have 5 reporters, we expect that the reported progress should be 14 (70/5 = 14).
	assert.Equal(t, hashlink.Progress(14), reporter.lastReportedProgress)
}
