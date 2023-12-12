package cli

import (
	"encoding/json"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type schemaNames struct {
	ResourceName string `json:"resourceName"`
}

var findSchemaNames openai.FunctionDefinition = openai.FunctionDefinition{
	Name:        "findSchemaNames",
	Description: "Get the list of possible fully-namespaced names for a specific Kubernetes resource. E.g. given `Container` return `io.k8s.api.core.v1.Container`. Given `EnvVarSource` return `io.k8s.api.core.v1.EnvVarSource`",
	Parameters: jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"resourceName": {
				Type:        jsonschema.String,
				Description: "The name of a Kubernetes resource or field.",
			},
		},
		Required: []string{"resourceName"},
	},
}

// Run fetches resource names based on the provided resource name and returns them as a string.
func (s *schemaNames) Run() (content string, err error) {
	// Fetch resource names
	names, err := fetchResourceNames(s.ResourceName)
	if err != nil {
		return "", err
	}

	// Join names with newline separator
	return strings.Join(names, "\n"), nil
}

type schema struct {
	ResourceType string `json:"resourceType"`
}

var getSchema openai.FunctionDefinition = openai.FunctionDefinition{
	Name:        "getSchema",
	Description: "Get the OpenAPI schema for a Kubernetes resource",
	Parameters: jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"resourceType": {
				Type:        jsonschema.String,
				Description: "The type of the Kubernetes resource or object (e.g. subresource). Must be fully namespaced, as returned by findSchemaNames",
			},
		},
		Required: []string{"resourceType"},
	},
}

// Run executes the schema fetching process and returns the schema content as a string.
// It fetches the schema for the specified resource type, marshals it into JSON, and returns the JSON string.
func (s *schema) Run() (content string, err error) {
	// Fetch the schema for the specified resource type
	schema, err := fetchSchemaForResource(s.ResourceType)
	if err != nil {
		return "", err
	}

	// Marshal the schema into JSON
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}

	return string(schemaBytes), nil
}

// funcCall is a function that handles different function calls based on the provided call name.
// It takes a pointer to an openai.FunctionCall as input and returns a string and an error.
func funcCall(call *openai.FunctionCall) (string, error) {
	switch call.Name {
	case findSchemaNames.Name:
		// Unmarshal the call arguments into a schemaNames struct
		var f schemaNames
		if err := json.Unmarshal([]byte(call.Arguments), &f); err != nil {
			return "", err
		}
		// Call the Run method of the schemaNames struct and return the result
		return f.Run()
	case getSchema.Name:
		// Unmarshal the call arguments into a schema struct
		var f schema
		if err := json.Unmarshal([]byte(call.Arguments), &f); err != nil {
			return "", err
		}
		// Call the Run method of the schema struct and return the result
		return f.Run()
	}
	return "", nil
}
