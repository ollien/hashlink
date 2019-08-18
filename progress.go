package hashlink

// Progress repressents the progress of something, on a scale of 0-100
type Progress int

// ProgressReporter will report the progress of a process
type ProgressReporter interface {
	// Progress will report the progress of the process
	ReportProgress(progress Progress)
}

// nilProgressReporter will do nothing when it receives a progress
type nilProgressReporter struct{}

// ReportProgress will do absolutely nothing when it receives a progress
func (reporter nilProgressReporter) ReportProgress(progress Progress) {

}
