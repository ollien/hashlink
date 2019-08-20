package hashlink

/*
	Copyright 2019 Nicholas Krichevsky

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

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
