package cli
//COMPLETE
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"github.com/janeczku/go-spinner"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/walles/env"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	apply     = "Apply"
	dontApply = "Don't Apply"
	reprompt  = "Reprompt"
)

//these variables help us work with the various environment variables
//we set flags and for each variable, we set the value of the variables from ev. variables
var (
	openaiAPIURLv1        = "https://api.openai.com/v1"             // The URL for the OpenAI API version 1.
	version               = "dev"                                   // The version of the Kubernetes Assistant CLI.
	kubernetesConfigFlags = genericclioptions.NewConfigFlags(false) // Flags for configuring the Kubernetes client.

	openAIDeploymentName = flag.String("openai-deployment-name", env.GetOr("OPENAI_DEPLOYMENT_NAME", env.String, "gpt-3.5-turbo-0301"), "The deployment name used for the model in OpenAI service.")                                                                                               // The name of the deployment used for the OpenAI model.
	openAIAPIKey         = flag.String("openai-api-key", env.GetOr("OPENAI_API_KEY", env.String, ""), "The API key for the OpenAI service. This is required.")                                                                                                                                     // The API key for the OpenAI service.
	openAIEndpoint       = flag.String("openai-endpoint", env.GetOr("OPENAI_ENDPOINT", env.String, openaiAPIURLv1), "The endpoint for OpenAI service. Defaults to"+openaiAPIURLv1+". Set this to your Local AI endpoint or Azure OpenAI Service, if needed.")                                      // The endpoint for the OpenAI service.
	azureModelMap        = flag.StringToString("azure-openai-map", env.GetOr("AZURE_OPENAI_MAP", env.Map(env.String, "=", env.String, ""), map[string]string{}), "The mapping from OpenAI model to Azure OpenAI deployment. Defaults to empty map. Example format: gpt-3.5-turbo=my-deployment.")  // The mapping from OpenAI model to Azure OpenAI deployment.
	requireConfirmation  = flag.Bool("require-confirmation", env.GetOr("REQUIRE_CONFIRMATION", strconv.ParseBool, true), "Whether to require confirmation before executing the command. Defaults to true.")                                                                                        // Whether to require confirmation before executing the command.
	temperature          = flag.Float64("temperature", env.GetOr("TEMPERATURE", env.WithBitSize(strconv.ParseFloat, 64), 0.0), "The temperature to use for the model. Range is between 0 and 1. Set closer to 0 if your want output to be more deterministic but less creative. Defaults to 0.0.") // The temperature to use for the model.
	raw                  = flag.Bool("raw", false, "Prints the raw YAML output immediately. Defaults to false.")                                                                                                                                                                                   // Whether to print the raw YAML output immediately.
	usek8sAPI            = flag.Bool("use-k8s-api", env.GetOr("USE_K8S_API", strconv.ParseBool, false), "Whether to use the Kubernetes API to create resources with function calling. Defaults to false.")                                                                                         // Whether to use the Kubernetes API to create resources with function calling.
	k8sOpenAPIURL        = flag.String("k8s-openapi-url", env.GetOr("K8S_OPENAPI_URL", env.String, ""), "The URL to a Kubernetes OpenAPI spec. Only used if use-k8s-api flag is true.")                                                                                                            // The URL to a Kubernetes OpenAPI spec.
	debug                = flag.Bool("debug", env.GetOr("DEBUG", strconv.ParseBool, false), "Whether to print debug logs. Defaults to false.")                                                                                                                                                     // Whether to print debug logs.
)

// InitAndExecute initializes the application and executes the root command.
// It checks if the OpenAI key is provided and exits if it is not.
// It then executes the root command.
//this is the function that's being called from main.go file
func InitAndExecute() {
	if *openAIAPIKey == "" {
		fmt.Println("Please provide an OpenAI key.")
		os.Exit(1)
	}

	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

//being called from InitAndExecute function above (which is in turn called at main.go)
// RootCmd returns the root command for the kubectl-assistant CLI.
// It sets up the command with the necessary flags, pre-run actions, and the main run function.
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kubectl-assistant",
		Short:        "kubectl-assistant",
		Long:         "kubectl-assistant is a plugin for kubectl that allows you to interact with OpenAI GPT API.",
		Version:      version,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Set the log level to debug if the debug flag is enabled
//we're checking if debuf flag is enabled, then we will set log level as debuglevel
//and we will call the print debug flags function that prints the debug flags
//debug is a flag that we've only defined above in this file
			if *debug {
//log is basically the logrus package accessible to us via named import of log
				log.SetLevel(log.DebugLevel)
				printDebugFlags()
			}
		},
		RunE: func(_ *cobra.Command, args []string) error {
//the prompt we need to run is accessible via the args variable
//if the length of args is zero, means there is no prompt provided <kubectl something something>
			// Check if a prompt is provided
			if len(args) == 0 {
				return fmt.Errorf("prompt must be provided")
			}
//if lenght of args is not zero and there's actually a value, we proceed
			// Run the main logic of the CLI
//this is the main part of this function, where we essentially call the run function
			err := run(args) //calling the run function defined below
			if err != nil {
				return err
			}
//this function assigned to RunE returns an error, we have returned error earlier
//but we are reaching this line if no error was not found and all went well
//so we are returning nil for the error value here
			return nil
		},
	}

	// Add Kubernetes configuration flags to the command
	kubernetesConfigFlags.AddFlags(cmd.PersistentFlags())

	return cmd //cmd is of type cobra.Command, a struct in the cobra package
}

//we hae set the level for logs in the previous function as debug level
//we are calling this particular function from there and Debugf function
//from the logrus library prints te various flags with their values
//basically printing out the variables we have set above in this file
func printDebugFlags() {
	log.Debugf("openai-endpoint: %s", *openAIEndpoint)
	log.Debugf("openai-deployment-name: %s", *openAIDeploymentName)
	log.Debugf("azure-openai-map: %s", *azureModelMap)
	log.Debugf("temperature: %f", *temperature)
	log.Debugf("use-k8s-api: %t", *usek8sAPI)
	log.Debugf("k8s-openapi-url: %s", *k8sOpenAPIURL)
}

//main -> initandExecute -> RootCmd -> run function this is how execution is
// run is the main function that executes the CLI command.
// It takes a slice of arguments and returns an error if any.
func run(args []string) error {
	
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Create new OAI clients
//we're calling the function from completion.go file to generate new OpenAIClients
	oaiClients, err := newOAIClients() //calling the function to create new OAI clients, this func. is in completion.go file
	if err != nil {
		return err
	}

	var action, completion string
	//user can generate kubectl manifest file and then he needs to take an action, apply it
	//or not apply and we need to handle both scenarios
	for action != apply {
//if the user action is not to apply, then we append the action to the args object
		args = append(args, action)

		// Create a spinner to show processing status
		//using the go-spinner package to show processing
		s := spinner.NewSpinner("Processing...")
		if !*debug && !*raw {
			s.SetCharset([]string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"})
			s.Start()
		}

// Calling the gptCompletion func. (in completion.go file) by passing oaiClients which we just created above
//we also pass context, arguments and DeploymentName to this function
//gptCompletion gives us the response in string format, this func. is defined in completion.go file
		completion, err = gptCompletion(ctx, oaiClients, args, *openAIDeploymentName)
		//handling the error for calling the function above
		if err != nil {
			return err
		}
//s contains the spinner from the go-spinner package, we're stopping it on this line 
		s.Stop()
//raw is a flag we've created on the top of this file
		if *raw {
//if boolean for the raw flag is true, we print out the completion output received by calling the
//gptcompletion package above
			fmt.Println(completion)
			return nil
		}
//the manifest created by open ai for kubernetes is in the completion variable, we're printing it now
		// Print the manifest to be applied
		text := fmt.Sprintf("✨ Attempting to apply the following manifest:\n%s", completion)
		fmt.Println(text)

		// Prompt user for action, action being apply or dontApply
		//userActionPrompt is a function defined BELOW
		action, err = userActionPrompt()
		if err != nil {
			return err
		}

		if action == dontApply {
			return nil
		}
	}

	// Apply the manifest
	//right now we're outside the for loop for the 
	//action being not equal to apply, meaning here the action is to apply the settings
	//apply manifest is a function in kubernetes.go and this is why we call the function
	return applyManifest(completion)
}

// userActionPrompt prompts the user for an action and returns the selected action.
// If requireConfirmation is not set, it immediately returns the "apply" action.
// Otherwise, it presents a prompt to the user with options to apply or not apply.
// The selected action is returned as a string.
// If an error occurs during the prompt, it returns the "dontApply" action and the error.
func userActionPrompt() (string, error) {
//requireConfirmation is a flag we've defined on top of this file, basically ask permission
//of the user before applying the manifest file (true by default)
	if !*requireConfirmation {
//if we have kept requireConfirmation as false, we can directly apply the manifest
//we return 'apply' which is a string, as this function is supposed to return a string
		return apply, nil
	}
//defining variables result to return from this function and err to handle errors
	var result string
	var err error
//starting with a slice with 2 values - apply and dontApply
	items := []string{apply, dontApply}
//the context here is the kuberenetes context and this function is in the kubernetes.go file
//we need the context to be able to apply the manifest file
	currentContext, err := getCurrentContextName()
//formatting the string to ask the user to apply or not apply the manifest file
//formatting to display three options - apply, dontapply and reprompt
	label := fmt.Sprintf("Would you like to apply this? [%[1]s/%[2]s/%[3]s]", reprompt, apply, dontApply)
//if while getting the currentContext, there's no error, then we will also add
//currentContext and the label formatted above (with the 3 options) to the label	
	if err == nil {
		label = fmt.Sprintf("(context: %[1]s) %[2]s", currentContext, label)
	}
//promptui is a package we have imported above, SelectWithAdd function
//takes in the label we have just formatted above, items are apply and dontApply
//reprompt is a const we have defined at the top
	prompt := promptui.SelectWithAdd{
		Label:    label,
		Items:    items,
		AddLabel: reprompt,
	}
	//use the run function in the promptui package to run the prompt we have 
	//created above having labels, items etc. and get a result
	//we want to prompt the user with options to apply or dontapply and 
	//get the prompt in the result and then return that from this function
	_, result, err = prompt.Run()
	//if there was an error running it or getting prompt from the user, by default
	//we will not apply the manifest because we don't know what will happen
	if err != nil {
		return dontApply, err
	}
//returning the result from the prompt run
	return result, nil
}
