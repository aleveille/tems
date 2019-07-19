package error

import (
	"fmt"
	"time"
)

// AppError is an error implementation that includes a time, a description message and a cause error
type AppError struct {
	When        time.Time
	Description string
	Cause       error
}

// NewAppError return a new AppError at the current time with the given description and cause error
func NewAppError(description string, cause error) AppError {
	return AppError{When: time.Now(), Description: description, Cause: cause}
}

func (e AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v\n\tCaused by: %+v", e.When.Format("2006-01-02 15:04:05"), e.Description, e.Cause)
	}
	return fmt.Sprintf("%s: %v", e.When.Format("2006-01-02 15:04:05"), e.Description)
}

// Unwrap returns the result of calling the Unwrap method on err, if err implements Unwrap.
// Otherwise, Unwrap returns nil.
func (e AppError) Unwrap(err error) error {
	return e.Cause
}
