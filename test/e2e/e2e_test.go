/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	route "github.com/openshift/api/route/v1"
	networkingv1alpha1 "github.com/stakater/ipshield-operator/api/v1alpha1"
	"github.com/stakater/ipshield-operator/internal/controller"

	"github.com/stakater/ipshield-operator/test/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const OperatorNamespace = "ipshield-operator-system"

// TODO add e2e tests
// Take look at https://github.com/cloudnative-pg/cloudnative-pg/tree/main/tests/e2e

const (
	IPShieldCRNamespace = "ipshield-cr"
	TestingNamespace    = "mywebserver-2"
	NginxDeployment     = "https://k8s.io/examples/application/deployment.yaml"
	RouteName           = "nginx-deployment"
)

var client kubeclient.Client
var clientset *kubernetes.Clientset

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(route.AddToScheme(scheme))
	utilruntime.Must(networkingv1alpha1.AddToScheme(scheme))

	config := ctrl.GetConfigOrDie()
	k8sclient, err := kubeclient.New(config, kubeclient.Options{Scheme: scheme})

	Expect(err).NotTo(HaveOccurred())
	Expect(k8sclient).NotTo(BeNil())
	client = k8sclient

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())

})

// Run e2e tests using the Ginkgo runner.
func TestRouteWhiteLists(t *testing.T) {
	RegisterFailHandler(Fail)
	fmt.Fprintf(GinkgoWriter, "Starting ipshield-operator suite\n")
	RunSpecs(t, "TestRouteWhiteLists")
}

func getRouteWhiteListSpec(name, ns string, ips []string) *networkingv1alpha1.RouteWhitelist {
	return &networkingv1alpha1.RouteWhitelist{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: networkingv1alpha1.RouteWhitelistSpec{
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"ipshield": strconv.FormatBool(true),
				},
			},
			IPRanges: ips,
		},
	}
}

var _ = Describe("controller", Ordered, func() {

	BeforeAll(func() {
		clientset, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
		Expect(err).To(BeNil())

		Expect(utils.CreateNamespace(context.TODO(), clientset, IPShieldCRNamespace)).Error().To(BeNil())
		Expect(utils.CreateNamespace(context.TODO(), clientset, OperatorNamespace)).Error().To(BeNil())
		Expect(utils.CreateNamespace(context.TODO(), clientset, TestingNamespace)).Error().To(BeNil())

		Expect(utils.Run(exec.Command("kubectl", "apply", "-f", NginxDeployment, "-n", TestingNamespace))).Error().ShouldNot(HaveOccurred())
		Expect(utils.CreateClusterIPService(context.TODO(), client, TestingNamespace, RouteName)).Error().ShouldNot(HaveOccurred())

	})

	AfterAll(func() {
		// By("uninstalling the Prometheus manager bundle")
		// utils.UninstallPrometheusOperator()

		// By("uninstalling the cert-manager bundle")
		// utils.UninstallCertManager()

		// By("removing manager namespace")
		// cmd := exec.Command("kubectl", "delete", "ns", namespace)
		// _, _ = utils.Run(cmd)
	})

	Context("Operator", func() {
		var whitelist *networkingv1alpha1.RouteWhitelist

		// It("should run successfully", func() {
		// 	var controllerPodName string
		// 	var err error

		// 	// projectimage stores the name of the image used in the example
		// 	var projectimage = "example.com/ipshield-operator:v0.0.1"

		// 	By("building the manager(Operator) image")
		// 	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
		// 	_, err = utils.Run(cmd)
		// 	ExpectWithOffset(1, err).NotTo(HaveOccurred())

		// 	By("loading the the manager(Operator) image on Kind")
		// 	err = utils.LoadImageToKindClusterWithName(projectimage)
		// 	ExpectWithOffset(1, err).NotTo(HaveOccurred())

		// 	By("installing CRDs")
		// 	cmd = exec.Command("make", "install")
		// 	_, err = utils.Run(cmd)
		// 	ExpectWithOffset(1, err).NotTo(HaveOccurred())

		// 	By("deploying the controller-manager")
		// 	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
		// 	_, err = utils.Run(cmd)
		// 	ExpectWithOffset(1, err).NotTo(HaveOccurred())

		// 	By("validating that the controller-manager pod is running as expected")
		// 	verifyControllerUp := func() error {
		// 		// Get pod name

		// 		cmd = exec.Command("kubectl", "get",
		// 			"pods", "-l", "control-plane=controller-manager",
		// 			"-o", "go-template={{ range .items }}"+
		// 				"{{ if not .metadata.deletionTimestamp }}"+
		// 				"{{ .metadata.name }}"+
		// 				"{{ \"\\n\" }}{{ end }}{{ end }}",
		// 			"-n", namespace,
		// 		)

		// 		podOutput, err := utils.Run(cmd)
		// 		ExpectWithOffset(2, err).NotTo(HaveOccurred())
		// 		podNames := utils.GetNonEmptyLines(string(podOutput))
		// 		if len(podNames) != 1 {
		// 			return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
		// 		}
		// 		controllerPodName = podNames[0]
		// 		ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

		// 		// Validate pod status
		// 	// 	cmd = exec.Command("kubectl", "get",
		// 	// 		"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
		// 	// 		"-n", namespace,
		// 	// 	)
		// 	// 	status, err := utils.Run(cmd)
		// 	// 	ExpectWithOffset(2, err).NotTo(HaveOccurred())
		// 	// 	if string(status) != "Running" {
		// 	// 		return fmt.Errorf("controller pod in %s status", status)
		// 	// 	}
		// 	// 	return nil
		// 	// }
		// 	// EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

		// })

		BeforeEach(func() {

			utils.Run(exec.Command("oc", "expose", "svc", RouteName, "-n", TestingNamespace))
			utils.Run(exec.Command("kubectl", "label", "route", RouteName, controller.IPShieldWatchedResourceLabel+"-", "-n", TestingNamespace))
			utils.Run(exec.Command("kubectl", "label", "route", RouteName, "ipshield=true", "-n", TestingNamespace))

			time.Sleep(1 * time.Second)
		})

		AfterEach(func() {

			client.Delete(context.TODO(), whitelist)

			utils.Run(exec.Command("kubectl", "delete", "route", RouteName, "-n", TestingNamespace))

			time.Sleep(1 * time.Second)
		})

		It("Deploy CR but route doesn't have label", func() {
			whitelist = getRouteWhiteListSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13"})
			err := client.Create(context.TODO(), whitelist)
			Expect(err).NotTo(HaveOccurred())

			r := &route.Route{}
			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())
			Expect(r.Labels).ShouldNot(HaveKey(controller.IPShieldWatchedResourceLabel))
			Expect(r.Annotations).ShouldNot(HaveKeyWithValue(controller.WhiteListAnnotation, "10.200.15.13"))
		})

		It("Deploy CR and route has label", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())
			Expect(r.Annotations).ShouldNot(HaveKey(controller.IPShieldWatchedResourceLabel))

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			whitelist = getRouteWhiteListSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13"})
			err = client.Create(context.TODO(), whitelist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(3 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Labels).Should(HaveKeyWithValue(controller.IPShieldWatchedResourceLabel, strconv.FormatBool(true)))
			Expect(r.Annotations).Should(HaveKeyWithValue(controller.WhiteListAnnotation, "10.200.15.13"))
		})

		It("Deploy CR and route already had whitelist 1 element", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			r.Annotations[controller.WhiteListAnnotation] = "192.168.10.32"

			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(3 * time.Second)

			whitelist = getRouteWhiteListSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), whitelist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(30 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.WhiteListAnnotation))
			Expect(strings.Split(r.Annotations[controller.WhiteListAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132", "192.168.10.32"))

		})

		It("Route and CR had one common IP", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			r.Annotations[controller.WhiteListAnnotation] = "10.200.15.13"

			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(3 * time.Second)

			whitelist = getRouteWhiteListSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), whitelist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(30 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.WhiteListAnnotation))
			Expect(strings.Split(r.Annotations[controller.WhiteListAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132"))

			Expect(client.Delete(context.TODO(), whitelist)).Error().ShouldNot(HaveOccurred())

			time.Sleep(30 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.WhiteListAnnotation))
			Expect(strings.Split(r.Annotations[controller.WhiteListAnnotation], " ")).
				Should(ConsistOf("10.200.15.13"))
		})

	})
})
