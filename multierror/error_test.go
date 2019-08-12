package multierror

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
		multiErrorTest{
			name: "no errors",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, "", multiError.Error())
			},
		},
		multiErrorTest{
			name: "one error",
			setup: func() *MultiError {
				return NewMultiError(errors.New("something broke :("))
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, "something broke :(", multiError.Error())
			},
		},
		multiErrorTest{
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
		multiErrorTest{
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
		multiErrorTest{
			name: "append nil error",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, 0, len(multiError.Errors()))
				multiError.Append(error(nil))
				assert.Equal(t, 0, len(multiError.Errors()))
			},
		},
	}

	runMultiErrorTestTable(t, tests)
}

func TestMultiError_Errors(t *testing.T) {
	tests := []multiErrorTest{
		multiErrorTest{
			name: "no errors",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				assert.Equal(t, []error{}, multiError.Errors())
			},
		},
		multiErrorTest{
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
		multiErrorTest{
			name: "ensure no mutation",
			setup: func() *MultiError {
				return NewMultiError()
			},
			test: func(t *testing.T, multiError *MultiError) {
				specialErrors := []specialError{
					specialError{5},
					specialError{23},
					specialError{42},
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
