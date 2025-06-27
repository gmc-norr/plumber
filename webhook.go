package plumber

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type Webhook struct {
	http.Client
	URL            string
	APIKey         string
	HeaderKey      string
	Method         string
	PlumberVersion string
}

type MarshableError struct {
	error
}

type MessageType int

const (
	MessageInit MessageType = iota
	MessageStart
	MessageProgress
	MessageEnd
)

func (t MessageType) String() string {
	switch t {
	case MessageInit:
		return "init"
	case MessageStart:
		return "start"
	case MessageProgress:
		return "progress"
	case MessageEnd:
		return "end"
	}
	return "undefined"
}

func (t MessageType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func NewMarshableError(err error) MarshableError {
	return MarshableError{
		error: err,
	}
}

func (err MarshableError) MarshalJSON() ([]byte, error) {
	if err.error == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(err.Error())
}

type WebhookMessage struct {
	Pipeline        string         `json:"pipeline"`
	PipelineVersion string         `json:"pipeline_version"`
	Workdir         string         `json:"workdir"`
	Message         string         `json:"message"`
	MessageType     MessageType    `json:"message_type"`
	Success         bool           `json:"success"`
	Error           MarshableError `json:"error"`
	Time            time.Time      `json:"time"`
}

func NewSt2Webhook(url string, apiKey string) *Webhook {
	return NewWebhook(url, apiKey, "St2-Api-Key")
}

func NewWebhook(url string, apiKey string, headerKey string) *Webhook {
	return &Webhook{
		Client:    http.Client{},
		URL:       url,
		APIKey:    apiKey,
		HeaderKey: headerKey,
		Method:    "POST",
	}
}

func (h *Webhook) DisableTLSVerification() {
	h.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

func (h *Webhook) SetCertificates(certs string) error {
	certFile, err := os.Open(certs)
	if err != nil {
		return fmt.Errorf("failed to open certificate file: %w", err)
	}
	caCert, err := io.ReadAll(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificates: %w", err)
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("failed to parse certificates: %w", err)
	}
	h.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: caCertPool,
		},
	}
	return nil
}

func (h *Webhook) webhookRequest(payload any) (*http.Request, error) {
	switch pt := payload.(type) {
	case WebhookMessage:
		pt.Time = time.Now()
		payload = pt
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	slog.Debug("webhook request", "url", h.URL, "payload", jsonPayload)
	bodyReader := bytes.NewReader(jsonPayload)
	r, err := http.NewRequest(h.Method, h.URL, bodyReader)
	if err != nil {
		return r, err
	}
	r.Header.Add(h.HeaderKey, h.APIKey)
	r.Header.Add("X-Plumber-Version", h.PlumberVersion)
	r.Header.Add("Content-Type", "application/json")
	return r, nil
}

func (h *Webhook) Send(payload any) error {
	r, err := h.webhookRequest(payload)
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	res, err := h.Do(r)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	slog.Debug("webhook response", "url", h.URL, "status", res.Status)
	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read webhook response body: %w", err)
		}
		return fmt.Errorf("webhook denied: status=%s, body=%s", res.Status, body)
	}
	return nil
}
