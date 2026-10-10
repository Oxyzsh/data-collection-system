package errors

import "fmt"

// AppError 表示可透传到接口层的业务错误。
type AppError struct {
	Code    string
	Key     string
	Args    map[string]interface{}
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
}

func New(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func NewWithKey(code, key string, args map[string]interface{}) *AppError {
	return &AppError{Code: code, Key: key, Args: args, Message: key}
}

func WrapWithKey(code, key string, args map[string]interface{}, cause error) *AppError {
	return &AppError{Code: code, Key: key, Args: args, Message: key, Cause: cause}
}
