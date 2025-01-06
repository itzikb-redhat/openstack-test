package openstack

import (
	"fmt"
	"os"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/openstack-test/test/extended/openstack/machines"
	"github.com/stretchr/objx"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

var _ = g.Describe("[sig-installer][Suite:openshift/openstack] Machine", func() {
	defer g.GinkgoRecover()

	var dcRhoso dynamic.Interface
	var dcShiftstack dynamic.Interface
	var machineResourcesRhoso []objx.Map
	var machineResourcesShiftstack []objx.Map

	g.BeforeEach(func(ctx g.SpecContext) {
	})

	g.It("are in phase Running foo", func(ctx g.SpecContext) {
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
		for _, machine := range machineResourcesShiftstack {
			o.Expect(machine.Get("status.phase").String()).To(o.Equal("Running"), "unexpected phase for Machine %q", machine.Get("metadata.name"))
		}

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
