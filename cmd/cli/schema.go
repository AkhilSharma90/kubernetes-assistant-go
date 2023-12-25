package cli
//COMPLETE
import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

//this func. is being called in both fetchResourceName and fetchSchemaForResource functions below
// fetchK8sSchema fetches the Kubernetes schema either from the Kubernetes API server or from a specified URL.
// It returns the schema as a map[string]interface{} and an error if any.
func fetchK8sSchema() (map[string]interface{}, error) {
	var body []byte
	var err error
//if the APIURL for k8s hasnt' been specified, we use exec package to create a command with kubectl
//this is done in the runKubectlCommand function that's called from here
	if *k8sOpenAPIURL == "" {
		log.Debugf("Fetching schema from Kubernetes API server")
//getKubeConfig function is defined in kubernetes.go file 
		kubeConfig := getKubeConfig()
//runKubectlCommand is defined below in this file, call it and get the response
//in the body variable
		body, err = runKubectlCommand("get", "--raw", "/openapi/v2", "--kubeconfig", kubeConfig)
		if err != nil {
			return nil, err
		}
	} else {
		//if k8s API URL is set, then we just make a GET request to it and get response
		log.Debugf("Fetching schema from %s", *k8sOpenAPIURL)
		response, err := http.Get(*k8sOpenAPIURL)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
//read the response body, and available to us in body variable
		body, err = io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
	}
//create a vraible schema to unmarshal content from the body into map format
	var schema map[string]interface{}
	err = json.Unmarshal(body, &schema)
	if err != nil {
		return nil, err
	}
//this function returns the map schema
	return schema, nil
}

// fetchResourceNames fetches the resource names that match the given resourceName.
// It retrieves the Kubernetes schema and searches for resource names in the schema definitions.
// The resourceName parameter is case-insensitive.
// It returns a slice of resource names and an error if fetching the schema or searching for resource names fails.
//this function is called in functions.go file
func fetchResourceNames(resourceName string) ([]string, error) {
	//calling the function defined just above in this file
	schema, err := fetchK8sSchema()
	if err != nil {
		return nil, err
	}
	//logging out the resourceName received as args
	log.Debugf("fetching resource name %s", resourceName)
//the schema variable (map) will have values for definitions and we capture that
//in the variable called definitions
	definitions, ok := schema["definitions"].(map[string]interface{})
	if !ok {
		return nil, errors.New("unable to assert schema definitions")
	}
//defining a slice resourceNames which we will return from this function
	var resourceNames []string
	//small process of ranging over the definitions and appending them to 
	//resourceNames slice
	for k := range definitions {
		if strings.Contains(strings.ToLower(k), strings.ToLower(resourceName)) {
			resourceNames = append(resourceNames, k)
		}
	}

	return resourceNames, nil
}

// fetchSchemaForResource fetches the schema for a given resource type.
// It returns the resource schema as a map[string]interface{} and an error if any.
func fetchSchemaForResource(resourceType string) (map[string]interface{}, error) {
	// Fetch the Kubernetes schema
	schema, err := fetchK8sSchema()
	if err != nil {
		return nil, err
	}

	// Extract the definitions from the schema
	definitions, ok := schema["definitions"].(map[string]interface{})
	if !ok {
		return nil, errors.New("unable to assert schema definitions")
	}

	// Fetch the resource schema for the given resource type
	log.Debugf("fetching resource schema %s", resourceType)
	//same steps as the previous function only thing changed is instead of getting
	//just the definitions, we're extracting the resourceType
	if resourceSchema, ok := definitions[resourceType]; ok {
		rs, ok := resourceSchema.(map[string]interface{})
		if !ok {
			return nil, errors.New("unable to assert resource schema")
		}
		return rs, nil
	}

	// Return an error if the resource schema is not found
	return nil, errors.New("unable to find resource schema")
}

// runKubectlCommand executes a kubectl command with the provided arguments and returns the output as a byte slice.
//function is being called in the fetchk8sSchema function above
func runKubectlCommand(args ...string) ([]byte, error) {
	// Create a new exec.Command with "kubectl" as the command and the provided arguments.
	//formulate a command for kubernetes, the command will be kubectl ls or something
	
	//to run a kubernetes command, we'd need to attach arguments with kubectl keyword
	//and run it as a command and this is done with the exec package that enables us
	//to create our own commands and run them
	cmd := exec.Command("kubectl", args...)

	// Create a buffer to store the command output.
	//a variable out has been defined as bytes.Buffer or temporary storage
	var out bytes.Buffer
	//we assign the std output of the command as out (which is bytes.Buffer)
	cmd.Stdout = &out

	// Run the command and wait for it to complete.
	//we run the command we had formulated above
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	// Return the command output as a byte slice.
	//this is the output from running the command
	return out.Bytes(), nil
}
