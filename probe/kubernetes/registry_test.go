package kubernetes_test

import (
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weaveworks/scope/probe/kubernetes"
	"github.com/weaveworks/scope/report"
	"github.com/weaveworks/scope/test"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"
)

type mockPod struct {
	p *api.Pod
}

func (p *mockPod) ID() string {
	return p.p.ObjectMeta.Namespace + "/" + p.p.ObjectMeta.Name
}

func (p *mockPod) Name() string {
	return p.p.ObjectMeta.Name
}

func (p *mockPod) Namespace() string {
	return p.p.ObjectMeta.Namespace
}

func (p *mockPod) Created() string {
	return p.p.ObjectMeta.CreationTimestamp.Format(time.RFC822)
}

func (p *mockPod) ContainerIDs() []string {
	ids := []string{}
	for _, container := range p.p.Status.ContainerStatuses {
		ids = append(ids, strings.TrimPrefix(container.ContainerID, "docker://"))
	}
	return ids
}

func (p *mockPod) GetNode() report.Node {
	return report.MakeNodeWith(map[string]string{
		kubernetes.PodID:           p.ID(),
		kubernetes.PodName:         p.Name(),
		kubernetes.Namespace:       p.Namespace(),
		kubernetes.PodCreated:      p.Created(),
		kubernetes.PodContainerIDs: strings.Join(p.ContainerIDs(), ", "),
	})
}

type mockClient struct {
	sync.RWMutex
	pods   []*api.Pod
	events []chan<- watch.Event
}

func (m *mockClient) AddEventListener(events chan<- watch.Event) error {
	m.Lock()
	defer m.Unlock()
	m.events = append(m.events, events)
	go func() {
		for _, p := range m.pods {
			events <- watch.Event{Type: watch.Added, Object: p}
		}
	}()
	return nil
}

func (m *mockClient) RemoveEventListener(events chan watch.Event) error {
	m.Lock()
	defer m.Unlock()
	for i, c := range m.events {
		if c == events {
			m.events = append(m.events[:i], m.events[i+1:]...)
		}
	}
	return nil
}

func (m *mockClient) send(event watch.Event) {
	m.RLock()
	defer m.RUnlock()
	for _, c := range m.events {
		c <- event
	}
}

var (
	pod1 = &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:              "pong",
			Namespace:         "ping",
			CreationTimestamp: util.Now(),
		},
		Status: api.PodStatus{
			HostIP: "1.2.3.4",
			ContainerStatuses: []api.ContainerStatus{
				{ContainerID: "container1"},
				{ContainerID: "container2"},
			},
		},
	}
	pod2 = &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:              "waff",
			Namespace:         "wiff",
			CreationTimestamp: util.Now(),
		},
		Status: api.PodStatus{
			HostIP: "1.2.3.5",
			ContainerStatuses: []api.ContainerStatus{
				{ContainerID: "container3"},
				{ContainerID: "container4"},
			},
		},
	}
)

func newMockClient() *mockClient {
	return &mockClient{
		pods: []*api.Pod{pod1},
	}
}

func setupStubs(mc *mockClient, f func()) {
	oldClient, oldNewPod := kubernetes.NewClientStub, kubernetes.NewPodStub
	defer func() {
		kubernetes.NewClientStub, kubernetes.NewPodStub = oldClient, oldNewPod
	}()

	kubernetes.NewClientStub = func(endpoint string) (kubernetes.Client, error) {
		return mc, nil
	}

	kubernetes.NewPodStub = func(p *api.Pod) kubernetes.Pod {
		return &mockPod{p}
	}

	f()
}

type podsByID []kubernetes.Pod

func (p podsByID) Len() int           { return len(p) }
func (p podsByID) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p podsByID) Less(i, j int) bool { return p[i].ID() < p[j].ID() }

func allPods(r kubernetes.Registry) []kubernetes.Pod {
	result := []kubernetes.Pod{}
	r.WalkPods(func(p kubernetes.Pod) {
		result = append(result, p)
	})
	sort.Sort(podsByID(result))
	return result
}

func TestRegistry(t *testing.T) {
	mc := newMockClient()
	setupStubs(mc, func() {
		registry, _ := kubernetes.NewRegistry("http://localhost:8080", 10*time.Second)
		defer registry.Stop()
		runtime.Gosched()

		{
			want := []kubernetes.Pod{&mockPod{pod1}}
			test.Poll(t, 10*time.Millisecond, want, func() interface{} {
				return allPods(registry)
			})
		}
	})
}

func TestRegistryEvents(t *testing.T) {
	mc := newMockClient()
	setupStubs(mc, func() {
		registry, _ := kubernetes.NewRegistry("http://localhost:8080", 10*time.Second)
		defer registry.Stop()
		runtime.Gosched()

		check := func(want []kubernetes.Pod) {
			test.Poll(t, 100*time.Millisecond, want, func() interface{} {
				return allPods(registry)
			})
		}

		{
			mc.Lock()
			mc.pods = append(mc.pods, pod2)
			mc.Unlock()
			mc.send(watch.Event{Type: watch.Added, Object: pod2})
			runtime.Gosched()
			check([]kubernetes.Pod{&mockPod{pod1}, &mockPod{pod2}})
		}

		{
			mc.send(watch.Event{Type: watch.Modified, Object: pod2})
			runtime.Gosched()
			check([]kubernetes.Pod{&mockPod{pod1}, &mockPod{pod2}})
		}

		{
			mc.Lock()
			mc.pods = mc.pods[:1]
			mc.Unlock()
			mc.send(watch.Event{Type: watch.Deleted, Object: pod2})
			runtime.Gosched()
			check([]kubernetes.Pod{&mockPod{pod1}})
		}

		{
			mc.Lock()
			mc.pods = []*api.Pod{}
			mc.Unlock()
			mc.send(watch.Event{Type: watch.Deleted, Object: pod1})
			runtime.Gosched()
			check([]kubernetes.Pod{})
		}
	})
}
