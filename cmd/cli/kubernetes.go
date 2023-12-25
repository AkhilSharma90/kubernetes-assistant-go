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

// applyManifest applies the provided manifest to the Kubernetes cluster.
func applyManifest(completion string) error {
	// Retrieve the Kubernetes configuration file path
	kubeConfig := getKubeConfig()

	// Build the Kubernetes client configuration from the provided kubeConfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return err
	}

	// Create a new Kubernetes client using the configuration
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Create a dynamic client for working with unstructured objects
	dd, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	var namespace string
	if *kubernetesConfigFlags.Namespace == "" {
		// If the namespace flag is not provided, retrieve the default namespace from the kubeConfig file
		clientConfig, err := getConfig(kubeConfig)
		if err != nil {
			return err
		}
		if clientConfig.Contexts[clientConfig.CurrentContext].Namespace == "" {
			namespace = defaultNamespace
		} else {
			namespace = clientConfig.Contexts[clientConfig.CurrentContext].Namespace
		}
	} else {
		// Use the provided namespace flag
		namespace = *kubernetesConfigFlags.Namespace
	}

	// Convert the completion string to a byte array
	manifest := []byte(completion)

	// Create a YAML or JSON decoder to decode the manifest
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 100)

	// Decode and apply each object in the manifest
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		// Decode the raw object into a typed object using the YAML decoding serializer
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return err
		}

		// Convert the typed object to an unstructured map
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}

		// Create an unstructured object from the unstructured map
		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		// Get the API group resources using the Kubernetes discovery API
		gr, err := restmapper.GetAPIGroupResources(c.Discovery())
		if err != nil {
			return err
		}

		// Create a REST mapper using the API group resources
		mapper := restmapper.NewDiscoveryRESTMapper(gr)

		// Get the REST mapping for the object's group version kind
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		var dri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// If the object is namespaced, set the namespace if not already set
			if unstructuredObj.GetNamespace() == "" {
				unstructuredObj.SetNamespace(namespace)
			}
			// Create a resource interface for the namespaced object
			dri = dd.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			// Create a resource interface for the non-namespaced object
			dri = dd.Resource(mapping.Resource)
		}

		// Apply the object to the cluster using the dynamic client
		if _, err := dri.Apply(context.Background(), unstructuredObj.GetName(), unstructuredObj, metav1.ApplyOptions{FieldManager: "application/apply-patch"}); err != nil {
			return err
		}
	}

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

// getCurrentContextName returns the name of the current context in the Kubernetes configuration.
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
