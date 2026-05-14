package protocol

import "net/http"

const (
	TypeRegister   = "register"
	TypeRegistered = "registered"
	TypeRequest    = "request"
	TypeResponse   = "response"
	TypeError      = "error"
)

const DefaultMaxBodyBytes int64 = 10 << 20

type Message struct {
	Type     string           `json:"type"`
	ID       string           `json:"id,omitempty"`
	Name     string           `json:"name,omitempty"`
	Token    string           `json:"token,omitempty"`
	Error    string           `json:"error,omitempty"`
	Request  *RequestPayload  `json:"request,omitempty"`
	Response *ResponsePayload `json:"response,omitempty"`
}

type RequestPayload struct {
	Method   string      `json:"method"`
	Path     string      `json:"path"`
	RawQuery string      `json:"raw_query,omitempty"`
	Header   http.Header `json:"header,omitempty"`
	Body     []byte      `json:"body,omitempty"`
}

type ResponsePayload struct {
	StatusCode int         `json:"status_code"`
	Header     http.Header `json:"header,omitempty"`
	Body       []byte      `json:"body,omitempty"`
}
