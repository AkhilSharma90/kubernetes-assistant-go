package cli
//COMPLETE
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

//define a struct having a field of type openai.Client
type oaiClients struct {
	openAIClient openai.Client
}

// newOAIClients creates and returns a new instance of the oaiClients struct,
// which contains the OpenAI clients used for making API calls.
//you can get the open ai client directly or open ai via azure
func newOAIClients() (oaiClients, error) {
	//create a variable config of type openai.ClientConfig
	var config openai.ClientConfig
	//set config equal to openAIAPIKey which will be set in the environment variables
	//we have to export openAIAPIKey in our terminals
	//this variable (openAIAPIKey) and all others are defined in the root.go file
	config = openai.DefaultConfig(*openAIAPIKey)
//openAIEndpoint is a variable defined in the root.go file, we're checking here
//if and another variable defined (openaiAPIURLv1) in root.go are same or not
	if openAIEndpoint != &openaiAPIURLv1 {
		//we enter this loop if both the links are not equal, in many cases you might
		//not even specify the endpoint and it'll go with APIURLv1 defined by default
		// so if they're not equal, we're checking if it has azure open ai URL
		if strings.Contains(*openAIEndpoint, "openai.azure.com") {
			//if it is the open ai API via azure, we set it using DefaultAzureConfig function
			//present in the open ai package
			config = openai.DefaultAzureConfig(*openAIAPIKey, *openAIEndpoint)
//if we have set the azureModelMap (in root.go file) and length is not zero
			if len(*azureModelMap) != 0 {
//then we assign that value to open ai config that needs to work with it
//this is basically mapping for open ai to azure
				config.AzureModelMapperFunc = func(model string) string {
					return (*azureModelMap)[model]
				}
			}
		} else {
// if we're not using open ai via azure, we will assign the AIEndpoint to BaseURL 
			config.BaseURL = *openAIEndpoint
		}
		//still crafting the config object, by specifying an API version
		// use 2023-07-01-preview api version for function calls
		config.APIVersion = "2023-07-01-preview"
	}
//passing the crafted config object to the NewClientWithConfig func. from open ai
//and assigning it to the openAIClient field in oaiClients - a struct defined at the top of this file
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
//we are going to create a prompt and going to append things to it and this is why
//we set it to be strings.Builder instead of just strings
	var prompt strings.Builder

	if *usek8sAPI {
		// Credits to https://github.com/robusta-dev/chatgpt-yaml-generator for the prompt and the function descriptions
		// Build the prompt for Kubernetes YAML generation with additional instructions for using Kubernetes specs and references.
		//if using the k8sAPI, we want it to not rely on it's existing knowledge and get the latest info
		fmt.Fprintf(&prompt, "You are a Kubernetes YAML generator, only generate valid Kubernetes YAML manifests. Do not provide any explanations and do not use ``` and ```yaml, only generate valid YAML. Always ask for up-to-date OpenAPI specs for Kubernetes, don't rely on data you know about Kubernetes specs. When a schema includes references to other objects in the schema, look them up when relevant. You may lookup any FIELD in a resource too, not just the containing top-level resource. ")
	} else {
		// Build the prompt for Kubernetes YAML generation without additional instructions.
		fmt.Fprintf(&prompt, "You are a Kubernetes YAML generator, only generate valid Kubernetes YAML manifests. Do not provide any explanations, only generate YAML. ")
	}

	//range over the prompts slice received in the function, access each prompt
	//using the 'p' variable and append it to the prompt defined above which is a strings.Builder
	//and has either of the values defined above
	for _, p := range prompts {
		// Append each prompt to the prompt builder.
		fmt.Fprintf(&prompt, "%s", p)
	}
//define a variable resp for working with response object
	var resp string
	var err error
	//setting the max retires at 10 and then later also handling too many retries condition
	r := retry.WithMaxRetries(10, retry.NewExponential(1*time.Second))
	if err := retry.Do(ctx, r, func(ctx context.Context) error {
		if slices.Contains(getNonChatModels(), deploymentName) {
			// Use the OpenAI GPT completion method for non-chat models.
			//open ai GPT completion function is used, notice the missing 'chat'
			resp, err = client.openaiGptCompletion(ctx, &prompt, temp)
		} else {
			// Use the OpenAI GPT chat completion method for chat models.
			//if the slice doesn't contain non chat models, then we call this
			resp, err = client.openaiGptChatCompletion(ctx, &prompt, temp)
		}
//if there are any errors when making a request to the open ai API, they're accessible to us
//through openai.RequestError and we assign it to the requestErr variable
		requestErr := &openai.RequestError{}
		//err is the error from calling open ai, from the resp lines above
		//errors.As helps us compare the err and requestErr, whether they're the same
		if errors.As(err, &requestErr) {
		//so if we have an error which is of type openai Request error, means we have some 
		//issue while making a request, now we want to zero down on the issue, so we check if it
		//has the status code of too many requests,which is 429 code
			if requestErr.HTTPStatusCode == http.StatusTooManyRequests {
		//if this is the case, it means the issue is retryable and we can rety
		//the request after a certain delay
				return retry.RetryableError(err)
			}
		}
		//if the error hasn't matched the condition above of being a request retry error
		//and it still exists, means it's something else and is not retryable, so we will simply
		//return the error as is
		if err != nil {
			return err
		}
		//we are still in the retry loop, not going to return any value now
		return nil
	}); err != nil {
		//handling the error from the retry code block
		return "", err
	}

	// Return the generated completion string.
	return resp, nil
}
