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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type specialError struct {
	magicNumber int
}

func (err specialError) Error() string {
	return fmt.Sprintf("there was an error with magic number %d", err.magicNumber)
}

type multiErrorTest struct {
	name  string
	setup func() *MultiError
	test  func(t *testing.T, multiError *MultiError)
}

func runMultiErrorTestTable(t *testing.T, table []multiErrorTest) {
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			multiError := tt.setup()
			tt.test(t, multiError)
		})
	}
}

func TestMultiError_Error(t *testing.T) {
	tests := []multiErrorTest{
		{
			name: "no errors",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, "", multiError.Error())
			},
		},
		{
			name: "one error",
			setup: func() *MultiError {
				return NewMultiError(errors.New("something broke :("))
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, "something broke :(", multiError.Error())
			},
		},
		{
			name: "several errors",
			setup: func() *MultiError {
				errors := []error{
					errors.New("something broke :("),
					errors.New("aw shucks"),
					errors.New("this is bad"),
				}

				return NewMultiError(errors...)
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, "something broke :(; aw shucks; this is bad", multiError.Error())
			},
		},
	}

	runMultiErrorTestTable(t, tests)
}

func TestMultiError_Append(t *testing.T) {
	tests := []multiErrorTest{
		{
			name: "append error",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, 0, len(multiError.Errors()))
				multiError.Append(errors.New("something broke :("))
				assert.Equal(t, 1, len(multiError.Errors()))
				assert.Equal(t, "something broke :(", multiError.Error())
			},
		},
		{
			name: "append nil error",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, 0, multiError.Len())
				multiError.Append(error(nil))
				assert.Equal(t, 0, multiError.Len())
			},
		},
	}

	runMultiErrorTestTable(t, tests)
}

func TestMultiError_Errors(t *testing.T) {
	tests := []multiErrorTest{
		{
			name: "no errors",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, []error{}, multiError.Errors())
			},
		},
		{
			name: "some errors",
			setup: func() *MultiError {
				return NewMultiError(errors.New("an error"), errors.New("something broke :("))
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, []error{
					errors.New("an error"),
					errors.New("something broke :("),
				}, multiError.Errors())
			},
		},
		{
			name: "ensure no mutation",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				specialErrors := []specialError{
					{5},
					{23},
					{42},
				}

				for _, err := range specialErrors {
					multiError.Append(err)
				}

				for i, err := range multiError.Errors() {
					assert.Equal(t, specialErrors[i], err)
				}
			},
		},
	}

	runMultiErrorTestTable(t, tests)
}
