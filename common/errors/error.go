package vdaerrors

type ErrorLevel string

const (
	ErrorLevelWarning  ErrorLevel = "WARNING"
	ErrorLevelUrgent   ErrorLevel = "URGENT"
	ErrorLevelCritical ErrorLevel = "CRITICAL"
	ErrorLevelFatal    ErrorLevel = "FATAL"
)

type ErrorType string

const (
	ErrorTypeValidationFailure          ErrorType = "VALIDATION_FAILURE"
	ErrorTypeInvalidOrder               ErrorType = "INVALID_ORDER"
	ErrorTypeOtherOrderActive           ErrorType = "OTHER_ORDER_ACTIVE"
	ErrorTypeStartNodeOutOfRange        ErrorType = "START_NODE_OUT_OF_RANGE"
	ErrorTypeOutdatedOrderUpdate        ErrorType = "OUTDATED_ORDER_UPDATE"
	ErrorTypeOrderUpdateFollowingCancel ErrorType = "ORDER_UPDATE_FOLLOWING_CANCEL"
	ErrorTypeSameOrderUpdateId          ErrorType = "SAME_ORDER_UPDATE_ID"
	ErrorTypeInvalidInstantAction       ErrorType = "INVALID_INSTANT_ACTION"
)

var ErrorTypeLevel = map[ErrorType]ErrorLevel{
	ErrorTypeValidationFailure:          ErrorLevelWarning,
	ErrorTypeInvalidOrder:               ErrorLevelWarning,
	ErrorTypeOtherOrderActive:           ErrorLevelWarning,
	ErrorTypeStartNodeOutOfRange:        ErrorLevelWarning,
	ErrorTypeOutdatedOrderUpdate:        ErrorLevelWarning,
	ErrorTypeOrderUpdateFollowingCancel: ErrorLevelWarning,
	ErrorTypeSameOrderUpdateId:          ErrorLevelWarning,
	ErrorTypeInvalidInstantAction:       ErrorLevelWarning,
}

type ErrorReference struct {
	ReferenceKey   string `json:"referenceKey"`
	ReferenceValue string `json:"referenceValue"`
}

type ErrorDescriptionTranslation struct {
	TranslationKey   string `json:"translationKey"`
	TranslationValue string `json:"translationValue"`
}

type VDAError struct {
	ErrorType                    ErrorType                     `json:"errorType"`
	ErrorLevel                   ErrorLevel                    `json:"errorLevel"`
	ErrorReferences              []ErrorReference              `json:"errorReferences"`
	ErrorDescription             string                        `json:"errorDescription"`
	ErrorHint                    string                        `json:"errorHint"`
	ErrorDescriptionTranslations []ErrorDescriptionTranslation `json:"errorDescriptionTranslations"`
}

func (e *VDAError) Error() string {
	return string(e.ErrorType) + ": " + e.ErrorDescription
}

func New(errType ErrorType, description string, ref ...ErrorReference) *VDAError {
	errLevel := ErrorTypeLevel[errType]
	return &VDAError{
		ErrorType:        errType,
		ErrorLevel:       errLevel,
		ErrorDescription: description,
		ErrorReferences:  ref,
	}
}
