package openstack

import (
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/openstack-test/test/extended/openstack/machines"
	"github.com/stretchr/objx"

	exutil "github.com/openshift/origin/test/extended/util"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	//monitoringv1alpha1 "sigs.k8s.io/prometheus-operator/pkg/apis/monitoring/v1alpha1"
)

var _ = g.Describe("[sig-installer][Suite:openshift/openstack] Machine", func() {
	defer g.GinkgoRecover()

	var dcRhoso dynamic.Interface
	var dcShiftstack dynamic.Interface
	var machineResourcesRhoso []objx.Map
	var machineResourcesShiftstack []objx.Map
	//var clientSet *kubernetes.Clientset
	oc := exutil.NewCLI("openstack")

	/*	g.BeforeEach(func(ctx g.SpecContext) {
			oc := exutil.NewCLI("openstack")

		})
	*/
	g.It("are in phase Running foo", func(ctx g.SpecContext) {
		/*	g.By("fetching Machines rhoso")

			rhosoKubeConfig := os.Getenv("RHOSO_KUBECONFIG")
			if rhosoKubeConfig == "" {
				e2eskipper.Skipf("RHOSO_KUBECONFIG must be set for this test to run")
			}
			e2e.TestContext.KubeConfig = rhosoKubeConfig
			SetTestContextHostFromKubeconfig(rhosoKubeConfig)
			rhosoCfg, err := e2e.LoadConfig()
			o.Expect(err).NotTo(o.HaveOccurred())
			dcRhoso, err = dynamic.NewForConfig(rhosoCfg)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("fetching Machines rhoso")
			machineResourcesRhoso, err = machines.List(ctx, dcRhoso)
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Machines rhoso: %v", machineResourcesRhoso[0].Get("metadata.name"))
			for _, machine := range machineResourcesRhoso {
				o.Expect(machine.Get("status.phase").String()).To(o.Equal("Running"), "unexpected phase for Machine %q", machine.Get("metadata.name"))
			}*/
		g.By("fetching Machines shiftstack")
		shiftstacKubeConfig := os.Getenv("KUBECONFIG")
		e2e.TestContext.KubeConfig = shiftstacKubeConfig
		SetTestContextHostFromKubeconfig(shiftstacKubeConfig)
		shiftstackCfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dcShiftstack, err = dynamic.NewForConfig(shiftstackCfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		machineResourcesShiftstack, err = machines.List(ctx, dcShiftstack)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Machines shiftstack: %v", machineResourcesShiftstack[0].Get("metadata.name"))
		if oc == nil {

		}
		for _, machine := range machineResourcesShiftstack {
			o.Expect(machine.Get("status.phase").String()).To(o.Equal("Running"), "unexpected phase for Machine %q", machine.Get("metadata.name"))
		}
		//		route, err := oc.AdminRouteClient().RouteV1().Routes("prometheus-k8s-federate").Get(ctx, "canary", metav1.GetOptions{})

		route, err := oc.AdminRouteClient().RouteV1().Routes("openshift-monitoring").Get(ctx, "prometheus-k8s-federate", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		//Get route for federate in shiftstack openshift-monitoring
		routeHost := route.Status.Ingress[0].Host
		e2e.Logf("Route Host: %v", route.Status.Ingress[0].Host)
		out, err := oc.Run("whoami").Args("-t").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		//Create token on shiftstack
		token := strings.TrimSpace(out)
		e2e.Logf("Token: %v", token)

		g.By("fetching Machines rhoso")

		rhosoKubeConfig := os.Getenv("RHOSO_KUBECONFIG")
		if rhosoKubeConfig == "" {
			e2eskipper.Skipf("RHOSO_KUBECONFIG must be set for this test to run")
		}
		e2e.TestContext.KubeConfig = rhosoKubeConfig
		SetTestContextHostFromKubeconfig(rhosoKubeConfig)
		rhosoCfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dcRhoso, err = dynamic.NewForConfig(rhosoCfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("fetching Machines rhoso")
		machineResourcesRhoso, err = machines.List(ctx, dcRhoso)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Machines rhoso: %v", machineResourcesRhoso[0].Get("metadata.name"))
		for _, machine := range machineResourcesRhoso {
			o.Expect(machine.Get("status.phase").String()).To(o.Equal("Running"), "unexpected phase for Machine %q", machine.Get("metadata.name"))
		}
		g.By("Creating a secret with token on rhoso openstack namespace")
		clientSet, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		secretData := map[string][]byte{
			"token": []byte(token),
		}
		secretDef := &v1core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ocp-federated",
			},
			Data: secretData,
		}
		client := clientSet.CoreV1().Secrets("openstack")
		secret, err := client.Create(ctx, secretDef, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer client.Delete(ctx, "ocp-federated", metav1.DeleteOptions{})
		e2e.Logf("Secret: %v", secret)
		time.Sleep(time.Second * 10)

		/*
			scrapeConfig := &monitoringv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sos1-federated",
					Namespace: "openstack",
					Labels:    map[string]string{"service": "metricStorage"},
				},
				Spec: monitoringv1alpha1.ScrapeConfigSpec{
					Scheme:         "https",
					MetricsPath:    "federate",
					ScrapeInterval: "30s",
					Params: map[string][]string{
						"match[]": {`{__name__=~"kube_node_info|kube_persistentvolume_info"}`},
					},
					Authorization: &monitoringv1alpha1.SafeAuthorization{
						Type: "Bearer",
						Credentials: &monitoringv1alpha1.SecretKeySelector{
							Name: "ocp-federated",
							Key:  "token",
						},
					},
					TLSConfig: &monitoringv1alpha1.SafeTLSConfig{
						InsecureSkipVerify: true,
					},
					StaticConfigs: []monitoringv1alpha1.StaticConfig{
						{
							Targets: []string{
								routeHost,
							},
						},
					},
				},
			}
		*/
		//pClient, err := versioned.NewForConfig(rhosoCfg)
		pClient, err := dynamic.NewForConfig(rhosoCfg)

		//scrapeConfig, err = pClient.MonitoringV1alpha1().ScrapeConfigs("openstack").Create(ctx, scrapeConfig, metav1.CreateOptions{})
		gvr := schema.GroupVersionResource{
			Group:    "monitoring.rhobs",
			Version:  "v1alpha1",
			Resource: "scrapeconfigs",
		}

		// Define the custom resource (ScrapeConfig) as a map
		scrapeConfig := map[string]interface{}{
			"apiVersion": "monitoring.rhobs/v1alpha1",
			"kind":       "ScrapeConfig",
			"metadata": map[string]interface{}{
				"name":      "sos-federated",
				"namespace": "openstack",
				"labels": map[string]interface{}{
					"service": "metricStorage",
				},
			},
			"spec": map[string]interface{}{
				"scheme":         "HTTPS",
				"metricsPath":    "federate",
				"scrapeInterval": "30s",
				"params": map[string]interface{}{
					"match[]": []string{
						`{__name__=~"kube_node_info|kube_persistentvolume_info"}`,
					},
				},
				"authorization": map[string]interface{}{
					"type": "Bearer",
					"credentials": map[string]interface{}{
						"name": "ocp-federated",
						"key":  "token",
					},
				},
				"tlsConfig": map[string]interface{}{
					"insecureSkipVerify": true,
				},
				"staticConfigs": []interface{}{
					map[string]interface{}{
						"targets": []string{
							routeHost,
						},
					},
				},
			},
		}
		g.By("Creating ScrapeConfig....")
		resourceClient := pClient.Resource(gvr).Namespace("openstack")
		scrapeCfg, err := resourceClient.Create(ctx, &unstructured.Unstructured{
			Object: scrapeConfig,
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("scrapeCfg: %v", scrapeCfg)
		defer resourceClient.Delete(ctx, "sos-federate", metav1.DeleteOptions{})
		time.Sleep(time.Second * 10)

	})
})

// Set the TestContextHost based on the Kubeconfig
func SetTestContextHostFromKubeconfig(kubeConfigPath string) error {
	e2e.TestContext.KubeConfig = kubeConfigPath

	kubeConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContext := kubeConfig.CurrentContext

	// Retrieve the cluster information for the active context
	context, ok := kubeConfig.Contexts[currentContext]
	if !ok {
		return fmt.Errorf("context %q not found in kubeconfig", currentContext)
	}

	cluster, ok := kubeConfig.Clusters[context.Cluster]
	if !ok {
		return fmt.Errorf("cluster %q not found in kubeconfig", context.Cluster)
	}

	e2e.TestContext.Host = cluster.Server
	return nil
}
