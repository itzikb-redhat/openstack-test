package openstack

import (
	"fmt"
	"context"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/servergroups"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	exutil "github.com/openshift/origin/test/extended/util"	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-installer][Feature:openstack] The OpenStack platform", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("openstack")

	// OCP 4.5: https://issues.redhat.com/browse/OSASINFRA-1300
	g.It("creates Control plane nodes in a server group", func() {
		ctx := context.TODO()

		g.By("preparing a dynamic client")
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("getting the IDs of the Control plane instances and their Server group")
		masterInstanceUUIDs := make([]interface{}, 0, 3)
		var serverGroupName string
		{

			clientSet, err := e2e.LoadClientset()
			o.Expect(err).NotTo(o.HaveOccurred())

			nodeList, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			ms := dc.Resource(schema.GroupVersionResource{
				Group:    "machine.openshift.io",
				Resource: "machines",
				Version:  "v1beta1",
			})
			for _, item := range nodeList.Items {
				uuid := strings.TrimPrefix(item.Spec.ProviderID, "openstack:///")
				masterInstanceUUIDs = append(masterInstanceUUIDs, uuid)

				machineAnnotation := strings.SplitN(item.Annotations["machine.openshift.io/machine"], "/", 2)
				o.Expect(machineAnnotation).To(o.HaveLen(2))

				res, err := ms.Namespace(machineAnnotation[0]).Get(ctx, machineAnnotation[1], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				instanceServerGroupNameField := getFromUnstructured(res, "spec", "providerSpec", "value", "serverGroupName")
				o.Expect(instanceServerGroupNameField).NotTo(o.BeNil(), "the Server group name should be present in the Machine definition")

				instanceServerGroupName := instanceServerGroupNameField.(string)
				o.Expect(instanceServerGroupName).NotTo(o.BeEmpty(), "the Server group name should not be the empty string")
				if serverGroupName == "" {
					serverGroupName = instanceServerGroupName
				} else {
					o.Expect(instanceServerGroupName).To(o.Equal(serverGroupName), "two Control plane Machines have different serverGroupName set")
				}
			}
		}

		g.By("checking the actual members of the Server group")
		{
			config, _ := installConfigFromCluster(oc.AdminKubeClient().CoreV1())
			config_map := config.(map[interface {}]interface {})
			e2e.Logf("My Config: %v",config_map["controlPlane"]["platform"])

			computeClient, err := client(serviceCompute)
			o.Expect(err).NotTo(o.HaveOccurred())

			serverGroupsWithThatName, err := serverGroupIDsFromName(computeClient, serverGroupName)
			o.Expect(serverGroupsWithThatName, err).To(o.HaveLen(1), "the server group name either was not found or is not unique")

			serverGroup, err := servergroups.Get(computeClient, serverGroupsWithThatName[0]).Extract()
			o.Expect(serverGroup.Members, err).To(o.ContainElements(masterInstanceUUIDs...))

			if len(serverGroup.Policies) == 1 {
				//host_ids := make([]string, 0)
				host_ids := make(map[string]int)

				for _, server_id := range masterInstanceUUIDs {
					server, err := servers.Get(computeClient, server_id.(string)).Extract()
					o.Expect(err).NotTo(o.HaveOccurred())
					host_ids[server.HostID] +=1
				}
				if serverGroup.Policies[0] == "anti-affinity"{
//				    o.Expect(host_ids).To(o.HaveLen(2)))
					o.Expect(host_ids).To(o.HaveLen(len(masterInstanceUUIDs)))

				}
			}

		}

	})
})

func getFromUnstructured(unstr *unstructured.Unstructured, keys ...string) interface{} {
	m := unstr.UnstructuredContent()
	for _, key := range keys[:len(keys)-1] {
		m = m[key].(map[string]interface{})
	}
	return m[keys[len(keys)-1]]
}

// IDsFromName returns zero or more IDs corresponding to a name. The returned
// error is only non-nil in case of failure.
func serverGroupIDsFromName(client *gophercloud.ServiceClient, name string) ([]string, error) {
	pages, err := servergroups.List(client).AllPages()
	if err != nil {
		return nil, err
	}

	all, err := servergroups.ExtractServerGroups(pages)
	if err != nil {
		return nil, err
	}

	IDs := make([]string, 0, len(all))
	for _, s := range all {
		if s.Name == name {
			IDs = append(IDs, s.ID)
		}
	}

	return IDs, nil
}

//type installConfig struct {
//	BASEDOMAIN string	 `json:"baseDomain,omitempty"`
//}

const (
	installConfigName = "cluster-config-v1"
)

func installConfigFromCluster(client clientcorev1.ConfigMapsGetter) (interface{},error) {
	cm, err := client.ConfigMaps("kube-system").Get(context.Background(), installConfigName, metav1.GetOptions{})
	if err != nil {
			return nil, err
	}
	data, ok := cm.Data["install-config"]
	if !ok {
			return nil, fmt.Errorf("no install-config found in kube-system/%s", installConfigName)
	}
	//config := &installConfig{}
    //config := &installConfig{}
	var config interface{}
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
			return nil, err
	}
	return config, nil
}



