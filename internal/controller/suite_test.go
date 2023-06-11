/*
Copyright 2023.

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
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"math/rand"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/matewolf/nginx-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc

	issuer     certv1.ClusterIssuer
	issuerName string
	namespace  = "default"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = clientgoscheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = operatorv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = certv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Register and start the Foo controller
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&NginxOperatorReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("nginx-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// Create issuer
	issuer = certv1.ClusterIssuer{
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				SelfSigned: &certv1.SelfSignedIssuer{},
			},
		},
	}

	// Deploy issuer
	issuerName = "issuer"
	issuer.SetName(issuerName)
	issuer.SetNamespace(namespace)
	err = k8sClient.Create(ctx, &issuer)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	k8sClient.Delete(ctx, &issuer)

})

var _ = Describe("NginxOperator controller", func() {
	When("When creating an NginxOperator instance", func() {
		var (
			nginxop  operatorv1alpha1.NginxOperator
			ownerref *metav1.OwnerReference
			name     string
			image    string
			hostname string
			port     int32
			replicas int32
			crIssuer string
		)

		BeforeEach(func() {
			// Create the MyResource instance
			image = fmt.Sprintf("nginx:latest")
			hostname = "hello-world.info"
			port = int32(80)
			crIssuer = "default/issuer"
			replicas = int32(1)
			nginxop = operatorv1alpha1.NginxOperator{
				Spec: operatorv1alpha1.NginxOperatorSpec{
					Hostname: &hostname,
					Port:     &port,
					Image:    &image,
					Issuer:   &crIssuer,
					Replicas: &replicas,
				},
			}

			// Deploy Nginxoperator
			name = fmt.Sprintf("nginxoperator-%d", rand.Intn(1000))
			nginxop.SetName(name)
			nginxop.SetNamespace(namespace)
			err := k8sClient.Create(ctx, &nginxop)
			Expect(err).NotTo(HaveOccurred())
			ownerref = metav1.NewControllerRef(
				&nginxop,
				operatorv1alpha1.GroupVersion.WithKind("NginxOperator"),
			)
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, &nginxop)
		})

		//It("should create services", func() {
		//	By("should create a deployment")
		//	var dep appsv1.Deployment
		//	Eventually(
		//		objectExists(name, namespace, &dep), 10, 1,
		//	).Should(BeTrue())
		//
		//	By("should create a service")
		//	var svc corev1.Service
		//	Eventually(
		//		objectExists(name, namespace, &svc), 10, 1,
		//	).Should(BeTrue())
		//
		//	By("should create an ingress")
		//	var ingress netv1.Ingress
		//	Eventually(
		//		objectExists(name, namespace, &ingress), 10, 1,
		//	).Should(BeTrue())
		//})
		//
		//When("deployment is found", func() {
		//	var dep appsv1.Deployment
		//	BeforeEach(func() {
		//		Eventually(
		//			objectExists(name, namespace, &dep),
		//			10, 1,
		//		).Should(BeTrue())
		//	})
		//
		//	It("should be owned by the NginxOperator instance", func() {
		//		Expect(dep.GetOwnerReferences()).
		//			To(ContainElement(*ownerref))
		//	})
		//
		//	It("should configured correctly", func() {
		//		By("should use image defined in NginxOperator instance")
		//		Expect(
		//			dep.Spec.Template.Spec.Containers[0].Image,
		//		).To(Equal(image))
		//
		//		By("should use the number of replica specified in NginxOperator instance")
		//		Expect(
		//			*dep.Spec.Replicas,
		//		).To(Equal(replicas))
		//
		//		By("should use port, defined in NginxOperator instance, as pod's container port")
		//		Expect(
		//			dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort,
		//		).To(Equal(port))
		//
		//		By("pod template labels should contain app name")
		//		Expect(
		//			dep.Spec.Template.Labels["app"],
		//		).To(Equal(name))
		//
		//		By("deployment selector should contain app name")
		//		Expect(
		//			dep.Spec.Selector.MatchLabels["app"],
		//		).To(Equal(name))
		//	})
		//
		//	It("annotations should have contain last modified value", func() {
		//		Expect(
		//			dep.ObjectMeta.Annotations[LastModifiedGeneration],
		//		).To(Equal("0"))
		//	})
		//})
		//
		//When("service is found", func() {
		//	var svc corev1.Service
		//	BeforeEach(func() {
		//		Eventually(
		//			objectExists(name, namespace, &svc),
		//			10, 1,
		//		).Should(BeTrue())
		//	})
		//
		//	It("target port should be the same defined in NginxOperator", func() {
		//		Expect(
		//			svc.Spec.Ports[0].TargetPort.IntVal,
		//		).To(Equal(port))
		//	})
		//
		//	It("selector should be the app name", func() {
		//		Expect(
		//			svc.Spec.Selector["app"],
		//		).To(Equal(name))
		//	})
		//
		//	It("annotations should have contain last modified value", func() {
		//		Expect(
		//			svc.ObjectMeta.Annotations[LastModifiedGeneration],
		//		).To(Equal("0"))
		//	})
		//
		//})
		//
		//When("ingress is found", func() {
		//	var ingress netv1.Ingress
		//	BeforeEach(func() {
		//		Eventually(
		//			objectExists(name, namespace, &ingress),
		//			10, 1,
		//		).Should(BeTrue())
		//	})
		//
		//	It("ingress annotation should contain issuer", func() {
		//		Expect(
		//			ingress.ObjectMeta.Annotations[ClusterIssuerAnnotation],
		//		).To(Equal(issuerName))
		//	})
		//
		//	It("ingress should be configured correctly", func() {
		//		By("tls hosts should contain hostname defined in NginxOperator")
		//		Expect(
		//			slices.ContainsFunc(ingress.Spec.TLS, func(tls netv1.IngressTLS) bool {
		//				return slices.Contains(tls.Hosts, hostname)
		//			}),
		//		).To(BeTrue())
		//
		//		By("rules should contain the a rule with a hostname defined in NginxOperator")
		//		Expect(ingress.Spec.Rules[0].Host).To(Equal(hostname))
		//
		//		By("and a service with a correct name")
		//		Expect(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name).To(Equal(name))
		//
		//	})
		//
		//	It("annotations should have contain last modified value", func() {
		//		Expect(
		//			ingress.ObjectMeta.Annotations[LastModifiedGeneration],
		//		).To(Equal("0"))
		//	})
		//})

		When("When the NginxOperator spec is changed", func() {
			var dep appsv1.Deployment
			var ingress netv1.Ingress
			var svc corev1.Service

			BeforeEach(func() {
				Eventually(
					objectExists(name, namespace, &dep), 10, 1,
				).Should(BeTrue())

				Eventually(
					objectExists(name, namespace, &svc), 10, 1,
				).Should(BeTrue())

				Eventually(
					objectExists(name, namespace, &ingress), 10, 1,
				).Should(BeTrue())
			})

			//When("replicas number is changed to 2", func() {
			//	var replica2 int32
			//
			//	BeforeEach(func() {
			//		replica2 = int32(2)
			//		k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &nginxop)
			//		nginxop.Spec.Replicas = &replica2
			//		err := k8sClient.Update(ctx, &nginxop)
			//		Expect(err).NotTo(HaveOccurred())
			//	})
			//
			//	It("deployment should be modified", func() {
			//		var currentDep appsv1.Deployment
			//		Eventually(assertOnObject(name, namespace, &currentDep, func(deployment *appsv1.Deployment) bool {
			//			return *deployment.Spec.Replicas == replica2
			//		}), 10, 1).Should(BeTrue())
			//	})
			//})
			//
			//When("hostname is changed to matewolf.dev", func() {
			//	var newHostname string
			//
			//	BeforeEach(func() {
			//		newHostname = "matewolf.dev"
			//		k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &nginxop)
			//		nginxop.Spec.Hostname = &newHostname
			//		err := k8sClient.Update(ctx, &nginxop)
			//		Expect(err).NotTo(HaveOccurred())
			//	})
			//
			//	It("ingress should be modified", func() {
			//		var currentIngress netv1.Ingress
			//		Eventually(
			//			assertOnObject(name, namespace, &currentIngress, func(t *netv1.Ingress) bool {
			//				return slices.Contains(t.Spec.TLS[0].Hosts, newHostname) &&
			//					t.Spec.Rules[0].Host == newHostname
			//			}), 10, 1,
			//		).Should(BeTrue())
			//	})
			//})
			//
			//When("image changed to nginx:1.25.0", func() {
			//	var newImage string
			//
			//	BeforeEach(func() {
			//		newImage = "nginx:1.25.0"
			//		k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &nginxop)
			//		nginxop.Spec.Image = &newImage
			//		err := k8sClient.Update(ctx, &nginxop)
			//		Expect(err).NotTo(HaveOccurred())
			//	})
			//
			//	It("deployment should be modified", func() {
			//		var currentDep appsv1.Deployment
			//		Eventually(assertOnObject(name, namespace, &currentDep, func(deployment *appsv1.Deployment) bool {
			//			return deployment.Spec.Template.Spec.Containers[0].Image == newImage
			//		}), 10, 1).Should(BeTrue())
			//	})
			//})

			When("issuer is changed to default/self-signed2", Ordered, func() {
				var newIssuerNamespacedName string
				var newIssuerName string

				var newIssuer certv1.Issuer

				BeforeAll(func() {
					// Create issuer
					newIssuer = certv1.Issuer{
						Spec: certv1.IssuerSpec{
							IssuerConfig: certv1.IssuerConfig{
								SelfSigned: &certv1.SelfSignedIssuer{},
							},
						},
					}

					// Deploy issuer
					newIssuerName = "issuer2"
					newIssuerNamespacedName = namespace + "/" + newIssuerName
					newIssuer.SetName(newIssuerName)
					newIssuer.SetNamespace(namespace)
					err := k8sClient.Create(ctx, &newIssuer)
					Expect(err).NotTo(HaveOccurred())
				})

				BeforeEach(func() {
					k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &nginxop)
					nginxop.Spec.Issuer = &newIssuerNamespacedName
					err := k8sClient.Update(ctx, &nginxop)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterAll(func() {
					k8sClient.Delete(ctx, &newIssuer)
				})

				It("ingress should be modified", func() {
					fmt.Println(ownerref)
					var currentIng netv1.Ingress
					Eventually(assertOnObject(name, namespace, &currentIng, func(ingress *netv1.Ingress) bool {
						return ingress.Annotations[IssuerAnnotation] == newIssuerName &&
							ingress.Annotations[ClusterIssuerAnnotation] == ""
					}), 10, 1).Should(BeTrue())
				})
			})

			When("when port is changed to 81", func() {
				var newPort int32

				BeforeEach(func() {
					newPort = int32(81)
					k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &nginxop)
					nginxop.Spec.Port = &newPort
					err := k8sClient.Update(ctx, &nginxop)
					Expect(err).NotTo(HaveOccurred())
				})

				It("deployment should be modified", func() {
					var currentDep appsv1.Deployment
					Eventually(assertOnObject(name, namespace, &currentDep, func(currentDep *appsv1.Deployment) bool {
						return currentDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort == newPort
					}), 10, 1).Should(BeTrue())
				})

				It("service should be modified", func() {
					var currentSvc corev1.Service
					Eventually(assertOnObject(name, namespace, &currentSvc, func(currentSvc *corev1.Service) bool {
						return currentSvc.Spec.Ports[0].TargetPort.IntVal == newPort
					}), 10, 1).Should(BeTrue())
				})
			})
		})
	})
})

func objectExists(
	name, namespace string, dep client.Object,
) func() bool {
	return func() bool {
		err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, dep)
		return err == nil
	}
}

func assertOnObject[T client.Object](name, namespace string, obj T, assertFunc func(T) bool) func() bool {
	return func() bool {
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj)
		if err != nil {
			return false
		}

		return assertFunc(obj)
	}
}
