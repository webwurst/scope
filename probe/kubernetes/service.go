package kubernetes

import (
	"fmt"
	"strings"
	"time"

	"github.com/weaveworks/scope/report"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
)

const (
	ServiceID      = "kubernetes_service_id"
	ServiceName    = "kubernetes_service_name"
	ServiceCreated = "kubernetes_service_created"
	ServicePorts   = "kubernetes_service_ports"
	ServiceIPs     = "kubernetes_service_ips"
)

// Service represents a Kubernetes service
type Service interface {
	ID() string
	Name() string
	Namespace() string
	GetNode() report.Node
	Selector() labels.Selector
}

type service struct {
	*api.Service
}

func NewService(s *api.Service) Service {
	return &service{Service: s}
}

func (s *service) ID() string {
	return s.ObjectMeta.Namespace + "/" + s.ObjectMeta.Name
}

func (s *service) Name() string {
	return s.ObjectMeta.Name
}

func (s *service) Namespace() string {
	return s.ObjectMeta.Namespace
}

func (s *service) Selector() labels.Selector {
	return labels.SelectorFromSet(labels.Set(s.Spec.Selector))
}

func (s *service) GetNode() report.Node {
	return report.MakeNodeWith(map[string]string{
		ServiceID:      s.ID(),
		ServiceName:    s.Name(),
		ServiceCreated: s.ObjectMeta.CreationTimestamp.Format(time.RFC822),
		Namespace:      s.Namespace(),
		ServicePorts:   strings.Join(s.ports(), ""),
		ServiceIPs:     strings.Join(s.ips(), " "),
	})
}

func (s *service) ports() []string {
	result := []string{}
	for _, port := range s.Spec.Ports {
		targetPort := port.TargetPort.String()
		if targetPort == "" || targetPort == "0" {
			targetPort = fmt.Sprint(port.Port)
		}
		result = append(result, fmt.Sprintf("%d/%s->%s", port.Port, port.Protocol, targetPort))
	}
	return result
}

func (s *service) ips() []string {
	ips := []string{s.Spec.ClusterIP}
	if s.Spec.Type == api.ServiceTypeClusterIP {
		return ips
	}

	// TODO: Get node ips here
	nodeIPs := []string{}
	ips = append(ips, nodeIPs...)
	if s.Spec.Type == api.ServiceTypeNodePort {
		return ips
	}

	ips = append(ips, s.Spec.ExternalIPs...)
	if s.Spec.LoadBalancerIP != "" {
		ips = append(ips, s.Spec.LoadBalancerIP)
	}

	for _, ingress := range s.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			ips = append(ips, ingress.IP)
		} else if ingress.Hostname != "" {
			ips = append(ips, ingress.Hostname)
		}
	}
	return ips
}
