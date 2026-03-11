package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"goated/internal/app"
)

func credsDir() string {
	cfg := app.LoadConfig()
	return filepath.Join(cfg.WorkspaceDir, "creds")
}

var credsCmd = &cobra.Command{
	Use:   "creds",
	Short: "Manage file-backed credentials (creds/*.txt)",
}

var credsSetCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Store a credential",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		dir := credsDir()
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("mkdir creds: %w", err)
		}
		path := filepath.Join(dir, key+".txt")
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("Set %s\n", key)
		return nil
	},
}

var credsGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Retrieve a credential value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		path := filepath.Join(credsDir(), key+".txt")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("credential %q not found", key)
			}
			return err
		}
		fmt.Print(strings.TrimRight(string(data), "\n"))
		return nil
	},
}

var credsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stored credential keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := credsDir()
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("(no credentials stored)")
				return nil
			}
			return err
		}
		var keys []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".txt") {
				keys = append(keys, strings.TrimSuffix(name, ".txt"))
			}
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			fmt.Println("(no credentials stored)")
			return nil
		}
		for _, k := range keys {
			fmt.Println(k)
		}
		return nil
	},
}

func init() {
	credsCmd.AddCommand(credsSetCmd, credsGetCmd, credsListCmd)
	rootCmd.AddCommand(credsCmd)
}
