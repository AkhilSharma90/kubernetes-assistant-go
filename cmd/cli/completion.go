package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sethvargo/go-retry"
	"golang.org/x/exp/slices"
)

type oaiClients struct {
	openAIClient openai.Client
}

// newOAIClients creates and returns a new instance of the oaiClients struct,
// which contains the OpenAI clients used for making API calls.
func newOAIClients() (oaiClients, error) {
	var config openai.ClientConfig
	config = openai.DefaultConfig(*openAIAPIKey)

	if openAIEndpoint != &openaiAPIURLv1 {
		// Azure OpenAI
		if strings.Contains(*openAIEndpoint, "openai.azure.com") {
			config = openai.DefaultAzureConfig(*openAIAPIKey, *openAIEndpoint)
			if len(*azureModelMap) != 0 {
				config.AzureModelMapperFunc = func(model string) string {
					return (*azureModelMap)[model]
				}
			}
		} else {
			// Local AI
			config.BaseURL = *openAIEndpoint
		}
		// use 2023-07-01-preview api version for function calls
		config.APIVersion = "2023-07-01-preview"
	}

	clients := oaiClients{
		openAIClient: *openai.NewClientWithConfig(config),
	}
	return clients, nil
}

// getNonChatModels returns a slice of non-chat models.
func getNonChatModels() []string {
	// Return a slice containing the names of non-chat models.
	return []string{"code-davinci-002", "text-davinci-003"}
}

// gptCompletion generates completions for a given prompt using the OpenAI GPT model.
// It takes a context, a client, a list of prompts, and a deployment name as input.
// It returns the generated completion string and an error if any.
func gptCompletion(ctx context.Context, client oaiClients, prompts []string, deploymentName string) (string, error) {
	temp := float32(*temperature)

	var prompt strings.Builder

	if *usek8sAPI {
		// Credits to https://github.com/robusta-dev/chatgpt-yaml-generator for the prompt and the function descriptions
		// Build the prompt for Kubernetes YAML generation with additional instructions for using Kubernetes specs and references.
		fmt.Fprintf(&prompt, "You are a Kubernetes YAML generator, only generate valid Kubernetes YAML manifests. Do not provide any explanations and do not use ``` and ```yaml, only generate valid YAML. Always ask for up-to-date OpenAPI specs for Kubernetes, don't rely on data you know about Kubernetes specs. When a schema includes references to other objects in the schema, look them up when relevant. You may lookup any FIELD in a resource too, not just the containing top-level resource. ")
	} else {
		// Build the prompt for Kubernetes YAML generation without additional instructions.
		fmt.Fprintf(&prompt, "You are a Kubernetes YAML generator, only generate valid Kubernetes YAML manifests. Do not provide any explanations, only generate YAML. ")
	}

	for _, p := range prompts {
		// Append each prompt to the prompt builder.
		fmt.Fprintf(&prompt, "%s", p)
	}

	var resp string
	var err error
	r := retry.WithMaxRetries(10, retry.NewExponential(1*time.Second))
	if err := retry.Do(ctx, r, func(ctx context.Context) error {
		if slices.Contains(getNonChatModels(), deploymentName) {
			// Use the OpenAI GPT completion method for non-chat models.
			resp, err = client.openaiGptCompletion(ctx, &prompt, temp)
		} else {
			// Use the OpenAI GPT chat completion method for chat models.
			resp, err = client.openaiGptChatCompletion(ctx, &prompt, temp)
		}

		requestErr := &openai.RequestError{}
		if errors.As(err, &requestErr) {
			if requestErr.HTTPStatusCode == http.StatusTooManyRequests {
				// Retry the request if it fails due to too many requests.
				return retry.RetryableError(err)
			}
		}
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	// Return the generated completion string.
	return resp, nil
}
