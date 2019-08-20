package multierror

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

import (
	"strings"
	"sync"
)

// MultiError represents a set of errors that occured. Implements the error interface.
type MultiError struct {
	errors    []error
	errorLock sync.RWMutex
}

// NewMultiError will make a MultiError with the given errors
func NewMultiError(errors ...error) *MultiError {
	errorList := errors
	if len(errorList) == 0 {
		errorList = make([]error, 0)
	}

	return &MultiError{
		errors: errorList,
	}
}

// Error will concatenate multiple errors
func (multiError *MultiError) Error() string {
	multiError.errorLock.RLock()
	defer multiError.errorLock.RUnlock()

	errorStrings := make([]string, len(multiError.errors))
	for i, err := range multiError.errors {
		errorStrings[i] = err.Error()
	}

	return strings.Join(errorStrings, "; ")
}

// Append will append an error to the list of errors within the MultiError.
// If a nil error is passed, it will be ignored.
func (multiError *MultiError) Append(err error) {
	if err == nil {
		return
	}

	multiError.errorLock.Lock()
	defer multiError.errorLock.Unlock()

	multiError.errors = append(multiError.errors, err)
}

// Errors will get all errors in the form they were passed in, rather than as a single error.
func (multiError *MultiError) Errors() []error {
	multiError.errorLock.RLock()
	defer multiError.errorLock.RUnlock()

	copiedErrors := make([]error, len(multiError.errors))
	copy(copiedErrors, multiError.errors)

	return copiedErrors
}

// Len will return the number of errors currently contained within the MultiError
func (multiError *MultiError) Len() int {
	multiError.errorLock.RLock()
	defer multiError.errorLock.RUnlock()

	return len(multiError.errors)
}
