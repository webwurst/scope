package kubernetes

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
)

const (
	Namespace = "kubernetes_namespace"
)

// Vars exported for testing.
var (
	NewClientStub  = NewClient
	NewPodStub     = NewPod
	NewServiceStub = NewService
)

// Registry keeps track of running kubernetes pods and services
type Registry interface {
	Stop()
	WalkPods(f func(Pod) error) error
	WalkServices(f func(Service) error) error
}

type registry struct {
	resyncPeriod time.Duration
	client       Client
}

// Client interface for mocking.
type Client interface {
	Stop()
	ListPods() ([]*api.Pod, error)
	ListServices() ([]*api.Service, error)
}

// NewRegistry returns a usable Registry. Don't forget to Stop it.
func NewRegistry(apiAddr string, resyncPeriod time.Duration) (Registry, error) {
	client, err := NewClientStub(apiAddr, resyncPeriod)
	if err != nil {
		return nil, err
	}

	return &registry{
		resyncPeriod: resyncPeriod,
		client:       client,
	}, nil
}

// Stop stops the Docker registry's event subscriber.
func (r *registry) Stop() {
	r.client.Stop()
}

func (r *registry) WalkPods(f func(Pod) error) error {
	pods, err := r.client.ListPods()
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if err := f(NewPodStub(pod)); err != nil {
			return err
		}
	}
	return nil
}

func (r *registry) WalkServices(f func(Service) error) error {
	services, err := r.client.ListServices()
	if err != nil {
		return err
	}
	for _, service := range services {
		if err := f(NewServiceStub(service)); err != nil {
			return err
		}
	}
	return nil
}
