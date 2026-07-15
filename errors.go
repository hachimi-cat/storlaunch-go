package storlaunch

import "fmt"

// Error is the single error type returned by every operation in this
// package. Wraps an opaque code plus a message so callers can branch on
// well-defined error reasons without matching strings.
//
// Mirrors `StorlaunchError` in the Node and Python SDKs:
//
//   - Status: HTTP status code, or 0 for client-side errors (timeout,
//     network, malformed response).
//   - Code: short machine-readable code from the API envelope's
//     `error.code`, or a local code like "timeout" / "network_error".
//   - Message: human-readable detail.
//   - RequestID: optional `meta.requestId` from the API envelope.
type Error struct {
	Status    int
	Code      string
	Message   string
	RequestID string
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("storlaunch: %s: %s (request_id=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("storlaunch: %s: %s", e.Code, e.Message)
}

func newErr(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}
