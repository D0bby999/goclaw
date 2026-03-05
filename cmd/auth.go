package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nextlevelbuilder/goclaw/internal/oauth"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with LLM providers via OAuth",
	}
	cmd.AddCommand(authOpenAICmd())
	cmd.AddCommand(authStatusCmd())
	cmd.AddCommand(authLogoutCmd())
	return cmd
}

func authOpenAICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "openai",
		Short: "Sign in with your ChatGPT subscription (OAuth)",
		Long:  "Authenticate with OpenAI using your ChatGPT Plus/Pro subscription via OAuth PKCE flow. This allows using your subscription's models without a separate API key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()

			fmt.Println("Starting OpenAI OAuth authentication...")
			fmt.Println("This will open your browser to sign in with your ChatGPT account.")
			fmt.Println()

			tokenResp, err := oauth.LoginOpenAI(ctx)
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			// Save token
			tokenPath := oauth.DefaultTokenPath()
			encKey := os.Getenv("GOCLAW_ENCRYPTION_KEY")
			ts := oauth.NewTokenSource(tokenPath, encKey)
			if err := ts.Save(tokenResp); err != nil {
				return fmt.Errorf("save token: %w", err)
			}

			fmt.Println()
			fmt.Println("Authentication successful!")
			fmt.Printf("Token saved to: %s\n", tokenPath)
			fmt.Printf("Token expires in: %s\n", time.Duration(tokenResp.ExpiresIn)*time.Second)
			fmt.Println()
			fmt.Println("GoClaw will register the 'openai-codex' provider using your ChatGPT subscription.")
			fmt.Println("Use model prefix 'openai-codex/' in agent config (e.g. openai-codex/gpt-4o).")
			fmt.Println("Token will be refreshed automatically before expiry.")
			return nil
		},
	}
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show OAuth authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenPath := oauth.DefaultTokenPath()
			if !oauth.TokenFileExists(tokenPath) {
				fmt.Println("No OAuth tokens found.")
				fmt.Println("Run 'goclaw auth openai' to authenticate.")
				return nil
			}

			encKey := os.Getenv("GOCLAW_ENCRYPTION_KEY")
			ts := oauth.NewTokenSource(tokenPath, encKey)
			token, err := ts.Token()
			if err != nil {
				fmt.Printf("Token file exists but is invalid: %v\n", err)
				fmt.Println("Run 'goclaw auth openai' to re-authenticate.")
				return nil
			}

			// Mask the token for display
			masked := token
			if len(token) > 12 {
				masked = token[:8] + "..." + token[len(token)-4:]
			}
			fmt.Printf("OpenAI OAuth: active\n")
			fmt.Printf("Token: %s\n", masked)
			fmt.Printf("Token file: %s\n", tokenPath)
			return nil
		},
	}
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout [provider]",
		Short: "Remove stored OAuth tokens",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := "openai"
			if len(args) > 0 {
				provider = args[0]
			}

			if provider != "openai" {
				return fmt.Errorf("unknown provider: %s (supported: openai)", provider)
			}

			tokenPath := oauth.DefaultTokenPath()
			if err := os.Remove(tokenPath); err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No OAuth token found for OpenAI.")
					return nil
				}
				return err
			}
			fmt.Println("OpenAI OAuth token removed.")
			return nil
		},
	}
}
