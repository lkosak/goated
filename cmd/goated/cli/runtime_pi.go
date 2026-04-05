package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"goated/internal/app"
)

var runtimePiCmd = &cobra.Command{
	Use:   "pi",
	Short: "Pi runtime management",
}

var runtimePiConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Interactively configure a Pi custom provider (writes ~/.pi/agent/models.json and pins it in goated.json)",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("=== pi runtime configure ===")
		fmt.Println()
		fmt.Println("This wizard adds (or updates) a custom provider in ~/.pi/agent/models.json")
		fmt.Println("and pins it as the active Pi provider/model in goated.json.")
		fmt.Println()
		fmt.Println("You'll need:")
		fmt.Println("  - a provider name (e.g. fireworks, openrouter, together)")
		fmt.Println("  - the provider's OpenAI-compatible base URL")
		fmt.Println("  - an API key for the provider")
		fmt.Println("  - the model ID the provider expects")
		fmt.Println()

		provider := promptRequired(reader, "Provider name", "fireworks")

		baseURL := promptRequired(reader, "Base URL", defaultBaseURLFor(provider))

		apiStyle := promptOneOf(reader, "API style (openai-completions/openai-responses/anthropic)", "openai-completions",
			[]string{"openai-completions", "openai-responses", "anthropic"})

		apiKey := promptSecretRequired(reader, "API key (hidden)")

		modelDefault := "kimi-k2p5"
		modelPrompt := "Model ID"
		if strings.EqualFold(provider, "fireworks") {
			modelPrompt = "Model ID (accounts/fireworks/models/ prefix added automatically)"
		}
		modelID := promptRequired(reader, modelPrompt, modelDefault)
		if strings.EqualFold(provider, "fireworks") && !strings.Contains(modelID, "/") {
			modelID = "accounts/fireworks/models/" + modelID
		}

		displayName := prompt(reader, "Display name", modelDefault)
		contextWindowStr := prompt(reader, "Context window (tokens)", "131072")
		maxTokensStr := prompt(reader, "Max output tokens", contextWindowStr)

		contextWindow := parseIntOrDefault(contextWindowStr, 131072)
		maxTokens := parseIntOrDefault(maxTokensStr, contextWindow)

		// Build the provider entry.
		providerEntry := map[string]any{
			"baseUrl":    baseURL,
			"api":        apiStyle,
			"apiKey":     apiKey,
			"authHeader": true,
			"compat": map[string]any{
				"supportsDeveloperRole":   false,
				"supportsReasoningEffort": false,
			},
			"models": []any{
				map[string]any{
					"id":            modelID,
					"name":          displayName,
					"reasoning":     true,
					"input":         []any{"text", "image"},
					"contextWindow": contextWindow,
					"maxTokens":     maxTokens,
					"cost": map[string]any{
						"input":      0,
						"output":     0,
						"cacheRead":  0,
						"cacheWrite": 0,
					},
				},
			},
		}

		// Merge into existing ~/.pi/agent/models.json if present.
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home dir: %w", err)
		}
		piAgentDir := filepath.Join(home, ".pi", "agent")
		if err := os.MkdirAll(piAgentDir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", piAgentDir, err)
		}
		modelsPath := filepath.Join(piAgentDir, "models.json")

		existing := map[string]any{}
		if data, err := os.ReadFile(modelsPath); err == nil {
			if err := json.Unmarshal(data, &existing); err != nil {
				return fmt.Errorf("parse existing %s: %w", modelsPath, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", modelsPath, err)
		}

		providers, _ := existing["providers"].(map[string]any)
		if providers == nil {
			providers = map[string]any{}
		}
		if _, exists := providers[provider]; exists {
			answer := prompt(reader, fmt.Sprintf("Provider %q already exists — overwrite? (Y/n)", provider), "Y")
			if !strings.EqualFold(strings.TrimSpace(answer), "y") {
				return fmt.Errorf("aborted by user")
			}
		}
		providers[provider] = providerEntry
		existing["providers"] = providers

		content, err := json.MarshalIndent(existing, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal models.json: %w", err)
		}
		content = append(content, '\n')
		if err := os.WriteFile(modelsPath, content, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", modelsPath, err)
		}
		fmt.Println()
		fmt.Printf("Wrote %s (mode 0600)\n", modelsPath)

		// Pin provider + model in goated.json.
		configPath := "goated.json"
		configMap, err := app.ReadConfigJSON(configPath)
		if err != nil {
			return fmt.Errorf("read goated.json: %w", err)
		}
		piSection, _ := configMap["pi"].(map[string]any)
		if piSection == nil {
			piSection = map[string]any{}
		}
		piSection["provider"] = provider
		piSection["model"] = modelID
		configMap["pi"] = piSection
		if err := app.WriteConfigJSON(configPath, configMap); err != nil {
			return fmt.Errorf("write goated.json: %w", err)
		}
		fmt.Printf("Pinned pi.provider=%s pi.model=%s in %s\n", provider, modelID, configPath)

		// Verify by running a real pi probe.
		fmt.Println()
		fmt.Println("Verifying with `pi --list-models`...")
		if err := verifyPiProvider(provider, modelID); err != nil {
			fmt.Printf("  WARNING: %v\n", err)
			fmt.Println("  The config was written, but the probe failed. Fix the underlying issue and re-run.")
			return nil
		}
		fmt.Println("  OK — provider and model are visible to pi.")

		fmt.Println()
		fmt.Println("Next step: restart the daemon to pick up the new config.")
		fmt.Println("  ./goated daemon restart --reason \"pi provider configured\"")
		return nil
	},
}

func defaultBaseURLFor(provider string) string {
	switch strings.ToLower(provider) {
	case "fireworks":
		return "https://api.fireworks.ai/inference/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "together":
		return "https://api.together.xyz/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	default:
		return ""
	}
}

func parseIntOrDefault(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func verifyPiProvider(provider, modelID string) error {
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi binary not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pi", "--list-models").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pi --list-models failed: %s", strings.TrimSpace(string(out)))
	}
	text := strings.ToLower(string(out))
	if !strings.Contains(text, strings.ToLower(provider)) {
		return fmt.Errorf("provider %q not visible in pi --list-models output", provider)
	}
	if !strings.Contains(text, strings.ToLower(modelID)) {
		return fmt.Errorf("model %q not visible in pi --list-models output", modelID)
	}
	return nil
}

func init() {
	runtimePiCmd.AddCommand(runtimePiConfigureCmd)
	runtimeCmd.AddCommand(runtimePiCmd)
}
