package schema

import "fmt"

//go:generate go run ./genregistry

// APIErrorClass is the registry class for a product API error. It is not written on the wire.
type APIErrorClass string

const (
	APIErrorClassValidation       APIErrorClass = "validation"         // 400
	APIErrorClassNotFound         APIErrorClass = "not_found"          // 404
	APIErrorClassConflict         APIErrorClass = "conflict"           // 409
	APIErrorClassMethodNotAllowed APIErrorClass = "method_not_allowed" // 405
	APIErrorClassInternal         APIErrorClass = "internal"           // 500
)

func (c APIErrorClass) String() string { return string(c) }

func (c APIErrorClass) Equals(s string) bool { return string(c) == s }

func (c APIErrorClass) Pointer() *APIErrorClass { return &c }

// FromValue parses s into an APIErrorClass. The error lists valid values.
func (c APIErrorClass) FromValue(s string) (APIErrorClass, error) {
	switch APIErrorClass(s) {
	case APIErrorClassValidation, APIErrorClassNotFound, APIErrorClassConflict,
		APIErrorClassMethodNotAllowed, APIErrorClassInternal:
		return APIErrorClass(s), nil
	default:
		return "", fmt.Errorf("unknown APIErrorClass %q: valid values are %v", s, ValuesAPIErrorClass())
	}
}

// ValuesAPIErrorClass returns every class in registry order.
func ValuesAPIErrorClass() []APIErrorClass {
	return []APIErrorClass{
		APIErrorClassValidation,
		APIErrorClassNotFound,
		APIErrorClassConflict,
		APIErrorClassMethodNotAllowed,
		APIErrorClassInternal,
	}
}

// FromStringAPIErrorClass parses s into an APIErrorClass.
func FromStringAPIErrorClass(s string) (APIErrorClass, error) {
	var z APIErrorClass
	return z.FromValue(s)
}

// APIErrorCode is a registered product error code. The string is the wire value.
type APIErrorCode string

const (
	APIErrorCodeInvalidJSON              APIErrorCode = "HTTP.INVALID_JSON"
	APIErrorCodeValidationFailed         APIErrorCode = "HTTP.VALIDATION_FAILED"
	APIErrorCodeInvalidQuery             APIErrorCode = "HTTP.INVALID_QUERY"
	APIErrorCodeIdempotencyKeyRequired   APIErrorCode = "HTTP.IDEMPOTENCY_KEY_REQUIRED"
	APIErrorCodeInvalidIdempotencyKey    APIErrorCode = "HTTP.INVALID_IDEMPOTENCY_KEY"
	APIErrorCodeIdempotencyKeyConflict   APIErrorCode = "HTTP.IDEMPOTENCY_KEY_CONFLICT"
	APIErrorCodeProfileNotFound          APIErrorCode = "PROFILES.NOT_FOUND"
	APIErrorCodeProfileInvalidDefinition APIErrorCode = "PROFILES.INVALID_DEFINITION"
	APIErrorCodeRunAlreadyRunning        APIErrorCode = "PROFILES.RUN_ALREADY_RUNNING"
	APIErrorCodeNoRun                    APIErrorCode = "PROFILES.NO_RUN"
	APIErrorCodeJobNotInScope            APIErrorCode = "PROFILES.JOB_NOT_IN_SCOPE"
	APIErrorCodeInvalidUserStatus        APIErrorCode = "PROFILES.INVALID_USER_STATUS"
	APIErrorCodeMethodNotAllowed         APIErrorCode = "HTTP.METHOD_NOT_ALLOWED"
	APIErrorCodeUnexpected               APIErrorCode = "INTERNAL.UNEXPECTED"
)

func (c APIErrorCode) String() string { return string(c) }

func (c APIErrorCode) Equals(s string) bool { return string(c) == s }

func (c APIErrorCode) Pointer() *APIErrorCode { return &c }

// FromValue parses s into an APIErrorCode. The error lists valid values.
func (c APIErrorCode) FromValue(s string) (APIErrorCode, error) {
	for _, spec := range errorSpecs {
		if spec.Code.Equals(s) {
			return spec.Code, nil
		}
	}
	return "", fmt.Errorf("unknown APIErrorCode %q: valid values are %v", s, ValuesAPIErrorCode())
}

// ValuesAPIErrorCode returns every code in registry order.
func ValuesAPIErrorCode() []APIErrorCode {
	out := make([]APIErrorCode, len(errorSpecs))
	for i, spec := range errorSpecs {
		out[i] = spec.Code
	}
	return out
}

// FromStringAPIErrorCode parses s into an APIErrorCode.
func FromStringAPIErrorCode(s string) (APIErrorCode, error) {
	var z APIErrorCode
	return z.FromValue(s)
}

// APIErrorSpec is one registered product error. Status and Message are the only wire values besides the code.
type APIErrorSpec struct {
	Code    APIErrorCode
	Class   APIErrorClass
	Status  int
	Message string
}

// APIError is the value handlers and the JSON helpers pass to the writer.
// Cause is logged and is not encoded.
type APIError struct {
	Code  APIErrorCode
	Cause error
}

// Error returns the wire code.
func (e APIError) Error() string { return e.Code.String() }

// Unwrap returns Cause.
func (e APIError) Unwrap() error { return e.Cause }

// errorSpecs is the source for Lookup and the generated code file. Order is the contracts table.
var errorSpecs = []APIErrorSpec{
	{Code: APIErrorCodeInvalidJSON, Class: APIErrorClassValidation, Status: 400, Message: "Request body is not valid JSON."},
	{Code: APIErrorCodeValidationFailed, Class: APIErrorClassValidation, Status: 400, Message: "Request body is invalid."},
	{Code: APIErrorCodeInvalidQuery, Class: APIErrorClassValidation, Status: 400, Message: "Invalid query."},
	{Code: APIErrorCodeIdempotencyKeyRequired, Class: APIErrorClassValidation, Status: 400, Message: "Header Idempotency-Key is required."},
	{Code: APIErrorCodeInvalidIdempotencyKey, Class: APIErrorClassValidation, Status: 400, Message: "Header Idempotency-Key must be a non-nil UUID."},
	{Code: APIErrorCodeIdempotencyKeyConflict, Class: APIErrorClassConflict, Status: 409, Message: "Idempotency key was reused with a different request."},
	{Code: APIErrorCodeProfileNotFound, Class: APIErrorClassNotFound, Status: 404, Message: "Profile not found."},
	{Code: APIErrorCodeProfileInvalidDefinition, Class: APIErrorClassValidation, Status: 400, Message: "Profile definition is invalid."},
	{Code: APIErrorCodeRunAlreadyRunning, Class: APIErrorClassConflict, Status: 409, Message: "A run is already in progress for this profile."},
	{Code: APIErrorCodeNoRun, Class: APIErrorClassNotFound, Status: 404, Message: "No run for this profile."},
	{Code: APIErrorCodeJobNotInScope, Class: APIErrorClassNotFound, Status: 404, Message: "Job not scored for this profile."},
	{Code: APIErrorCodeInvalidUserStatus, Class: APIErrorClassValidation, Status: 400, Message: "Invalid user status."},
	{Code: APIErrorCodeMethodNotAllowed, Class: APIErrorClassMethodNotAllowed, Status: 405, Message: "Method not allowed."},
	{Code: APIErrorCodeUnexpected, Class: APIErrorClassInternal, Status: 500, Message: "Internal server error."},
}

// Lookup returns the spec for a registered code.
func Lookup(code APIErrorCode) (APIErrorSpec, bool) {
	for _, spec := range errorSpecs {
		if spec.Code == code {
			return spec, true
		}
	}
	return APIErrorSpec{}, false
}

// Codes returns wire codes in registry order. The generated JSON file is built from this list.
func Codes() []string {
	codes := make([]string, len(errorSpecs))
	for i, spec := range errorSpecs {
		codes[i] = spec.Code.String()
	}
	return codes
}
