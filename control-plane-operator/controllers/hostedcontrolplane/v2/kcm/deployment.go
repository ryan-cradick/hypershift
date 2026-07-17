package kcm

import (
	"fmt"
	"strings"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/control-plane-operator/controllers/hostedcontrolplane/manifests"
	"github.com/openshift/hypershift/support/config"
	component "github.com/openshift/hypershift/support/controlplane-component"
	"github.com/openshift/hypershift/support/netutil"
	"github.com/openshift/hypershift/support/podspec"
	"github.com/openshift/hypershift/support/proxy"
	"github.com/openshift/hypershift/support/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func adaptDeployment(cpContext component.WorkloadContext, deployment *appsv1.Deployment) error {
	hcp := cpContext.HCP

	featureGates, err := config.FeatureGatesFromConfigMap(cpContext.Context, cpContext.Client, cpContext.HCP.Namespace)
	if err != nil {
		return err
	}

	podspec.UpdateContainer(ComponentName, deployment.Spec.Template.Spec.Containers, func(c *corev1.Container) {
		c.Args = append(c.Args,
			fmt.Sprintf("--cluster-cidr=%s", netutil.FirstClusterCIDR(hcp.Spec.Networking.ClusterNetwork)),
			fmt.Sprintf("--service-cluster-ip-range=%s", netutil.FirstServiceCIDR(hcp.Spec.Networking.ServiceNetwork)),
		)
		// This value comes from the Cloud Provider Azure documentation: https://cloud-provider-azure.sigs.k8s.io/install/azure-ccm/#kube-controller-manager
		if hcp.Spec.Platform.Type == hyperv1.AzurePlatform {
			c.Args = append(c.Args, fmt.Sprintf("--cloud-provider=%s", "external"))
		}

		// Enable node CIDR allocation if `.Spec.Networking.AllocateNodeCIDRs` is "Enabled"
		if hcp.Spec.Networking.AllocateNodeCIDRs != nil && *hcp.Spec.Networking.AllocateNodeCIDRs == hyperv1.AllocateNodeCIDRsEnabled {
			c.Args = append(c.Args, "--allocate-node-cidrs=true")
		} else {
			c.Args = append(c.Args, "--allocate-node-cidrs=false")
		}

		if hcp.Spec.Platform.Type == hyperv1.IBMCloudPlatform {
			c.Args = append(c.Args, "--node-monitor-grace-period=55s")
		} else {
			c.Args = append(c.Args, "--node-monitor-grace-period=50s")
		}

		if tlsMinVersion := config.MinTLSVersion(hcp.Spec.Configuration.GetTLSSecurityProfile()); tlsMinVersion != "" {
			c.Args = append(c.Args, fmt.Sprintf("--tls-min-version=%s", tlsMinVersion))
		}
		if cipherSuites := config.CipherSuites(hcp.Spec.Configuration.GetTLSSecurityProfile()); len(cipherSuites) != 0 {
			c.Args = append(c.Args, fmt.Sprintf("--tls-cipher-suites=%s", strings.Join(cipherSuites, ",")))
		}
		if util.StringListContains(hcp.Annotations[hyperv1.DisableProfilingAnnotation], ComponentName) {
			c.Args = append(c.Args, "--profiling=false")
		}

		for _, f := range featureGates {
			c.Args = append(c.Args, fmt.Sprintf("--feature-gates=%s", f))
		}

		proxy.SetEnvVars(&c.Env)

		// Always add the service-serving-ca volume and mount with optional=true so that the
		// deployment spec is stable regardless of whether the configmap exists yet. This avoids
		// a pod restart when the Hosted Cluster Config Operator populates the configmap after
		// the initial control-plane reconcile.
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "service-serving-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: manifests.ServiceServingCA(hcp.Namespace).Name,
					},
					Optional: ptr.To(true),
				},
			},
		})

		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      "service-serving-ca",
			MountPath: "/etc/kubernetes/certs/service-ca",
		})
	})

	return nil
}
