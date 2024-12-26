package openstack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	machinev1alpha1 "github.com/openshift/api/machine/v1alpha1"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	framework "github.com/openshift/cluster-api-actuator-pkg/pkg/framework"
	"github.com/openshift/openstack-test/test/extended/openstack/client"
	"github.com/openshift/openstack-test/test/extended/openstack/machines"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = g.Describe("[sig-installer][Suite:openshift/openstack] Bugfix", func() {
	defer g.GinkgoRecover()

	var dc dynamic.Interface
	var clientSet *kubernetes.Clientset
	var networkClient *gophercloud.ServiceClient

	g.BeforeEach(func(ctx g.SpecContext) {
		g.By("preparing openshift dynamic client")
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err = dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		clientSet, err = e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		skipUnlessMachineAPIOperator(ctx, dc, clientSet.CoreV1().Namespaces())

		g.By("preparing the openstack client")
		networkClient, err = client.GetServiceClient(ctx, openstack.NewNetworkV2)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to build the OpenStack client")
	})

	g.Context("bug_1765:", func() {
		g.It("[Serial] noAllowedAddressPairs on one port should not affect other ports", func(ctx g.SpecContext) {
			// Check the scenario at https://issues.redhat.com/browse/OCPBUGS-1765

			g.By("Fetching worker machineSets")
			var rawBytes []byte
			var newProviderSpec machinev1alpha1.OpenstackProviderSpec
			var rclient runtimeclient.Client

			machineSets, err := getMachineSets(ctx, dc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Error getting the workers machinesets")

			if len(machineSets) == 0 {
				e2eskipper.Skipf("Expects at least one worker machineset. Found none.")
			}

			cfg, err := e2e.LoadConfig()
			o.Expect(err).NotTo(o.HaveOccurred(), "Error getting cluster config")

			rclient, err = runtimeclient.New(cfg, runtimeclient.Options{})
			o.Expect(err).NotTo(o.HaveOccurred(), "Error creating a runtime client")

			// Create an additional network
			extraNetworkName := fmt.Sprintf("%v-%v", "foonet", RandomSuffix())
			networkCreateOpts := networks.CreateOpts{Name: extraNetworkName}
			extraNetwork, err := networks.Create(ctx, networkClient, networkCreateOpts).Extract()
			o.Expect(err).NotTo(o.HaveOccurred(), "Error creating network")
			//defer networks.Delete(ctx, networkClient, extraNetwork.ID)
			defer deleteNetwork(extraNetwork.ID, networkClient)

			err = machinev1.AddToScheme(scheme.Scheme)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to add Machine to scheme")
			err = configv1.AddToScheme(scheme.Scheme)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to add Config to scheme")

			newMachinesetParams := framework.BuildMachineSetParams(ctx, rclient, 1)
			rawBytes, err = json.Marshal(newMachinesetParams.ProviderSpec.Value)
			o.Expect(err).NotTo(o.HaveOccurred(), "Error marshaling new MachineSet Provider Spec")

			err = json.Unmarshal(rawBytes, &newProviderSpec)
			o.Expect(err).NotTo(o.HaveOccurred(), "Error unmarshaling new MachineSet Provider Spec")

			var extraNetworkParam machinev1alpha1.NetworkParam
			extraNetworkParam.NoAllowedAddressPairs = true
			extraNetworkParam.Filter.ID = extraNetwork.ID

			newProviderSpec.Networks = append(newProviderSpec.Networks, extraNetworkParam)

			newMachinesetParams.Name = fmt.Sprintf("%v-%v", "bogus", RandomSuffix())
			newProviderSpecJson, err := json.Marshal(newProviderSpec)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to marshal new Machineset provider spec")
			newMachinesetParams.ProviderSpec.Value.Raw = newProviderSpecJson

			g.By("Create a new machineSet with bogus server group ID")
			ms, err := framework.CreateMachineSet(rclient, newMachinesetParams)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create a Machineset")
			defer DeleteMachinesetsDefer(rclient, ms)

			err = GetMachinesetRetry(ctx, rclient, ms, true)

			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get the new Machineset")
			err = waitUntilNMachinesPrefix(ctx, dc, ms.Name, 1)
			g.By("Sleeping...")
			time.Sleep(10 * time.Second)
			newMachines, err := machines.List(ctx, dc, machines.ByMachineSet(ms.Labels["machine.openshift.io/cluster-api-machineset"]))
			o.Expect(err).NotTo(o.HaveOccurred(), "error fetching machines for MachineSet %q", ms.Name)

			var machineNetworks []string
			for _, machine := range newMachines {
				for _, net := range objects(machine.Get("spec.providerSpec.value.networks")) {
					machineNetworks = append(machineNetworks, net.Get("filter.id").String())
				}
			}

			for _, net := range machineNetworks {
				portListOpts := ports.ListOpts{
					NetworkID: net,
				}
				g.By(fmt.Sprintf("Current network: %v", net))
				allPages, err := ports.List(networkClient, portListOpts).AllPages(ctx)
				o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get ports")
				allPorts, err := ports.ExtractPorts(allPages)
				o.Expect(err).NotTo(o.HaveOccurred(), "Failed to extract ports")
				for _, port := range allPorts {
					if strings.Contains(port.Name, newMachinesetParams.Name) {
						if net == extraNetwork.ID {
							o.Expect(len(port.AllowedAddressPairs)).To(o.Equal(0))
							g.By("Found, Should noallowed empty")
						} else {
							g.By("Found, Should noallowed full")
							o.Expect(len(port.AllowedAddressPairs)).To(o.Not(o.Equal(0)))
						}
					}
				}
			}
			o.Expect(err).To(o.HaveOccurred(), "foo")
			g.By("Sleeping...")
			time.Sleep(15 * time.Second)
			g.By("Deleting the new machineset")
			framework.DeleteMachineSets(rclient, ms)
			err = GetMachinesetRetry(ctx, rclient, ms, false)
			o.Expect(errors.IsNotFound(err)).To(o.BeTrue(), "Machineset %v was not deleted", ms.Name)

			err = waitUntilNMachinesPrefix(ctx, dc, ms.Name, 0)
			g.By(fmt.Sprintf("Deleting network %v", extraNetworkName))
			r2 := networks.Delete(ctx, networkClient, extraNetwork.ID)
			g.By(fmt.Sprintf("network delete: %v", r2.Result))
		})
	})
})

func deleteNetwork(netID string, networkClient *gophercloud.ServiceClient) {

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var allPorts []ports.Port

	for {
		select {
		case <-ctx.Done():
			return
		default:
			portListOpts := ports.ListOpts{
				NetworkID: netID,
			}
			allPages, err := ports.List(networkClient, portListOpts).AllPages(ctx)
			if err == nil {
				allPorts, err = ports.ExtractPorts(allPages)
			}
			if err == nil {
				if len(allPorts) == 0 {
					networks.Delete(ctx, networkClient, netID)
					return
				} else {
					for _, port := range allPorts {
						ports.Delete(ctx, networkClient, port.ID)
					}
				}
			}
		}
	}
}
