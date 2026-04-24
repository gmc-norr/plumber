package plumber

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testCase struct {
	message   any
	version   string
	apiKey    string
	shouldErr bool
}

func testServer(apiKey string, testcase *testCase, t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("St2-Api-Key"); key == "" {
			t.Log("missing api key")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("missing api key"))
			return
		} else if key != apiKey {
			t.Log("invalid api key")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("invalid api key"))
			return
		}
		if r.Header.Get("X-Plumber-Version") != testcase.version {
			t.Logf("unexpected version %s", r.Header.Get("X-Plumber-Version"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message": "webhook received"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestSt2Webhook(t *testing.T) {
	testcases := map[string]testCase{
		"text message": {
			message: "hello",
			version: "0.1.0",
			apiKey:  "supersecret",
		},
		"webhook message": {
			message: WebhookMessage{
				Pipeline:        "test",
				PipelineVersion: "1.0.0",
				Message:         "this is a test",
				MessageType:     MessageInit,
				Success:         true,
				Error:           NewMarshableError(nil),
			},
			version: "2.1.0",
			apiKey:  "supersecret",
		},
		"text message with incorrect key": {
			message:   "hello",
			version:   "0.1.0",
			apiKey:    "Supersecret",
			shouldErr: true,
		},
		"webhook message with missing key": {
			message: WebhookMessage{
				Pipeline:        "test",
				PipelineVersion: "1.0.0",
				Message:         "this is a test",
				MessageType:     MessageInit,
				Success:         true,
				Error:           NewMarshableError(nil),
			},
			version:   "2.1.0",
			shouldErr: true,
		},
	}

	serverkey := "supersecret"

	for name, c := range testcases {
		t.Run(name, func(t *testing.T) {
			server := testServer(serverkey, &c, t)
			defer server.Close()
			t.Logf("sending webook request to test server at %s", server.URL)
			h := NewSt2Webhook(server.URL, c.apiKey)
			h.PlumberVersion = c.version
			err := h.Send(context.Background(), c.message)
			t.Logf("send error: %v", err)
			if (err != nil) != c.shouldErr {
				if c.shouldErr {
					t.Errorf("expected an error, got %q", err)
				} else {
					t.Errorf("expected no error, got %q", err)
				}
			}
		})
	}
}

func TestFailedConnection(t *testing.T) {
	testcases := map[string]testCase{
		"wrong server": {
			message: "hello",
			version: "0.1.0",
			apiKey:  "supersecret",
		},
	}

	for name, c := range testcases {
		t.Run(name, func(t *testing.T) {
			server := testServer("supersecret", &c, t)
			defer server.Close()
			h := NewSt2Webhook("http://thisisnotright.localhost", c.apiKey)
			err := h.Send(context.Background(), c.message)
			t.Logf("send error: %v", err)
			if err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}
