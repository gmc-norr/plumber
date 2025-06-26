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
)

type Webhook struct {
	http.Client
	URL            string
	APIKey         string
	HeaderKey      string
	Method         string
	PlumberVersion string
}

type MessageType int

const (
	MessageStart MessageType = iota
	MessageProgress
	MessageEnd
)

func (t MessageType) String() string {
	switch t {
	case MessageStart:
		return "start"
	case MessageProgress:
		return "progress"
	case MessageEnd:
		return "end"
	}
	return "undefined"
}

type WebhookMessage struct {
	Pipeline        string
	PipelineVersion string
	Workdir         string
	Message         string
	MessageType     MessageType
	Success         bool
	Error           error
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
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	bodyReader := bytes.NewReader(jsonPayload)
	r, err := http.NewRequest(h.Method, h.URL, bodyReader)
	if err != nil {
		return r, err
	}
	r.Header.Add(h.HeaderKey, h.APIKey)
	r.Header.Add("X-Plumber-Version", h.PlumberVersion)
	return r, nil
}

func (h *Webhook) Send(payload any) error {
	r, err := h.webhookRequest(payload)
	slog.Debug("sending message to webhook", "url", h.URL, "payload", fmt.Sprintf("%v", payload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	res, err := h.Do(r)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read webhook response body: %w", err)
		}
		return fmt.Errorf("webhook request failed status=%s, body=%s", res.Status, body)
	}
	return nil
}
