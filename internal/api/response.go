package api

// Response is the envelope every api-gateway endpoint answers with.
type Response[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}
