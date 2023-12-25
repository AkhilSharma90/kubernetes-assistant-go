package cli
//COMPLETE
import (
	"encoding/json"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

//name for the schema of kubernetes resource, defining the struct here
//and methods for the struct below
type schemaNames struct {
	ResourceName string `json:"resourceName"`
}

//defining findSchemaNames as an openAI functionDefiniion
//we pass this function when we make a chat completion request to openai in openai.go file
//open ai package is available to us as openai due to named import
//we are defining these as variables that are of type openai function definition
var findSchemaNames openai.FunctionDefinition = openai.FunctionDefinition{
	Name:        "findSchemaNames",
	Description: "Get the list of possible fully-namespaced names for a specific Kubernetes resource. E.g. given `Container` return `io.k8s.api.core.v1.Container`. Given `EnvVarSource` return `io.k8s.api.core.v1.EnvVarSource`",
	//parameters is a field required to define something as open ai function
	//it has a type, which is usually object and some properties, in our case
	//we just have resourceName, which is also the field from the schemaNames struct defined above
	//it will have a type (string, since it's a single field from struct as defined above) and a description
	Parameters: jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"resourceName": {
				Type:        jsonschema.String,
				Description: "The name of a Kubernetes resource or field.",
			},
		},
		//the JSON object needs to have resourceName
		Required: []string{"resourceName"},
	},
}

// Run fetches resource names based on the provided resource name and returns them as a string.
//being called in the funcCall function below
func (s *schemaNames) Run() (content string, err error) {
	// Fetch resource names
	//s is a struct with field ResourceName and that's what we're accessing here
	//calling this func. defined in schema.go
	//the function just gets resourcenames in string format
	names, err := fetchResourceNames(s.ResourceName)
	if err != nil {
		return "", err
	}

	// Join names with newline separator in single string and send
	return strings.Join(names, "\n"), nil
}


//just like we defined schema for kubernertes resource name, this one is for
//resource type of kubernetes
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
	//this function is defined in schema.go file and gets the resourceType
	schema, err := fetchSchemaForResource(s.ResourceType)
	if err != nil {
		return "", err
	}

	// Marshal the schema into JSON because we will return it as string from this func.
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}

	return string(schemaBytes), nil
}

// funcCall is a function that handles different function calls based on the provided call name.
// It takes a pointer to an openai.FunctionCall as input and returns a string and an error.
//we call this function from openai.go file in the chatCompletion function, when we have received response
//from open ai and want to implement the function received in response
func funcCall(call *openai.FunctionCall) (string, error) {
	switch call.Name {
	case findSchemaNames.Name:
		// Unmarshal the call arguments into a schemaNames struct
		//schemaNames is a struct defined above in this file
		var f schemaNames
		//call is the open ai function call, we unmarshall it into schemaNames
		if err := json.Unmarshal([]byte(call.Arguments), &f); err != nil {
			return "", err
		}
		// Call the Run method of the schemaNames struct and return the result
		//Run for schemaNames method has been defined above in this file
		//since we're calling the method for f, a particular instance of schemaNames,
		//we have unmarshalles the arguments into schemaNames above
		return f.Run()
	case getSchema.Name:
		// Unmarshal the call arguments into a schema struct
		//schema struct has been defined above and f is a variable of that type
		var f schema
		//unmarchalling if the case is getSchema.Name
		if err := json.Unmarshal([]byte(call.Arguments), &f); err != nil {
			return "", err
		}
		// Call the Run method of the schema struct and return the result
		//calling the Run method of the schema struct, has been defined above
		return f.Run()
	}
	return "", nil
}
