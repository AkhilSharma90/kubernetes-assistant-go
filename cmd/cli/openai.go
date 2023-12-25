package cli
//COMPLETE
import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
)

//defining our datatype functionCallType of type string
type functionCallType string
//defining 2 variables of the type functionCallType
const (
	fnCallAuto functionCallType = "auto"
	fnCallNone functionCallType = "none"
)

//if you want to use open AI chat models, you have to use chat completion function
//it takes multpiple messages (or a complete dialogue) and not just a prompt

// openaiGptCompletion is a function that sends a completion request to the OpenAI GPT-3 API
// and returns the generated text based on the provided prompt.
func (c *oaiClients) openaiGptCompletion(ctx context.Context, prompt *strings.Builder, temp float32) (string, error) {
	// Create a completion request with the provided prompt and temperature
	req := openai.CompletionRequest{
		Prompt:      []string{prompt.String()},
		Echo:        false,
		//n basically controls how many chat completion options you want open ai to
		//generate for you, keep it 1 if you want a low bill. if you're building something more
		//advanced, keep it more than 2 so that you can pick from different options
		N:           1,
		//sampling temperature, between 0 and 2. if it's high like 0.8, output will be a bit
		//more random, but output will be controlled if it's closer to 0, will be more deterministic
		Temperature: temp,
	}

	// Send the completion request to the OpenAI GPT API
	//passing the req object crafted above to a func. available in openAI library
	//c being the oaiclients being used to access this particular method
	resp, err := c.openAIClient.CreateCompletion(ctx, req)
	//handling the error from the chatgpt request
	if err != nil {
		return "", err
	}

	// Check if the response contains exactly one choice
	//if you select n more than 1, you will get more choices
	if len(resp.Choices) != 1 {
		return "", fmt.Errorf("expected choices to be 1 but received: %d", len(resp.Choices))
	}

	// Return the generated text from the response
	//the first choice from the response is what we want to return from here
	return resp.Choices[0].Text, nil
}

// openaiGptChatCompletion is a function that performs chat completion using OpenAI GPT model.
// It takes a context, a prompt, and a temperature as input and returns the completed chat response or an error.
func (c *oaiClients) openaiGptChatCompletion(ctx context.Context, prompt *strings.Builder, temp float32) (string, error) {
	//defining some variables to work with request, response etc.
	var (
		resp     openai.ChatCompletionResponse
		req      openai.ChatCompletionRequest
		funcName *openai.FunctionCall
		content  string
		err      error
	)

	// Determine the type of function call based on whether the k8s API is being used or not.
	fnCallType := fnCallAuto
	//if K8sAPI is not being used (i.e the flag is false)
	if !*usek8sAPI {
		//then function call will be of type None
		fnCallType = fnCallNone
	}

	for {
		// Append the content to the prompt.
		prompt.WriteString(content)
		log.Debugf("prompt: %s", prompt.String())

		// Create the chat completion request.
//if you notice, a different function is called here "chatCompletionRequest"
//from open ai, while in the function above, we call CompletionRequest function
//this one takes the model name, slice of messages that contains the messages 
//in the chat so far, N, temp, functions to be called
//the functions are kubernetes related functions defined in the functions.go file
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
			//sending the variables defined as FunctionDefition in functions.go file
				findSchemaNames,
				getSchema,
			},
			FunctionCall: fnCallType,
		}
//calling the API's function CreateChatCompltion by passing the request object
		// Call the OpenAI API to get the chat completion response.
		resp, err = c.openAIClient.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", err
		}
//the response has FunctionCall data and we'll extract that in funcName variable
//defined with the variables earlier in this function
		funcName = resp.Choices[0].Message.FunctionCall
		// If there is no function call, we are done.
		if funcName == nil {
			break
		}
		//if there is a function to be called, we will print that we're calling that function
		//and will print it's name
		log.Debugf("calling function: %s", funcName.Name)

		// If there is a function call, we need to call it and get the result.
		//calling the function here and the result that comes back will be captured in content
		//content is a variable we have defined earlier which is of type string
		//funcCall function is also defined in functions.go
		content, err = funcCall(funcName)
		if err != nil {
			return "", err
		}
	}
//if length is more than 1, we will send an error just like the previous function
//this usually happens is n is set to be more than 1, open ai returns more options
	if len(resp.Choices) != 1 {
		return "", fmt.Errorf("expected choices to be 1 but received: %d", len(resp.Choices))
	}
//select the content of the first choice in the response and capture that in result
	result := resp.Choices[0].Message.Content
	//print the result, we will be returning it from this function
	log.Debugf("result: %s", result)

	// Remove unnecessary backticks if they are in the output.
	//the trim ticks function is mentioned below, for working with yaml files
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
