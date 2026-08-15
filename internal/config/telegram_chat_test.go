package config

import (
	"strings"
	"testing"
)

func TestTelegramChatIDPrecedence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		cfg          TelegramAlertsConfig
		wantApp      string
		wantServer   string
		appLegacy    bool
		serverLegacy bool
	}{
		{
			name: "A_app_specific_wins_over_legacy",
			cfg: TelegramAlertsConfig{
				AppAlertChatID: "app-specific",
				AlertChatID:    "legacy-shared",
			},
			wantApp:      "app-specific",
			wantServer:   "legacy-shared",
			appLegacy:    false,
			serverLegacy: true,
		},
		{
			name: "B_server_specific_wins_over_legacy",
			cfg: TelegramAlertsConfig{
				ServerAlertChatID: "server-specific",
				AlertChatID:       "legacy-shared",
			},
			wantApp:      "legacy-shared",
			wantServer:   "server-specific",
			appLegacy:    true,
			serverLegacy: false,
		},
		{
			name: "C_app_falls_back_to_legacy",
			cfg: TelegramAlertsConfig{
				AlertChatID: "legacy-shared",
			},
			wantApp:      "legacy-shared",
			wantServer:   "legacy-shared",
			appLegacy:    true,
			serverLegacy: true,
		},
		{
			name: "D_server_falls_back_to_legacy",
			cfg: TelegramAlertsConfig{
				AlertChatID: "legacy-shared",
			},
			wantApp:      "legacy-shared",
			wantServer:   "legacy-shared",
			appLegacy:    true,
			serverLegacy: true,
		},
		{
			name: "G_app_and_server_keep_distinct_destinations",
			cfg: TelegramAlertsConfig{
				AppAlertChatID:    "app-chat",
				ServerAlertChatID: "server-chat",
				AlertChatID:       "legacy-ignored",
			},
			wantApp:      "app-chat",
			wantServer:   "server-chat",
			appLegacy:    false,
			serverLegacy: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.cfg.AppChatID() != tc.wantApp {
				t.Fatalf("AppChatID=%q want %q", tc.cfg.AppChatID(), tc.wantApp)
			}
			if tc.cfg.ServerChatID() != tc.wantServer {
				t.Fatalf("ServerChatID=%q want %q", tc.cfg.ServerChatID(), tc.wantServer)
			}
			if tc.cfg.AppUsesLegacyChatFallback() != tc.appLegacy {
				t.Fatalf("AppUsesLegacyChatFallback=%v want %v", tc.cfg.AppUsesLegacyChatFallback(), tc.appLegacy)
			}
			if tc.cfg.ServerUsesLegacyChatFallback() != tc.serverLegacy {
				t.Fatalf("ServerUsesLegacyChatFallback=%v want %v", tc.cfg.ServerUsesLegacyChatFallback(), tc.serverLegacy)
			}
		})
	}
}

func TestTelegramRequiredModeRejectsMissingChat(t *testing.T) {
	t.Parallel()
	t.Run("E_app_chat_missing", func(t *testing.T) {
		t.Parallel()
		cfg := TelegramAlertsConfig{
			Enabled:           true,
			Required:          true,
			AppBotToken:       "app-token",
			ServerBotToken:    "server-token",
			ServerAlertChatID: "server-chat",
		}
		err := cfg.validate()
		if err == nil {
			t.Fatal("expected APP chat validation error")
		}
		if !strings.Contains(err.Error(), "APP") {
			t.Fatalf("error should mention APP: %v", err)
		}
	})
	t.Run("F_server_chat_missing", func(t *testing.T) {
		t.Parallel()
		cfg := TelegramAlertsConfig{
			Enabled:        true,
			Required:       true,
			AppBotToken:    "app-token",
			AppAlertChatID: "app-chat",
			ServerBotToken: "server-token",
		}
		err := cfg.validate()
		if err == nil {
			t.Fatal("expected SERVER chat validation error")
		}
		if !strings.Contains(err.Error(), "SERVER") {
			t.Fatalf("error should mention SERVER: %v", err)
		}
	})
	t.Run("both_configured_ok", func(t *testing.T) {
		t.Parallel()
		cfg := TelegramAlertsConfig{
			Enabled:           true,
			Required:          true,
			AppBotToken:       "app-token",
			AppAlertChatID:    "app-chat",
			ServerBotToken:    "server-token",
			ServerAlertChatID: "server-chat",
		}
		if err := cfg.validate(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("no_cross_fallback_app_to_server", func(t *testing.T) {
		t.Parallel()
		cfg := TelegramAlertsConfig{
			AppAlertChatID:    "",
			ServerAlertChatID: "server-only",
		}
		if got := cfg.AppChatID(); got != "" {
			t.Fatalf("APP must not fall back to SERVER chat, got %q", got)
		}
		if cfg.ServerChatID() != "server-only" {
			t.Fatalf("ServerChatID=%q", cfg.ServerChatID())
		}
	})
}
