package whisper

import (
	"context"
	_ "embed"
	"fmt"
	"nicemaxxingbot/app/config"
	"os"
	"strings"

	"github.com/samber/do"
	"github.com/sashabaranov/go-openai"
)

//go:embed SYSTEM_PROMPT.txt
var systemPrompt string

type Client struct {
	cfg    *config.Config
	client *openai.Client
}

func NewClient(di *do.Injector) (*Client, error) {
	cfg := do.MustInvoke[*config.Config](di)
	clientConfig := openai.DefaultConfig("<n/a>")
	clientConfig.BaseURL = cfg.Whisper.BaseURL
	client := openai.NewClientWithConfig(clientConfig)

	return &Client{
		cfg:    cfg,
		client: client,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.GetModel(ctx, c.cfg.Whisper.Model)
	if err != nil {
		return fmt.Errorf("failed to ping client: %w", err)
	}

	return nil
}

func (c *Client) TranscribeFile(ctx context.Context, filePath string) (string, error) {
	audioFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()

	resp, err := c.client.CreateTranscription(ctx, openai.AudioRequest{
		Model: c.cfg.Whisper.Model,
		// TODO: FIX: system prompt corrupts the results
		//Prompt:   systemPrompt,
		FilePath: filePath,
		Format:   openai.AudioResponseFormatJSON,
	})
	if err != nil {
		return "", fmt.Errorf("CreateTranscription: %w", err)
	}

	result := strings.TrimSpace(resp.Text)

	if result == "" {
		return "", fmt.Errorf("empty transcription result")
	}

	return result, nil
}
