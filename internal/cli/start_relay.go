package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hardhacker/hylo/internal/relay"
	"github.com/spf13/cobra"
)

func newStartRelayCmd() *cobra.Command {
	var (
		listen  string
		state   string
		joinKey string
	)
	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Start the public Relay reverse-tunnel server",
		Long: `Start the public Relay server for mobile remote access.

Put a TLS reverse proxy (such as Caddy or Nginx) in front of --listen,
allowing WebSocket upgrades and unbuffered SSE streaming.

--join-key (or HYLO_RELAY_JOIN_KEY) is required. Home Hylo servers must
present that shared key to register and connect.`,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runRelay(listen, state, joinKey)
		},
	}
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:8080", "listen address (put TLS termination in front)")
	cmd.Flags().StringVar(&state, "state", "relay-state.json", "credential state file")
	cmd.Flags().StringVar(&joinKey, "join-key", os.Getenv("HYLO_RELAY_JOIN_KEY"), "shared key required for connectors to register and connect (env HYLO_RELAY_JOIN_KEY)")
	return cmd
}

func runRelay(listen, state, joinKey string) error {
	joinKey = strings.TrimSpace(joinKey)
	if joinKey == "" {
		return errors.New("relay: --join-key or HYLO_RELAY_JOIN_KEY is required")
	}
	r, err := relay.New(relay.Config{
		StateFile:    state,
		AutoRegister: true,
		JoinKey:      joinKey,
	})
	if err != nil {
		return fmt.Errorf("init relay: %w", err)
	}

	s := &http.Server{
		Addr:              listen,
		Handler:           r.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		slog.Info("relay shutting down")
		r.Close()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(shutCtx)
	}()

	slog.Info("relay listening", "addr", listen, "state", state)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("relay server: %w", err)
	}
	return nil
}
