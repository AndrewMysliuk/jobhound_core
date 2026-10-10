package schema

// APIErrorBody is the standard JSON error envelope for non-2xx responses (spec.md).
type APIErrorBody struct {
	Error APIErrorDetail `json:"error"`
}

// APIErrorDetail is the nested error object.
type APIErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
