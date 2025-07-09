package errors

import "fmt"

type ReasonedError struct {
	reason  string
	message string
}

func (e *ReasonedError) Error() string {
	return e.message
}

func (e *ReasonedError) Reason() string {
	return e.reason
}

func NewReasonedError(reason, format string, a ...interface{}) error {
	message := fmt.Sprintf(format, a...)
	return &ReasonedError{
		reason:  reason,
		message: message,
	}
}
