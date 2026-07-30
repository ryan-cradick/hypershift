package configoperator

import (
	"testing"

	. "github.com/onsi/gomega"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	component "github.com/openshift/hypershift/support/controlplane-component"
	"github.com/openshift/hypershift/support/metrics"

	prometheusoperatorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAdaptPodMonitor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		platform       hyperv1.PlatformType
		existingAnnots map[string]string
		verify         func(g Gomega, podMonitor *prometheusoperatorv1.PodMonitor)
	}{
		{
			name:     "When platform is not IBMCloud, it should not set the metrics-service annotation",
			platform: hyperv1.AWSPlatform,
			verify: func(g Gomega, podMonitor *prometheusoperatorv1.PodMonitor) {
				g.Expect(podMonitor.Annotations).NotTo(HaveKey("hypershift.openshift.io/metrics-service"))
			},
		},
		{
			name:     "When platform is IBMCloud and annotations are nil, it should set the metrics-service annotation to roks-metrics",
			platform: hyperv1.IBMCloudPlatform,
			verify: func(g Gomega, podMonitor *prometheusoperatorv1.PodMonitor) {
				g.Expect(podMonitor.Annotations).To(HaveKeyWithValue("hypershift.openshift.io/metrics-service", "roks-metrics"))
			},
		},
		{
			name:     "When platform is IBMCloud and annotations already exist, it should add metrics-service annotation preserving existing ones",
			platform: hyperv1.IBMCloudPlatform,
			existingAnnots: map[string]string{
				"existing-key": "existing-value",
			},
			verify: func(g Gomega, podMonitor *prometheusoperatorv1.PodMonitor) {
				g.Expect(podMonitor.Annotations).To(HaveKeyWithValue("hypershift.openshift.io/metrics-service", "roks-metrics"))
				g.Expect(podMonitor.Annotations).To(HaveKeyWithValue("existing-key", "existing-value"))
			},
		},
		{
			name:     "When platform is NonePlatform, it should not set the metrics-service annotation",
			platform: hyperv1.NonePlatform,
			verify: func(g Gomega, podMonitor *prometheusoperatorv1.PodMonitor) {
				g.Expect(podMonitor.Annotations).NotTo(HaveKey("hypershift.openshift.io/metrics-service"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			hcp := &hyperv1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: hyperv1.HostedControlPlaneSpec{
					Platform: hyperv1.PlatformSpec{
						Type: tt.platform,
					},
					ClusterID: "test-cluster-id",
				},
			}

			podMonitor := &prometheusoperatorv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.existingAnnots,
				},
				Spec: prometheusoperatorv1.PodMonitorSpec{
					PodMetricsEndpoints: []prometheusoperatorv1.PodMetricsEndpoint{
						{},
					},
				},
			}

			cpContext := component.WorkloadContext{
				HCP:        hcp,
				MetricsSet: metrics.MetricsSetAll,
			}

			err := adaptPodMonitor(cpContext, podMonitor)
			g.Expect(err).ToNot(HaveOccurred())

			tt.verify(g, podMonitor)
		})
	}
}
