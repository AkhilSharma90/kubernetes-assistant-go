package cli

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

// fetchK8sSchema fetches the Kubernetes schema either from the Kubernetes API server or from a specified URL.
// It returns the schema as a map[string]interface{} and an error if any.
func fetchK8sSchema() (map[string]interface{}, error) {
	var body []byte
	var err error

	if *k8sOpenAPIURL == "" {
		log.Debugf("Fetching schema from Kubernetes API server")
		// TODO: can we use kube discovery cache here?
		kubeConfig := getKubeConfig()
		body, err = runKubectlCommand("get", "--raw", "/openapi/v2", "--kubeconfig", kubeConfig)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO: we should cache this or read from a local file
		log.Debugf("Fetching schema from %s", *k8sOpenAPIURL)
		response, err := http.Get(*k8sOpenAPIURL)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		body, err = io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
	}

	var schema map[string]interface{}
	err = json.Unmarshal(body, &schema)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

// fetchResourceNames fetches the resource names that match the given resourceName.
// It retrieves the Kubernetes schema and searches for resource names in the schema definitions.
// The resourceName parameter is case-insensitive.
// It returns a slice of resource names and an error if fetching the schema or searching for resource names fails.
func fetchResourceNames(resourceName string) ([]string, error) {
	schema, err := fetchK8sSchema()
	if err != nil {
		return nil, err
	}
	log.Debugf("fetching resource name %s", resourceName)

	definitions, ok := schema["definitions"].(map[string]interface{})
	if !ok {
		return nil, errors.New("unable to assert schema definitions")
	}

	var resourceNames []string
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
func runKubectlCommand(args ...string) ([]byte, error) {
	// Create a new exec.Command with "kubectl" as the command and the provided arguments.
	cmd := exec.Command("kubectl", args...)

	// Create a buffer to store the command output.
	var out bytes.Buffer
	cmd.Stdout = &out

	// Run the command and wait for it to complete.
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	// Return the command output as a byte slice.
	return out.Bytes(), nil
}
