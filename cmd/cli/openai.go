package cli

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
)

type functionCallType string

const (
	fnCallAuto functionCallType = "auto"
	fnCallNone functionCallType = "none"
)

// openaiGptCompletion is a function that sends a completion request to the OpenAI GPT-3 API
// and returns the generated text based on the provided prompt.
func (c *oaiClients) openaiGptCompletion(ctx context.Context, prompt *strings.Builder, temp float32) (string, error) {
	// Create a completion request with the provided prompt and temperature
	req := openai.CompletionRequest{
		Prompt:      []string{prompt.String()},
		Echo:        false,
		N:           1,
		Temperature: temp,
	}

	// Send the completion request to the OpenAI GPT API
	resp, err := c.openAIClient.CreateCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	// Check if the response contains exactly one choice
	if len(resp.Choices) != 1 {
		return "", fmt.Errorf("expected choices to be 1 but received: %d", len(resp.Choices))
	}

	// Return the generated text from the response
	return resp.Choices[0].Text, nil
}

// openaiGptChatCompletion is a function that performs chat completion using OpenAI GPT model.
// It takes a context, a prompt, and a temperature as input and returns the completed chat response or an error.
func (c *oaiClients) openaiGptChatCompletion(ctx context.Context, prompt *strings.Builder, temp float32) (string, error) {
	var (
		resp     openai.ChatCompletionResponse
		req      openai.ChatCompletionRequest
		funcName *openai.FunctionCall
		content  string
		err      error
	)

	// Determine the type of function call based on whether the k8s API is being used or not.
	fnCallType := fnCallAuto
	if !*usek8sAPI {
		fnCallType = fnCallNone
	}

	for {
		// Append the content to the prompt.
		prompt.WriteString(content)
		log.Debugf("prompt: %s", prompt.String())

		// Create the chat completion request.
		req = openai.ChatCompletionRequest{
			Model: *openAIDeploymentName,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt.String(),
				},
			},
			N:           1,
			Temperature: temp,
			Functions: []openai.FunctionDefinition{
				findSchemaNames,
				getSchema,
			},
			FunctionCall: fnCallType,
		}

		// Call the OpenAI API to get the chat completion response.
		resp, err = c.openAIClient.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", err
		}

		funcName = resp.Choices[0].Message.FunctionCall
		// If there is no function call, we are done.
		if funcName == nil {
			break
		}
		log.Debugf("calling function: %s", funcName.Name)

		// If there is a function call, we need to call it and get the result.
		content, err = funcCall(funcName)
		if err != nil {
			return "", err
		}
	}

	if len(resp.Choices) != 1 {
		return "", fmt.Errorf("expected choices to be 1 but received: %d", len(resp.Choices))
	}

	result := resp.Choices[0].Message.Content
	log.Debugf("result: %s", result)

	// Remove unnecessary backticks if they are in the output.
	result = trimTicks(result)

	return result, nil
}

// trimTicks removes the tick marks from a given string.
// It replaces all occurrences of "```yaml" and "```" with an empty string.
// The modified string is then returned.
func trimTicks(str string) string {
	trimStr := []string{"```yaml", "```"}
	for _, t := range trimStr {
		str = strings.ReplaceAll(str, t, "")
	}
	return str
}
