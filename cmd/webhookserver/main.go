package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	var apiKey, apiKeyHeader string
	cmd := &cobra.Command{
		Use:   "[flags]",
		Short: "Test server for listening to webhooks",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			apiKey, _ = cmd.Flags().GetString("api-key")
			apiKeyHeader, _ = cmd.Flags().GetString("api-key-header")
			if (apiKey == "") != (apiKeyHeader == "") {
				return errors.New("both or neither of api-key and api-key-header needs to be defined")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			return Serve(port, apiKey, apiKeyHeader)
		},
	}

	cmd.Flags().IntP("port", "p", 3000, "Port to listen on")
	cmd.Flags().String("api-key-header", "", "API key HTTP header name")
	cmd.Flags().String("api-key", "", "API key for authentication")

	return cmd
}

func Serve(port int, key string, keyName string) error {
	mux := http.NewServeMux()
	mux.Handle("POST /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("\nnew webhook message")
		if key != "" && keyName != "" {
			if key != r.Header.Get(keyName) {
				slog.Error("unauthorized; mismatching api key")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Println("authorization ok")
		}
		var payload map[string]any
		b, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read request body", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(b, &payload); err != nil {
			slog.Error("failed unmarshal payload", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for k, v := range payload {
			fmt.Printf("\t%s: %s\n", k, v)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("listening for webhook messages on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	cmd := NewRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
