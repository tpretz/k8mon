package main

import (
	"context"
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {

	// define a variable called "kubeconfig" to store the path to the kubeconfig file
	var kubeconfig string

	// check if home directory path is not empty.
	// if not, contruct the path to kubeconfig file
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// build configuration to connect to a K8s cluster based on command-line flags and provided kubeconfig path
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	// if external kubeconfig file either wasn't found, wasn't accessible, or was invalid
	// throw error
	if err != nil {
		fmt.Println("Falling back to in-cluster config")

		// retrieve configuration from environment variables and service account tokens available within pod
		config, err = rest.InClusterConfig()

		// if even the in-cluster configuration setup fails, raise panic
		if err != nil {
			panic(err.Error())
		}
	}
	// create new dynamic client for interacting with K8s API
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// define variable, "thefoothebar" holding definition of custom resource
	// specifies that resource belongs to API group "myk8s.io", with version "v1", & plural name of resource
	monitors := schema.GroupVersionResource{Group: "k8mon.tpretz.com", Version: "v1", Resource: "monitors"}

	// creates new informer for specific resource i.e. "thefoothebar"
	// this informer watches for changes to resource & maintains a local cache of all resources of this type
	informer := cache.NewSharedIndexInformer(
		// callbacks defining how to list and watch the resources, respectively
		&cache.ListWatch{
			// ListFunc initially populates informer's cache
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return dynClient.Resource(monitors).Namespace("").List(context.TODO(), options)
			},
			// WatchFunc keeps cache updated with any changes
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return dynClient.Resource(monitors).Namespace("").Watch(context.TODO(), options)
			},
		},

		// specifies that informer is for unstructured data
		// unstructured data represent any K8s resource without needing a predefined struct
		&unstructured.Unstructured{},

		// resync period of 0 means that informer will not resync the resources unless explicitly triggered
		0,
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Printf("New monitor added: %s\n", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			fmt.Printf("Monitor updated: %s\n", newObj)
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Printf("Monitor deleted: %s\n", obj)
		},
	})

	// starts the informer
	stop := make(chan struct{})
	defer close(stop)
	go informer.Run(stop)

	// wait for the informer's cache to sync
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		panic("failed to sync")
	}

	<-stop
}
