package agentruntime

import (
	"errors"
	"strings"
)

type ErrorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Retryable bool   `json:"retryable"`
}

func NewErrorEnvelope(code string, message string, source string, retryable bool) ErrorEnvelope {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "runtime.error"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "runtime error"
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "agent-runtime"
	}
	return ErrorEnvelope{Code: code, Message: message, Source: source, Retryable: retryable}
}

func ErrorEnvelopeFromStatus(status string, source string, err error) ErrorEnvelope {
	message := ""
	if err != nil {
		message = err.Error()
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "failed"
	}
	switch status {
	case "planning_failed":
		return NewErrorEnvelope("runtime.planning_failed", message, valueOrSource(source, "planner"), true)
	case "tool_failed":
		return NewErrorEnvelope("runtime.tool_failed", message, valueOrSource(source, "tool-executor"), true)
	case "preflight_failed":
		return NewErrorEnvelope("runtime.preflight_failed", message, valueOrSource(source, "tool-preflight"), false)
	case "failed":
		return NewErrorEnvelope("runtime.failed", message, source, true)
	default:
		return NewErrorEnvelope("runtime."+strings.ReplaceAll(status, " ", "_"), message, source, true)
	}
}

func ErrorEnvelopeFromHTTPStatus(status int, message string) ErrorEnvelope {
	code := "gateway.error"
	retryable := status >= 500
	switch {
	case status == 400:
		code = "gateway.bad_request"
	case status == 401 || status == 403:
		code = "gateway.unauthorized"
	case status == 404:
		code = "gateway.not_found"
	case status == 405:
		code = "gateway.method_not_allowed"
	case status >= 500:
		code = "gateway.internal_error"
	}
	return NewErrorEnvelope(code, message, "gateway", retryable)
}

func valueOrSource(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func ErrorEnvelopeMessage(envelope *ErrorEnvelope, fallback string) string {
	if envelope == nil {
		return fallback
	}
	if strings.TrimSpace(envelope.Message) != "" {
		return envelope.Message
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return envelope.Code
}

func ErrorEnvelopeFromError(code string, source string, err error) ErrorEnvelope {
	if err == nil {
		err = errors.New("runtime error")
	}
	return NewErrorEnvelope(code, err.Error(), source, true)
}
