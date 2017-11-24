package tunnel

import (
	"bytes"
	"fmt"

	// v1 "k8s.io/api/core/v1"
	"github.com/golang/glog"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/pkg/api/v1"
	ext_v1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// Tunnel is the interface for different implementation of
// the cloudflare-warp tunnel, matching an external hostname
// to a kubernetes service
type Tunnel interface {

	// Config returns the tunnel configuration
	Config() Config

	// Start the tunnel, making it active
	Start() error

	// Stop the tunnel, making it inactive
	Stop() error

	// Active tells whether the tunnel is active or not
	Active() bool

	// TearDown cleans up all external resources
	TearDown() error

	// CheckStatus validates the current state of the tunnel
	CheckStatus() error
}

type Config struct {
	ServiceName      string
	Namespace        string
	ExternalHostname string
	CertificateName  string
}

// compare origin.tunnelConfig
// type TunnelConfig struct {
// 	EdgeAddr          string
// 	OriginUrl         string
// 	Hostname          string
// 	OriginCert        []byte
// 	TlsConfig         *tls.Config
// 	Retries           uint
// 	HeartbeatInterval time.Duration
// 	MaxHeartbeats     uint64
// 	ClientID          string
// 	ReportedVersion   string
// 	LBPool            string
// 	Tags              []tunnelpogs.Tag
// 	ConnectedSignal   h2mux.Signal
// }

// TunnelPodManager manages a single tunnel created and managed in a pod
type TunnelPodManager struct {
	id             string
	client         kubernetes.Interface
	config         *Config
	replicaSetName string
	active         bool
}

// NewTunnelPodManager creates a new pod-manager in a namespace, and can create, destroy and
// check the status of that pod.
func NewTunnelPodManager(client kubernetes.Interface, config *Config) (Tunnel, error) {

	manager := TunnelPodManager{
		id:     utilrand.String(8),
		client: client,
		config: config,
	}

	err := manager.init()
	if err != nil {
		return nil, err
	}
	return &manager, nil
}

func (mgr *TunnelPodManager) Config() Config {
	return *mgr.config
}

func (mgr *TunnelPodManager) init() error {
	// check existence of configmap
	_, err := mgr.client.CoreV1().ConfigMaps(mgr.config.Namespace).Get(mgr.config.CertificateName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}

	rs := mgr.getReplicaSetDefinition()
	rs, err = mgr.client.ExtensionsV1beta1().ReplicaSets(mgr.config.Namespace).Create(rs)
	if err != nil {
		return err
	}
	mgr.replicaSetName = rs.ObjectMeta.Name
	mgr.active = false
	return nil
}

func (mgr *TunnelPodManager) getReplicaSetDefinition() *ext_v1beta1.ReplicaSet {
	var initialReplicas int32 = 0

	return &ext_v1beta1.ReplicaSet{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: fmt.Sprintf("cfwarp-%s", mgr.id),
		},
		Spec: ext_v1beta1.ReplicaSetSpec{
			Replicas: &initialReplicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:   fmt.Sprintf("cfwarp-%s", mgr.id),
					Labels: mgr.podLabels(),
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "cloudflare-warp-cert",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: mgr.config.CertificateName,
									},
								},
							},
						},
					},
					Containers: []v1.Container{
						{
							Name:  "cloudflare-warp",
							Image: "quay.io/stackpoint/cloudflare-warp:3ef7efb",
							Command: []string{
								"/cloudflare-warp",
								"--url",
								mgr.config.ServiceName,
								"--hostname",
								mgr.config.ExternalHostname,
								"--origincert",
								"/etc/cloudflare-warp/cert.pem",
								"--loglevel",
								"debug",
							},
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 8080,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "cloudflare-warp-cert",
									MountPath: "/etc/cloudflare-warp",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (mgr *TunnelPodManager) Start() error {

	replicasets := mgr.client.ExtensionsV1beta1().ReplicaSets(mgr.config.Namespace)
	rs, err := replicasets.Get(mgr.replicaSetName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}
	var activeSize int32 = 1
	rs.Spec.Replicas = &activeSize
	_, err = replicasets.Update(rs)
	if err != nil {
		return err
	}
	mgr.active = true
	return nil
}

func (mgr *TunnelPodManager) Active() bool {
	return mgr.active
}

func (mgr *TunnelPodManager) Stop() error {
	if !mgr.active {
		return nil
	}

	glog.V(4).Infof("Stopping tunnel %s", mgr.id)

	replicasets := mgr.client.ExtensionsV1beta1().ReplicaSets(mgr.config.Namespace)
	rs, err := replicasets.Get(mgr.replicaSetName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}
	var inactiveSize int32 = 0
	rs.Spec.Replicas = &inactiveSize
	_, err = replicasets.Update(rs)
	if err != nil {
		return err
	}
	mgr.active = false
	return nil

}

func (mgr *TunnelPodManager) TearDown() error {
	err := mgr.Stop()
	if err != nil {
		return err
	}
	return mgr.client.ExtensionsV1beta1().ReplicaSets(mgr.config.Namespace).Delete(mgr.replicaSetName, &meta_v1.DeleteOptions{})
}

// not sure this is useful if we are using a replicaset
func (mgr *TunnelPodManager) CheckStatus() error {
	// labelSet := mgr.podLabels()
	// lo := meta_v1.ListOptions{LabelSelector: labelSet.String()}
	// should use a cache ...
	pod, err := mgr.client.CoreV1().Pods(mgr.config.Namespace).Get(mgr.replicaSetName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}
	if pod == nil {
		// glog.V(2).Infof("No pods found for selector %s, reinitializing", labelSet.String())
		glog.V(2).Infof("No pods found for name %s, reinitializing", mgr.replicaSetName)
		return mgr.Start()
	}
	ready := false
	for _, c := range pod.Status.Conditions {
		if c.Type == "Ready" && c.Status == "True" {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("Pod %s exists but is not ready", pod.ObjectMeta.Name)
	}
	return nil
}

func (mgr *TunnelPodManager) podLabels() fields.Set {
	// XXX inherit other labels
	return map[string]string{
		"warp.cloudflare/id":       mgr.id,
		"warp.cloudflare/url":      mgr.config.ServiceName,
		"warp.cloudflare/hostname": mgr.config.ExternalHostname,
	}

}

// not good to read the log into a string
func (mgr *TunnelPodManager) logs() string {
	req := mgr.client.CoreV1().Pods(mgr.config.Namespace).GetLogs("cloudflare-warp", &v1.PodLogOptions{})

	readCloser, err := req.Stream()
	if err != nil {
		return ""
	}
	defer readCloser.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(readCloser)
	return buf.String()

}
