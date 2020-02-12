package config

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform/dag"
	"golang.org/x/xerrors"
)

// Status defines the current state of a resource
type Status string

// ResourceType is the type of the resource
type ResourceType string

// Applied means the resrouce has been successfully created
const Applied Status = "applied"

// PendingCreation means the resource has not yet been created
const PendingCreation Status = "pending_creation"

// PendingModification means the resource has been created but is pending an update
const PendingModification Status = "pending_modification"

// Failed means the resource failed during creation
const Failed Status = "failed"

type Resource interface {
	Info() *ResourceInfo
}

// Resource is the embedded type for any config resources
type ResourceInfo struct {
	// Name is the name of the resource
	Name string `json:"name"`
	// Type is the type of resource, this is the text representation of the golang type
	Type ResourceType `json:"type"`
	// Status is the current status of the resource, this is always PendingCreation initially
	Status Status `json:"status"`
	// DependsOn is a list of objects which must exist before this resource can be applied
	DependsOn []string `json:"depends_on"`
}

func (r *ResourceInfo) Info() *ResourceInfo {
	return r
}

// Config defines the stack config
type Config struct {
	Blueprint *Blueprint `json:"blueprint"`
	Resources []Resource `json:"resources"`
}

// ResourceNotFoundError is thrown when a resource could not be found
type ResourceNotFoundError struct {
	Name string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("Resource not found: %s", e.Name)
}

// ResourceExistsError is thrown when a resource already exists in the resource list
type ResourceExistsError struct {
	Name string
}

func (e ResourceExistsError) Error() string {
	return fmt.Sprintf("Resource already exists: %s", e.Name)
}

// New creates a new Config with the default WAN network
func New() *Config {
	c := &Config{}

	// add the default WAN
	wan := NewNetwork("wan")
	wan.Subnet = "10.200.0.0/16"

	return c
}

// FindResource returns the resource for the given name
// name is defined with the convention [type].[name]
// if a resource can not be found resource will be null and an
// error will be returned
//
// e.g. to find a cluster named k3s
// r, err := c.FindResource("cluster.k3s")
func (c *Config) FindResource(name string) (Resource, error) {
	parts := strings.Split(name, ".")

	for _, r := range c.Resources {
		if r.Info().Type == ResourceType(parts[0]) && r.Info().Name == parts[1] {
			return r, nil
		}
	}

	return nil, ResourceNotFoundError{name}
}

// AddResource adds a given resource to the resource list
// if the resource already exists an error will be returned
func (c *Config) AddResource(r Resource) error {
	if _, err := c.FindResource(fmt.Sprintf("%s.%s", r.Info().Type, r.Info().Name)); err != nil {
		if xerrors.Is(err, ResourceNotFoundError{}) {
			return ResourceExistsError{r.Info().Name}
		}
	}

	c.Resources = append(c.Resources, r)

	return nil
}

// DoYaLikeDAGs dags? yeah dags! oh, dogs.
// https://www.youtube.com/watch?v=ZXILzUpVx7A&t=0s
func (c *Config) DoYaLikeDAGs() (*dag.AcyclicGraph, error) {
	graph := &dag.AcyclicGraph{}

	// Loop over all resources and add to dag
	for _, resource := range c.Resources {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range c.Resources {
		for _, d := range resource.Info().DependsOn {
			dependency, err := c.FindResource(d)
			if xerrors.Is(err, ResourceNotFoundError{}) {
				return nil, xerrors.Errorf("Could not build graph from resources: %w", err)
			}

			graph.Connect(dag.BasicEdge(dependency, resource))
		}
	}

	return graph, nil
}

// ResourceCount defines the number of resources in a config
func (c *Config) ResourceCount() int {
	return len(c.Resources)
}
