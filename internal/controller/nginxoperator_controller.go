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
	e "errors"
	"fmt"
	"github.com/matewolf/nginx-operator/assets"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	operatorv1alpha1 "github.com/matewolf/nginx-operator/api/v1alpha1"
)

var (
	CLUSTER_ISSUER_ANNOTATION = "cert-manager.io/cluster-issuer"
	ISSUER_ANNOTATION         = "cert-manager.io/issuer"
	LAST_MODIFIED_ANNOTATION  = "nginx-operator/last-modified"
)

// NginxOperatorReconciler reconciles a NginxOperator object
type NginxOperatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=operator.matewolf.dev,resources=nginxoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.matewolf.dev,resources=nginxoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.matewolf.dev,resources=nginxoperators/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func ignoreDeletionPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			fmt.Print("Update - ")
			switch e.ObjectOld.(type) {
			case *operatorv1alpha1.NginxOperator:
				fmt.Print("Operator -")
			case *appsv1.Deployment:
				fmt.Print("Deployment -")
			case *netv1.Ingress:
				fmt.Print("Ingress -")
			case *corev1.Service:
				fmt.Print("Service -")
			}
			if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
				fmt.Println("Mehet")
				return true
			} else {
				fmt.Println("Blokk")
				return false
			}
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			fmt.Print("Delete - ")

			switch e.Object.(type) {
			case *operatorv1alpha1.NginxOperator:
				fmt.Print("Operator -")
			case *appsv1.Deployment:
				fmt.Print("Deployment -")
			case *netv1.Ingress:
				fmt.Print("Ingress -")
			case *corev1.Service:
				fmt.Print("Service -")
			}
			return true
		},
		CreateFunc: func(createEvent event.CreateEvent) bool {
			fmt.Print("Create - ")
			switch createEvent.Object.(type) {
			case *operatorv1alpha1.NginxOperator:
				fmt.Print("Operator -")
			case *appsv1.Deployment:
				fmt.Print("Deployment -")
			case *netv1.Ingress:
				fmt.Print("Ingress -")
			case *corev1.Service:
				fmt.Print("Service -")
			}
			fmt.Println()
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			fmt.Print("Create - ")
			switch genericEvent.Object.(type) {
			case *operatorv1alpha1.NginxOperator:
				fmt.Print("Operator -")
			case *appsv1.Deployment:
				fmt.Print("Deployment -")
			case *netv1.Ingress:
				fmt.Print("Ingress -")
			case *corev1.Service:
				fmt.Print("Service -")
			}
			fmt.Println()
			return true
		},
	}
}

func (r *NginxOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile")

	// Try to get operator CR
	operatorCR := &operatorv1alpha1.NginxOperator{}
	err := r.Get(ctx, req.NamespacedName, operatorCR)
	if err != nil && errors.IsNotFound(err) {
		if errors.IsNotFound(err) {
			logger.Info("Operator custom resource not found in the cluster")
		} else {
			logger.Info("Loading operator custom resource is failed")
		}
		return ctrl.Result{}, nil
	}

	err = r.handleDeployment(ctx, req, operatorCR)
	if err != nil {
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, operatorCR)})
	}

	err = r.handleService(ctx, req, operatorCR)
	if err != nil {
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, operatorCR)})
	}

	err = r.handleIngress(ctx, req, operatorCR)
	if err != nil {
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, operatorCR)})
	}

	r.setCrTrueCondition(
		operatorCR,
		operatorv1alpha1.ReasonSucceeded,
		fmt.Sprintf("Successfull reconcile"),
	)

	return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, operatorCR)})
}

// SetupWithManager sets up the controller with the Manager.
func (r *NginxOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NginxOperator{}).
		Owns(&appsv1.Deployment{}).
		WithEventFilter(ignoreDeletionPredicate()).
		Complete(r)
}

func (r *NginxOperatorReconciler) handleDeployment(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator) error {
	logger := log.FromContext(ctx)
	deployment := &appsv1.Deployment{}

	err := r.Get(
		ctx,
		client.ObjectKey{
			Namespace: req.Namespace,
			Name:      req.Name,
		},
		deployment,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.createDeployment(ctx, req, operatorCR); err != nil {
				logger.Error(err, "Error creating operand deployment.")
				r.setCrFalseCondition(
					operatorCR,
					operatorv1alpha1.ReasonCreateDeploymentFailed,
					"Error creating operand deployment.",
				)
			}
			return nil
		} else {
			logger.Error(err, "Error getting existing operand deployment")
			r.setCrFalseCondition(
				operatorCR,
				operatorv1alpha1.ReasonDeploymentNotAvailable,
				"Error getting existing operand deployment",
			)
			return err
		}
	}

	if err = r.updateDeployment(ctx, req, operatorCR, deployment); err != nil {
		logger.Error(err, "Error update operand deployment")
		r.setCrFalseCondition(
			operatorCR,
			operatorv1alpha1.ReasonUpdateDeploymentFailed,
			"Error update operand deployment.",
		)
	}

	return nil
}

func (r *NginxOperatorReconciler) createDeployment(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator) error {
	deployment, err := assets.GetDeploymentFromFile("manifests/nginx_deployment.yaml")
	if err != nil {
		return err
	}

	deployment.Name = req.Name
	deployment.Namespace = req.Namespace
	deployment.ObjectMeta.Labels["app"] = req.Name
	deployment.Spec.Selector.MatchLabels["app"] = req.Name
	deployment.Spec.Template.ObjectMeta.Labels["app"] = req.Name

	if operatorCR.Spec.Image != nil {
		deployment.Spec.Template.Spec.Containers[0].Image = *operatorCR.Spec.Image
	}

	if operatorCR.Spec.Port != nil {
		deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = *operatorCR.Spec.Port
	}

	if operatorCR.Spec.Replicas != nil {
		deployment.Spec.Replicas = operatorCR.Spec.Replicas
	}

	if err = ctrl.SetControllerReference(operatorCR, deployment, r.Scheme); err != nil {
		return err
	}

	if err = r.Create(ctx, deployment); err != nil {
		return err
	}

	return nil
}

func (r *NginxOperatorReconciler) updateDeployment(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator, deployment *appsv1.Deployment) error {

	shouldUpdate := false

	if deployment.Spec.Template.Spec.Containers[0].Image != *operatorCR.Spec.Image {
		deployment.Spec.Template.Spec.Containers[0].Image = *operatorCR.Spec.Image
		shouldUpdate = true
	}

	if deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != *operatorCR.Spec.Port {
		deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = *operatorCR.Spec.Port
		shouldUpdate = true
	}

	if *deployment.Spec.Replicas != *operatorCR.Spec.Replicas {
		deployment.Spec.Replicas = operatorCR.Spec.Replicas
		shouldUpdate = true
	}

	if shouldUpdate {
		if err := ctrl.SetControllerReference(operatorCR, deployment, r.Scheme); err != nil {
			return err
		}

		if err := r.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

func (r *NginxOperatorReconciler) handleService(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator) error {
	logger := log.FromContext(ctx)
	service := &corev1.Service{}

	err := r.Get(ctx, req.NamespacedName, service)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.createService(ctx, req, operatorCR); err != nil {
				logger.Error(err, "Error creating operand service.")
				r.setCrFalseCondition(
					operatorCR,
					operatorv1alpha1.ReasonCreateServiceFailed,
					"Error creating operand service.",
				)
				return err
			}
		} else {
			logger.Error(err, "Error getting operand service.")
			r.setCrFalseCondition(
				operatorCR,
				operatorv1alpha1.ReasonServiceNotAvailable,
				"Error getting operand service.",
			)
			return err
		}
	}

	if err = r.updateService(ctx, req, operatorCR, service); err != nil {
		logger.Error(err, "Error updating operand service.")
		r.setCrFalseCondition(
			operatorCR,
			operatorv1alpha1.ReasonUpdateServiceFailed,
			"Error updating operand service.",
		)
		return err
	}

	return nil
}

func (r *NginxOperatorReconciler) createService(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator) error {
	service, err := assets.GetServiceFromFile("manifests/nginx_svc.yaml")
	if err != nil {
		return err
	}

	service.Name = req.Name
	service.Namespace = req.Namespace
	service.ObjectMeta.Labels["app"] = req.Name
	service.Spec.Selector["app"] = req.Name

	if operatorCR.Spec.Port != nil {
		service.Spec.Ports[0].TargetPort.IntVal = *operatorCR.Spec.Port
	}

	if err = ctrl.SetControllerReference(operatorCR, service, r.Scheme); err != nil {
		return err
	}

	if err = r.Create(ctx, service); err != nil {
		return err
	}

	return nil
}

func (r *NginxOperatorReconciler) updateService(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator, service *corev1.Service) error {
	shouldUpdate := false

	if operatorCR.Spec.Port != nil {
		if service.Spec.Ports[0].Port != *operatorCR.Spec.Port {
			service.Spec.Ports[0].Port = *operatorCR.Spec.Port
			shouldUpdate = true
		}
	}

	if shouldUpdate {
		if err := ctrl.SetControllerReference(operatorCR, service, r.Scheme); err != nil {
			return err
		}

		if err := r.Update(ctx, service); err != nil {
			return err
		}
	}

	return nil
}

func (r *NginxOperatorReconciler) handleIngress(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator) error {
	logger := log.FromContext(ctx)
	ingress := &netv1.Ingress{}

	err := r.Get(ctx, req.NamespacedName, ingress)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.createIngress(ctx, req, operatorCR); err != nil {
				logger.Error(err, "Error creating ingress.")
				r.setCrFalseCondition(
					operatorCR,
					operatorv1alpha1.ReasonCreateIngressFailed,
					"Error creating ingress.",
				)
				return err
			}
		} else {
			logger.Error(err, "Error getting operand ingress.")
			r.setCrFalseCondition(
				operatorCR,
				operatorv1alpha1.ReasonIngressNotAvailable,
				"Error getting operand ingress.",
			)
		}
	}

	if err = r.updateIngress(ctx, req, operatorCR, ingress); err != nil {
		logger.Error(err, "Error updating operand ingress.")
		r.setCrFalseCondition(
			operatorCR,
			operatorv1alpha1.ReasonUpdateIngressFailed,
			"Error updating operand ingress.",
		)
	}

	return nil
}

func (r *NginxOperatorReconciler) createIngress(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator) error {
	ingress, err := assets.GetIngressFromFile("manifests/nginx_ingress.yaml")
	if err != nil {
		return err
	}

	if err = r.setIssuerAnnotation(ctx, ingress, operatorCR); err != nil {
		return err
	}

	ingress.Name = req.Name
	ingress.Namespace = req.Namespace
	ingress.ObjectMeta.Labels["app"] = req.Name
	ingress.Spec.TLS[0] = netv1.IngressTLS{
		Hosts:      []string{*operatorCR.Spec.Hostname},
		SecretName: *operatorCR.Spec.Hostname,
	}
	pathType := netv1.PathTypePrefix
	ingress.Spec.Rules[0] = netv1.IngressRule{
		Host: *operatorCR.Spec.Hostname,
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: &pathType,
						Backend: netv1.IngressBackend{
							Service: &netv1.IngressServiceBackend{
								Name: req.Name,
								Port: netv1.ServiceBackendPort{
									Number: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	if err = ctrl.SetControllerReference(operatorCR, ingress, r.Scheme); err != nil {
		return err
	}

	if err = r.Create(ctx, ingress); err != nil {
		return err
	}

	return nil
}

func (r *NginxOperatorReconciler) updateIngress(ctx context.Context, req ctrl.Request, operatorCR *operatorv1alpha1.NginxOperator, ingress *netv1.Ingress) error {
	shouldUpdate := false
	if operatorCR.Spec.Hostname != nil {
		if ingress.Spec.TLS[0].Hosts[0] != *operatorCR.Spec.Hostname {
			shouldUpdate = true
			ingress.Spec.TLS[0] = netv1.IngressTLS{
				Hosts:      []string{*operatorCR.Spec.Hostname},
				SecretName: *operatorCR.Spec.Hostname,
			}
		}

		if ingress.Spec.Rules[0].Host != *operatorCR.Spec.Hostname {
			shouldUpdate = true
			pathType := netv1.PathTypePrefix
			ingress.Spec.Rules[0] = netv1.IngressRule{
				Host: *operatorCR.Spec.Hostname,
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: netv1.IngressBackend{
									Service: &netv1.IngressServiceBackend{
										Name: req.Name,
										Port: netv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			}
		}
	}

	issuerKey, err := r.getObjectKeyFromIssuer(*operatorCR.Spec.Issuer)
	if err != nil {
		return err
	}

	if ingress.Annotations[ISSUER_ANNOTATION] != issuerKey.Name ||
		ingress.Annotations[CLUSTER_ISSUER_ANNOTATION] != issuerKey.Name {
		shouldUpdate = true
		if err = r.setIssuerAnnotation(ctx, ingress, operatorCR); err != nil {
			return err
		}
	}

	if shouldUpdate {
		if err := ctrl.SetControllerReference(operatorCR, ingress, r.Scheme); err != nil {
			return err
		}

		if err := r.Update(ctx, ingress); err != nil {
			return err
		}
	}

	return nil
}

func (r *NginxOperatorReconciler) setIssuerAnnotation(ctx context.Context, ingress *netv1.Ingress, operatorCR *operatorv1alpha1.NginxOperator) error {
	key, err := r.getObjectKeyFromIssuer(*operatorCR.Spec.Issuer)
	if err != nil {
		return err
	}

	issuer, err := r.getIssuer(ctx, key)
	if err != nil {
		return err
	}

	switch issuer.(type) {
	case *cmapi.Issuer:
		ingress.Annotations[ISSUER_ANNOTATION] = key.Name
	case *cmapi.ClusterIssuer:
		ingress.Annotations[CLUSTER_ISSUER_ANNOTATION] = key.Name
	}

	return nil
}

func (r *NginxOperatorReconciler) getIssuer(ctx context.Context, key client.ObjectKey) (runtime.Object, error) {
	issuer := &cmapi.Issuer{}
	err := r.Get(ctx, key, issuer)
	if err == nil {
		return issuer, nil
	}

	clusterIssuer := &cmapi.ClusterIssuer{}
	err = r.Get(ctx, key, clusterIssuer)
	if err == nil {
		return clusterIssuer, nil
	}

	return nil, e.New("issuer not found")
}

func (r *NginxOperatorReconciler) getObjectKeyFromIssuer(issuerStr string) (client.ObjectKey, error) {
	parts := strings.Split(issuerStr, "/")

	if len(parts) != 2 {
		return client.ObjectKey{}, e.New("wrong issuer string format")
	}

	return client.ObjectKey{Namespace: parts[0], Name: parts[1]}, nil
}

func (r *NginxOperatorReconciler) setCrTrueCondition(cr *operatorv1alpha1.NginxOperator, reason, message string) {
	r.setCrCondition(cr, reason, message, metav1.ConditionTrue)
}

func (r *NginxOperatorReconciler) setCrFalseCondition(cr *operatorv1alpha1.NginxOperator, reason, message string) {
	r.setCrCondition(cr, reason, message, metav1.ConditionFalse)
}

func (r *NginxOperatorReconciler) setCrCondition(cr *operatorv1alpha1.NginxOperator, reason, message string, status metav1.ConditionStatus) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               "ReconcileFailed",
		Status:             status,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            message,
	})
}
