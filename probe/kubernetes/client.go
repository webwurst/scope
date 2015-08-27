package kubernetes

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

type client struct {
	quit             chan struct{}
	client           *unversioned.Client
	podReflector     *cache.Reflector
	serviceReflector *cache.Reflector
	podStore         *cache.StoreToPodLister
	serviceStore     *cache.StoreToServiceLister
}

func NewClient(addr string, resyncPeriod time.Duration) (Client, error) {
	c, err := unversioned.New(&unversioned.Config{Host: addr})
	if err != nil {
		return nil, err
	}

	podListWatch := cache.NewListWatchFromClient(c, "pods", api.NamespaceAll, fields.Everything())
	podStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	podReflector := cache.NewReflector(podListWatch, &api.Pod{}, podStore, resyncPeriod)

	serviceListWatch := cache.NewListWatchFromClient(c, "services", api.NamespaceAll, fields.Everything())
	serviceStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	serviceReflector := cache.NewReflector(serviceListWatch, &api.Service{}, serviceStore, resyncPeriod)

	quit := make(chan struct{})
	podReflector.RunUntil(quit)
	serviceReflector.RunUntil(quit)

	return &client{
		quit:             quit,
		client:           c,
		podReflector:     podReflector,
		podStore:         &cache.StoreToPodLister{podStore},
		serviceReflector: serviceReflector,
		serviceStore:     &cache.StoreToServiceLister{serviceStore},
	}, nil
}

func (c *client) ListPods() ([]*api.Pod, error) {
	return c.podStore.List(labels.Everything())
}

func (c *client) ListServices() ([]*api.Service, error) {
	list, err := c.serviceStore.List()
	items := []*api.Service{}
	for i := range list.Items {
		items = append(items, &(list.Items[i]))
	}
	return items, err
}
func (c *client) Stop() {
	close(c.quit)
}
