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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	route "github.com/openshift/api/route/v1"
	networkingv1alpha1 "github.com/stakater/ipshield-operator/api/v1alpha1"
	"github.com/stakater/ipshield-operator/internal/controller"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

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

	Expect(utils.CreateNamespace(context.TODO(), clientset, OperatorNamespace)).To(Succeed())

})

var _ = Describe("controller", Ordered, func() {

	BeforeAll(func() {
		Expect(utils.CreateNamespace(context.TODO(), clientset, IPShieldCRNamespace)).To(Succeed())
		Expect(utils.CreateNamespace(context.TODO(), clientset, TestingNamespace)).To(Succeed())
		Expect(utils.CreateNginxDeployment(context.TODO(), client, RouteName, TestingNamespace)).
			To(Succeed())
		Expect(utils.CreateClusterIPService(context.TODO(), client, TestingNamespace, RouteName)).To(Succeed())

		Expect(utils.DeleteIfExists(context.TODO(), client, &route.Route{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      RouteName,
				Namespace: TestingNamespace,
			},
			Spec:   route.RouteSpec{},
			Status: route.RouteStatus{},
		})).To(Succeed())
	})

	AfterAll(func() {
		Expect(utils.DeleteIfExists(context.TODO(), client, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      RouteName,
				Namespace: TestingNamespace,
			}})).To(Succeed())

		Expect(utils.DeleteIfExists(context.TODO(), client, &v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      RouteName,
				Namespace: TestingNamespace,
			}})).To(Succeed())

		Expect(utils.DeleteNamespaceIfExists(context.TODO(), clientset, TestingNamespace)).To(Succeed())
		Expect(utils.DeleteNamespaceIfExists(context.TODO(), clientset, IPShieldCRNamespace)).To(Succeed())
	})

	Context("Operator", func() {
		var allowlist *networkingv1alpha1.RouteAllowlist

		It("should run successfully", func() {
			Skip("Skipping installation test")
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/ipshield-operator:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", OperatorNamespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", OperatorNamespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

		})

		BeforeEach(func() {

			Expect(utils.Run(exec.Command("oc", "expose", "svc", RouteName, "-n", TestingNamespace))).
				Error().
				NotTo(HaveOccurred())

			Expect(utils.Run(exec.Command("kubectl",
				"label",
				"route",
				RouteName,
				controller.IPShieldWatchedResourceLabel+"-",
				"-n", TestingNamespace))).
				Error().
				ShouldNot(HaveOccurred())

			Expect(utils.Run(exec.Command("kubectl",
				"label",
				"route", RouteName,
				"ipshield=true",
				"-n", TestingNamespace))).
				Error().
				ShouldNot(HaveOccurred())

			time.Sleep(3 * time.Second)
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), allowlist))

			Expect(utils.Run(exec.Command("kubectl", "delete", "route", RouteName, "-n", TestingNamespace))).
				Error().
				ShouldNot(HaveOccurred())
			time.Sleep(3 * time.Second)
		})

		It("Deploy CR but route doesn't have label", func() {
			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13"})
			err := client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			r := &route.Route{}
			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())
			Expect(r.Labels).ShouldNot(HaveKey(controller.IPShieldWatchedResourceLabel))
			Expect(r.Annotations).ShouldNot(HaveKeyWithValue(controller.AllowlistAnnotation, "10.200.15.13"))
		})

		It("Deploy CR and route has label", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())
			Expect(r.Annotations).ShouldNot(HaveKey(controller.IPShieldWatchedResourceLabel))

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(3 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Labels).Should(HaveKeyWithValue(controller.IPShieldWatchedResourceLabel, strconv.FormatBool(true)))
			Expect(r.Annotations).Should(HaveKeyWithValue(controller.AllowlistAnnotation, "10.200.15.13"))
		})

		It("Deploy CR and route already had allowlist 1 element", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			r.Annotations[controller.AllowlistAnnotation] = "192.168.10.32"

			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(5 * time.Second)

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(5 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132", "192.168.10.32"))

		})

		It("Route and CR had one common IP", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			r.Annotations[controller.AllowlistAnnotation] = "10.200.15.13"

			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(5 * time.Second)

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(5 * time.Second)

			Expect(client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)).
				Should(Succeed())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132"))

			Expect(client.Delete(context.TODO(), allowlist)).Error().ShouldNot(HaveOccurred())

			time.Sleep(5 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13"))
		})

		It("IPShield watch label was updated to false when route initially had pre populated allowlist", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			r.Annotations[controller.AllowlistAnnotation] = "10.200.15.13"

			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(5 * time.Second)

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(5 * time.Second)

			Expect(client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r))
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132"))

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(false)
			Expect(client.Update(context.TODO(), r)).Error().ShouldNot(HaveOccurred())

			time.Sleep(5 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13"))
		})

		It("IPShield watch label was updated to false when route initially had empty allowlist", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)

			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(5 * time.Second)

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(5 * time.Second)

			Expect(client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r))
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132"))

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(false)
			Expect(client.Update(context.TODO(), r)).Error().ShouldNot(HaveOccurred())

			time.Sleep(5 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).ShouldNot(HaveKey(controller.AllowlistAnnotation))
		})

		It("allowlist annotation was modified directly", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(5 * time.Second)

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(5 * time.Second)

			Expect(client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)).
				To(Succeed())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132"))

			r.Annotations[controller.AllowlistAnnotation] = "10.13.42.54"
			Expect(client.Update(context.TODO(), r)).Error().ShouldNot(HaveOccurred())

			time.Sleep(5 * time.Second)

			err = client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132", "10.13.42.54"))
		})

		It("has multiple CRs", func() {
			r := &route.Route{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			r.Labels[controller.IPShieldWatchedResourceLabel] = strconv.FormatBool(true)
			Expect(client.Update(context.TODO(), r)).NotTo(HaveOccurred())

			time.Sleep(5 * time.Second)

			allowlist = utils.GetRouteAllowlistSpec("sample", IPShieldCRNamespace, []string{"10.200.15.13", "10.200.15.132"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			allowlist := utils.GetRouteAllowlistSpec("samplex2", IPShieldCRNamespace, []string{"10.200.15.14", "10.200.15.135"})
			err = client.Create(context.TODO(), allowlist)
			Expect(err).NotTo(HaveOccurred())

			// takes a while to update the route
			time.Sleep(5 * time.Second)

			Expect(client.Get(context.TODO(), types.NamespacedName{Name: RouteName, Namespace: TestingNamespace}, r)).
				To(Succeed())
			Expect(r).NotTo(BeNil())

			Expect(r.Annotations).Should(HaveKey(controller.AllowlistAnnotation))
			Expect(strings.Split(r.Annotations[controller.AllowlistAnnotation], " ")).
				Should(ConsistOf("10.200.15.13", "10.200.15.132", "10.200.15.14", "10.200.15.135"))

			Expect(client.Delete(context.TODO(), allowlist)).Error().ShouldNot(HaveOccurred())
		})
	})

})
