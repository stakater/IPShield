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

package controller

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/route/v1"
	"github.com/stakater/ipshield-operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	scheme2 "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/stakater/ipshield-operator/api/v1alpha1"
)

var _ = Describe("RouteAllowlist Controller", Ordered, func() {

	var (
		ctx        context.Context
		reconciler *RouteAllowlistReconciler
		allowlist  *networkingv1alpha1.RouteAllowlist
		r          *unstructured.Unstructured
		fakeClient client.Client
	)

	ctx = context.Background()

	BeforeEach(func() {
		ctx = context.Background()
		scheme := scheme2.Scheme
		Expect(networkingv1alpha1.AddToScheme(scheme)).To(Succeed())

		r = &unstructured.Unstructured{}
		r.SetUnstructuredContent(map[string]interface{}{
			"apiVersion": "route.openshift.io/v1",
			"kind":       "Route",
			"metadata": map[string]interface{}{
				"name":      "test-route",
				"namespace": "default",
				"labels": map[string]interface{}{
					"app":                        "test",
					"ipshield":                   "true",
					IPShieldWatchedResourceLabel: "true",
				},
				"annotations": map[string]string{
					"openshift.io/host.generated": "true",
				},
			},
			"spec": map[string]interface{}{
				"host": "test.example.com",
				"tls":  map[string]interface{}{},
			},
		})

		// Create config map
		configMap := &unstructured.Unstructured{}
		configMap.SetUnstructuredContent(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "watched-routes",
				"namespace": DefaultWatchNamespace,
			},
			"data": nil,
		})

		allowlist = utils.GetRouteAllowlistSpec("test-route", DefaultWatchNamespace, []string{"10.100.123.24"})

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(r, allowlist, configMap).
			WithStatusSubresource(r, allowlist, configMap).
			Build()

		reconciler = &RouteAllowlistReconciler{
			Client:             fakeClient,
			Scheme:             scheme,
			WatchNamespace:     DefaultWatchNamespace,
			WatchResourceLabel: IPShieldWatchedResourceLabel,
		}
	})

	AfterEach(func() {
	})

	It("should successfully reconcile the resource", func() {
		By("Reconciling the created resource")

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: allowlist.Namespace,
				Name:      allowlist.Name,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		osRoute := &v1.Route{}
		err = fakeClient.Get(ctx, types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}, osRoute)

		Expect(err).NotTo(HaveOccurred())
		Expect(osRoute.Annotations).To(HaveKeyWithValue(AllowlistAnnotation, "10.100.123.24"))

		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: allowlist.Namespace, Name: allowlist.Name}, allowlist)).Should(Succeed())
		Expect(allowlist.Status.Conditions).To(HaveLen(1))

		Expect(allowlist.Status.Conditions[0].Type).Should(Equal("Admitted"))

		watchedRoutes := &corev1.ConfigMap{}
		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: DefaultWatchNamespace, Name: WatchedRoutesConfigMapName}, watchedRoutes)).Should(Succeed())
		Expect(watchedRoutes.Data).To(HaveKey(fmt.Sprintf("%s__%s", osRoute.Namespace, osRoute.Name)))
		ok, err := controllerutil.HasOwnerReference(watchedRoutes.OwnerReferences, allowlist, scheme2.Scheme)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It(" will test that backup is stored in configmap", func() {
		By("Reconciling the created resource")

		osRoute := &v1.Route{}
		err := fakeClient.Get(ctx, types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}, osRoute)
		Expect(err).NotTo(HaveOccurred())

		osRoute.Annotations[AllowlistAnnotation] = "10.33.52.5"
		Expect(fakeClient.Update(ctx, osRoute)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: allowlist.Namespace,
				Name:      allowlist.Name,
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}, osRoute)).To(Succeed())
		Expect(osRoute.Annotations).To(HaveKey(AllowlistAnnotation))
		Expect(strings.Split(osRoute.Annotations[AllowlistAnnotation], " ")).Should(ConsistOf([]string{"10.100.123.24", "10.33.52.5"}))

		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: allowlist.Namespace, Name: allowlist.Name}, allowlist)).Should(Succeed())
		Expect(allowlist.Status.Conditions).To(HaveLen(1))

		Expect(allowlist.Status.Conditions[0].Type).Should(Equal("Admitted"))

		watchedRoutes := &corev1.ConfigMap{}
		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: DefaultWatchNamespace, Name: WatchedRoutesConfigMapName}, watchedRoutes)).Should(Succeed())
		Expect(watchedRoutes.Data).To(HaveKeyWithValue(fmt.Sprintf("%s__%s", osRoute.Namespace, osRoute.Name), "10.33.52.5"))
	})

	It("will test that backup is properly cleaned up in configmap", func() {
		By("Reconciling the created resource")

		osRoute := &v1.Route{}
		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}, osRoute)).To(Succeed())

		osRoute.Annotations[AllowlistAnnotation] = "10.33.52.5"
		Expect(fakeClient.Update(ctx, osRoute)).Should(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: allowlist.Namespace,
				Name:      allowlist.Name,
			},
		})

		Expect(err).NotTo(HaveOccurred())

		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}, osRoute)).To(Succeed())
		Expect(osRoute.Annotations).To(HaveKey(AllowlistAnnotation))
		Expect(strings.Split(osRoute.Annotations[AllowlistAnnotation], " ")).Should(ConsistOf([]string{"10.100.123.24", "10.33.52.5"}))

		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: allowlist.Namespace, Name: allowlist.Name}, allowlist)).Should(Succeed())
		Expect(allowlist.Status.Conditions).To(HaveLen(1))

		Expect(allowlist.Status.Conditions[0].Type).Should(Equal("Admitted"))

		watchedRoutes := &corev1.ConfigMap{}
		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: DefaultWatchNamespace, Name: WatchedRoutesConfigMapName}, watchedRoutes)).Should(Succeed())
		Expect(watchedRoutes.Data).To(HaveKeyWithValue(fmt.Sprintf("%s__%s", osRoute.Namespace, osRoute.Name), "10.33.52.5"))

		Expect(fakeClient.Delete(ctx, allowlist)).Should(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: allowlist.Namespace,
				Name:      allowlist.Name,
			},
		})

		Expect(err).NotTo(HaveOccurred())

		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}, osRoute)).To(Succeed())
		Expect(osRoute.Annotations).To(HaveKey(AllowlistAnnotation))
		Expect(strings.Split(osRoute.Annotations[AllowlistAnnotation], " ")).Should(ConsistOf([]string{"10.33.52.5"}))

		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: DefaultWatchNamespace, Name: WatchedRoutesConfigMapName}, watchedRoutes)).Should(Succeed())
		Expect(watchedRoutes.Data).ShouldNot(HaveKey(fmt.Sprintf("%s__%s", osRoute.Namespace, osRoute.Name)))

		ok, err := controllerutil.HasOwnerReference(watchedRoutes.OwnerReferences, allowlist, scheme2.Scheme)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})
})
