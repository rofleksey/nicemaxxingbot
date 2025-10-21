package openai

import (
	"context"
	_ "embed"
	"fmt"
	"nicemaxxingbot/app/config"
	"strings"

	"github.com/rofleksey/meg"
	"github.com/samber/do"
	"github.com/sashabaranov/go-openai"
)

//go:embed SYSTEM_PROMPT.txt
var systemPrompt string

type Client struct {
	cfg        *config.Config
	freeClient *openai.Client
	client     *openai.Client
}

func newOpenaiClient(cfg *config.Config, free bool) *openai.Client {
	var openaiCfg config.OpenAI

	if free {
		openaiCfg = cfg.FreeOpenAI
	} else {
		openaiCfg = cfg.OpenAI
	}

	clientConfig := openai.DefaultConfig(openaiCfg.Token)
	clientConfig.BaseURL = openaiCfg.BaseURL
	return openai.NewClientWithConfig(clientConfig)
}

func NewClient(di *do.Injector) (*Client, error) {
	cfg := do.MustInvoke[*config.Config](di)

	return &Client{
		cfg:        cfg,
		freeClient: newOpenaiClient(cfg, true),
		client:     newOpenaiClient(cfg, false),
	}, nil
}

func (c *Client) doCompletionRequest(ctx context.Context, text string, useFreeClient bool) (*openai.ChatCompletionResponse, error) {
	var client *openai.Client
	var model string

	if useFreeClient {
		client = c.freeClient
		model = c.cfg.FreeOpenAI.Model
	} else {
		client = c.client
		model = c.cfg.OpenAI.Model
	}

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
			MaxCompletionTokens: 10000,
			Seed:                meg.ToPtr(2025),
		},
	)

	return &resp, err
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.freeClient.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping free client: %w", err)
	}

	_, err = c.client.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping paid client: %w", err)
	}

	return nil
}

func (c *Client) CheckToxicity(ctx context.Context, text string, useFreeClient bool) (string, bool, error) {
	resp, err := c.doCompletionRequest(ctx, text, useFreeClient)
	if err != nil {
		return "", false, fmt.Errorf("CreateChatCompletion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", false, fmt.Errorf("empty openai response")
	}

	rawResult := strings.TrimSpace(resp.Choices[0].Message.Content)

	if rawResult == "OK" {
		return "", false, nil
	}

	if strings.HasPrefix(rawResult, "TOXIC:") {
		rawResult = strings.TrimPrefix(rawResult, "TOXIC:")
		rawResult = strings.TrimSpace(rawResult)
		return rawResult, true, nil
	}

	return "", false, fmt.Errorf("invalid openai response: %s", rawResult)
}
