package cli

import (
	"bytes"
	"context"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

const defaultNamespace = "default"

//completion string received here is the yaml file returned by chatgptcompletion api
//we are calling this functin from root.go after asking the user whether he wants to apply
// applyManifest applies the provided manifest to the Kubernetes cluster.
func applyManifest(completion string) error {
	// Retrieve the Kubernetes configuration file path, just returns the file path
	kubeConfig := getKubeConfig()

	//pass the file path and get the config values
	// Build the Kubernetes client configuration from the provided kubeConfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return err
	}

	//pass the config values to get a client, which we can access through 'c'
	// Create a new Kubernetes client using the configuration
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Create a dynamic client for working with unstructured objects
	//The dynamic package in Kubernetes client libraries (like client-go in Go) provides a client for working with arbitrary resources in a dynamic fashion. 
	//Instead of using a strongly typed client for each specific resource (e.g., Pods, Services), the dynamic client allows you to interact with resources without knowing their types at compile time.
	dd, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	var namespace string
	//we defined a variable kubernetesConfigFlags in root.go file to determine config flags for kubernetes
	//if their namespace is not provided, then we get defaultNameSpace
	if *kubernetesConfigFlags.Namespace == "" {
		// If the namespace flag is not provided, retrieve the default namespace from the kubeConfig file
		//call the getConfig function defined below
		clientConfig, err := getConfig(kubeConfig)
		if err != nil {
			return err
		}
		//if even after getting kuubeConfig, in clientConfig, there's no namespace defined,
		//use defaultNamespace
		if clientConfig.Contexts[clientConfig.CurrentContext].Namespace == "" {
			//defaultNameSpace constant is defined above in this file
			namespace = defaultNamespace
		} else {
			namespace = clientConfig.Contexts[clientConfig.CurrentContext].Namespace
		}
	} else {
		//else if configFlag's namespace has a value set, use that
		// Use the provided namespace flag
		namespace = *kubernetesConfigFlags.Namespace
	}

	//we have received completion string as args in this function, before we can apply it
	//as manifest, we need to convert it
	// Convert the completion string to a byte array
	manifest := []byte(completion)

	// Create a YAML or JSON decoder to decode the manifest
	//note we are using YAMLorJSONDecoder, meaning we are prepared for both data types
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 100)

	// Decode and apply each object in the manifest
	for {
		//runtime.RawExtension is a type provided by the Kubernetes client libraries. 
		//It is used to represent arbitrary JSON or yaml data without unmarshaling it into a specific struct. 
		//This can be useful in situations where you want to work with Kubernetes resources that have dynamic or unknown structures.
		var rawObj runtime.RawExtension
		//decoder already has the manifest file, we want to structure it like rawObj
		//and decode it into the rawObj variable, since we don't know the structure of the JSON data
		//at compile time, so need RawExtension, we will further process rawObj now
		
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		// Decode the raw object into a typed object using the YAML decoding serializer
		//here obj is the decoded object for data that was stored in rawObj 
		//we basically created a new yaml decodingSerializer to process JSON data into something golang understands
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		//gvk is groupVersionKind data of the decoded object, provides info about the API group, version and kind of the resource
		
		if err != nil {
			return err
		}

		// Convert the strongly typed object that golang understands to an unstructured map
		//so that we can process it further
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}

		//we now have an unstructured map and need an unstructured object from it
		// Create an unstructured object from the unstructured map
		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		// Get the API group resources using the Kubernetes discovery API
		//c is our kubernetes client
		//get a mapping of API groups and the associated resources available in a Kubernetes cluster.
		gr, err := restmapper.GetAPIGroupResources(c.Discovery())
		if err != nil {
			return err
		}

		// Create a REST mapper using the API group resources
		//the gr variable contains info about the API group resources, we got this from above
		//
		mapper := restmapper.NewDiscoveryRESTMapper(gr)

		// Get the REST mapping for the object's group version kind, since we want to call REST API to apply manifest
		// A REST mapper is responsible for mapping group-version-resource (GVR) identifiers to their corresponding REST endpoints.
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		//we need a dynamic resource interfece and dri is a short form for it
		//This variable is intended to represent an interface for interacting with dynamic (untyped) Kubernetes resources.
//This interface defines methods for performing CRUD (Create, Read, Update, Delete) operations on Kubernetes resources without requiring a statically generated client for each specific resource type.
		
		var dri dynamic.ResourceInterface
		//mapping has the REST mapping available and we're checking if the namespace matches

//In Kubernetes, a namespace is a way to divide cluster resources between multiple users (via resource units like pods, services, etc.). 
//It provides a scope for names, meaning that names of resources must be unique within a namespace, but they can be repeated across namespaces.
//A "namespace scope" in the context of your code refers to whether a particular Kubernetes resource is bound within the context of a namespace. 
//When a resource is namespace-scoped, it means that it exists within a specific namespace, and operations on that resource are limited to that namespace.
		
//This code checks whether the resource represented by mapping is namespace-scoped.
//meta.RESTScopeNameNamespace refers to a constant value defined in the k8s.io/apimachinery/pkg/api/meta package of the Kubernetes Go client library. 
//This constant represents the string identifier for the namespace scope of a Kubernetes resource.
if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// check if namespace for unstructured obj is empty,
			//set the namespace if not already set
			if unstructuredObj.GetNamespace() == "" {
				unstructuredObj.SetNamespace(namespace)
			}
			// Create a resource interface for the namespaced object
			dri = dd.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			//if namespace doesn't match, we will
			// Create a resource interface for the non-namespaced object
			dri = dd.Resource(mapping.Resource)
		}

		// Apply the object to the cluster using the dynamic client
		//this line is the main business logic where the manifest is applied
		//the purpose of the above if-else statement was to set the value for dri so we can use it to apply manifest
		if _, err := dri.Apply(context.Background(), unstructuredObj.GetName(), unstructuredObj, metav1.ApplyOptions{FieldManager: "application/apply-patch"}); err != nil {
			return err
		}
	}
//this function applies manifest and doesn't return any value, just an error,
//so if everything went well, we'll return nil as the error
	return nil
}

// getKubeConfig returns the path to the Kubernetes configuration file.
func getKubeConfig() string {
	var kubeConfig string

	
	//usually you'd find the config file in home directory in the path ~/.kube/config
	//but you might have a separate kubeConfig, if you don't have it or
	// If the KubeConfig flag is not set, use the default path: ~/.kube/config.
	if *kubernetesConfigFlags.KubeConfig == "" {
		kubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	} else {
		// else, If the KubeConfig flag is set, use the provided path.
		kubeConfig = *kubernetesConfigFlags.KubeConfig
	}

	return kubeConfig
}

// getConfig retrieves the Kubernetes configuration from the specified kubeConfig file.
func getConfig(kubeConfig string) (api.Config, error) {
	// Create a new NonInteractiveDeferredLoadingClientConfig with the specified kubeConfig file path.
	// This config will be used to load the Kubernetes configuration.
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfig},
		&clientcmd.ConfigOverrides{
			CurrentContext: "",
		}).RawConfig()
	if err != nil {
		return api.Config{}, err
	}

	// Return the parsed configuration.
	return config, nil
}

//we are calling this function in the root.go file and we need the context to be able
//to apply the manifest settings
// getCurrentContextName returns the name of the current context in the Kubernetes configuration.
//first we will call the getKubeConfig func. to get the config file
//then we call getConfig func. to retrieve the actual kube config from the file
func getCurrentContextName() (string, error) {
	// getKubeConfig retrieves the path to the Kubernetes configuration file.
	kubeConfig := getKubeConfig()

	// getConfig reads the Kubernetes configuration file and returns the parsed configuration.
	config, err := getConfig(kubeConfig)
	if err != nil {
		return "", err
	}

	// Extract the name of the current context from the configuration.
	currentContext := config.CurrentContext

	return currentContext, nil
}
