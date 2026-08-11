package errors

type ErrorLevel string

const (
	ErrorLevelWarning  ErrorLevel = "WARNING"
	ErrorLevelUrgent   ErrorLevel = "URGENT"
	ErrorLevelCritical ErrorLevel = "CRITICAL"
	ErrorLevelFatal    ErrorLevel = "FATAL"
)

type ErrorType string

const (
	ErrorTypeValidationFailure ErrorType = "VALIDATION_FAILURE"
	ErrorTypeInvalidOrder      ErrorType = "INVALID_ORDER"
)
