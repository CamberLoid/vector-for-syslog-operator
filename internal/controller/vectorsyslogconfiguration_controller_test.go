/*
Copyright 2026.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vectorsyslogv1alpha1 "lab.camber.moe/VectorForSyslogOperator/api/v1alpha1"
)

var _ = Describe("VectorSyslogConfiguration Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			configName = "test-config"
			sourceName = "test-source"
		)

		ctx := context.Background()

		configNamespacedName := types.NamespacedName{
			Name:      configName,
			Namespace: "default",
		}
		sourceNamespacedName := types.NamespacedName{
			Name:      sourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the VectorSocketSource first")
			source := &vectorsyslogv1alpha1.VectorSocketSource{}
			err := k8sClient.Get(ctx, sourceNamespacedName, source)
			if err != nil && errors.IsNotFound(err) {
				source = &vectorsyslogv1alpha1.VectorSocketSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sourceName,
						Namespace: "default",
						Labels: map[string]string{
							"app": "test",
						},
					},
					Spec: vectorsyslogv1alpha1.VectorSocketSourceSpec{
						Mode: "tcp",
						Port: 5140,
					},
				}
				Expect(k8sClient.Create(ctx, source)).To(Succeed())
			}

			By("creating the VectorSyslogConfiguration")
			config := &vectorsyslogv1alpha1.VectorSyslogConfiguration{}
			err = k8sClient.Get(ctx, configNamespacedName, config)
			if err != nil && errors.IsNotFound(err) {
				config = &vectorsyslogv1alpha1.VectorSyslogConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configName,
						Namespace: "default",
					},
					Spec: vectorsyslogv1alpha1.VectorSyslogConfigurationSpec{
						SourceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
						GlobalPipeline: vectorsyslogv1alpha1.GlobalPipelineSpec{
							Sinks: map[string]runtime.RawExtension{
								"console": {
									Raw: []byte(`{"type":"console","inputs":"$$VectorForSyslogOperatorSources$$","encoding":{"codec":"json"}}`),
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, config)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the VectorSyslogConfiguration")
			config := &vectorsyslogv1alpha1.VectorSyslogConfiguration{}
			err := k8sClient.Get(ctx, configNamespacedName, config)
			if err == nil {
				Expect(k8sClient.Delete(ctx, config)).To(Succeed())
			}

			By("Cleanup the VectorSocketSource")
			source := &vectorsyslogv1alpha1.VectorSocketSource{}
			err = k8sClient.Get(ctx, sourceNamespacedName, source)
			if err == nil {
				Expect(k8sClient.Delete(ctx, source)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &VectorSyslogConfigurationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: configNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
