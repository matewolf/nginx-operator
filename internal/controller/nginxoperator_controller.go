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
	"github.com/matewolf/nginx-operator/internal/predicates"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"strconv"
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
	ClusterIssuerAnnotation = "cert-manager.io/cluster-issuer"
	IssuerAnnotation        = "cert-manager.io/issuer"
	LastModifiedGeneration  = "nginxoperator/last-modified-generation"
)

// NginxOperatorReconciler reconciles a NginxOperator object
type NginxOperatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=operator.matewolf.dev,resources=nginxoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.matewolf.dev,resources=nginxoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.matewolf.dev,resources=nginxoperators/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=``,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cert-manager.io,resources=clusterissuers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

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

	if err = r.validateCR(operatorCR); err != nil {
		logger.Error(err, "Error validating custom resource")
		r.setCrTrueCondition(operatorCR, operatorv1alpha1.ReasonCustomResourceInvalid, err.Error())
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, operatorCR)})
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

	r.setCrFalseCondition(
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
		Owns(&corev1.Service{}).
		Owns(&netv1.Ingress{}).
		WithEventFilter(predicates.NewNginxOperatorPredicate()).
		Complete(r)
}

func (r *NginxOperatorReconciler) validateCR(cr *operatorv1alpha1.NginxOperator) error {
	if cr.Spec.Image == nil || cr.Spec.Hostname == nil || cr.Spec.Issuer == nil {
		return e.New("At least one of Image, Hostname and Issuer is invalid")
	}
	return nil
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
				r.setCrTrueCondition(
					operatorCR,
					operatorv1alpha1.ReasonCreateDeploymentFailed,
					"Error creating operand deployment.",
				)
				r.Recorder.Event(operatorCR, corev1.EventTypeWarning, operatorv1alpha1.ReasonCreateDeploymentFailed, fmt.Sprintf("Error creating deployment."))
				return err
			}
			return nil
		} else {
			logger.Error(err, "Error getting existing operand deployment")
			r.setCrTrueCondition(
				operatorCR,
				operatorv1alpha1.ReasonDeploymentNotAvailable,
				"Error getting existing operand deployment",
			)
			return err
		}
	}

	if err = r.updateDeployment(ctx, req, operatorCR, deployment); err != nil {
		logger.Error(err, "Error update operand deployment")
		r.setCrTrueCondition(
			operatorCR,
			operatorv1alpha1.ReasonUpdateDeploymentFailed,
			"Error update operand deployment.",
		)
		r.Recorder.Event(operatorCR, corev1.EventTypeWarning, operatorv1alpha1.ReasonUpdateDeploymentFailed, fmt.Sprintf("Error update deployment."))
		return err
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

	deployment.Spec.Template.Spec.Containers[0].Image = *operatorCR.Spec.Image

	if operatorCR.Spec.Port != nil {
		deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = *operatorCR.Spec.Port
	}

	if operatorCR.Spec.Replicas != nil {
		deployment.Spec.Replicas = operatorCR.Spec.Replicas
	}

	r.setLastModifiedGen(deployment)

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

	if operatorCR.Spec.Port != nil {
		if deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != *operatorCR.Spec.Port {
			deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = *operatorCR.Spec.Port
			shouldUpdate = true
		}
	}

	if operatorCR.Spec.Replicas != nil {
		if *deployment.Spec.Replicas != *operatorCR.Spec.Replicas {
			deployment.Spec.Replicas = operatorCR.Spec.Replicas
			shouldUpdate = true
		}
	}

	if shouldUpdate {
		r.setLastModifiedGen(deployment)

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
				r.setCrTrueCondition(
					operatorCR,
					operatorv1alpha1.ReasonCreateServiceFailed,
					"Error creating operand service.",
				)
				r.Recorder.Event(operatorCR, corev1.EventTypeWarning, operatorv1alpha1.ReasonCreateServiceFailed, fmt.Sprintf("Error create service: %s", err.Error()))
				return err
			}
			return nil
		} else {
			logger.Error(err, "Error getting operand service.")
			r.setCrTrueCondition(
				operatorCR,
				operatorv1alpha1.ReasonServiceNotAvailable,
				"Error getting operand service.",
			)
			return err
		}
	}

	if err = r.updateService(ctx, req, operatorCR, service); err != nil {
		logger.Error(err, "Error updating operand service.")
		r.setCrTrueCondition(
			operatorCR,
			operatorv1alpha1.ReasonUpdateServiceFailed,
			"Error updating operand service.",
		)
		r.Recorder.Event(operatorCR, corev1.EventTypeWarning, operatorv1alpha1.ReasonUpdateServiceFailed, fmt.Sprintf("Error update service: %s", err.Error()))
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

	r.setLastModifiedGen(service)

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
		r.setLastModifiedGen(service)

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
				r.setCrTrueCondition(
					operatorCR,
					operatorv1alpha1.ReasonCreateIngressFailed,
					"Error creating ingress.",
				)
				r.Recorder.Event(operatorCR, corev1.EventTypeWarning, operatorv1alpha1.ReasonCreateIngressFailed, fmt.Sprintf("Error create ingress: %s", err.Error()))
				return err
			}
			return nil
		} else {
			logger.Error(err, "Error getting operand ingress.")
			r.setCrTrueCondition(
				operatorCR,
				operatorv1alpha1.ReasonIngressNotAvailable,
				"Error getting operand ingress.",
			)
			return err
		}
	}

	if err = r.updateIngress(ctx, req, operatorCR, ingress); err != nil {
		logger.Error(err, "Error updating operand ingress.")
		r.setCrTrueCondition(
			operatorCR,
			operatorv1alpha1.ReasonUpdateIngressFailed,
			"Error updating operand ingress.",
		)
		r.Recorder.Event(operatorCR, corev1.EventTypeWarning, operatorv1alpha1.ReasonUpdateIngressFailed, fmt.Sprintf("Error update ingress: %s", err.Error()))
		return err
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

	r.setLastModifiedGen(ingress)

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

	if ingress.Annotations[IssuerAnnotation] != issuerKey.Name &&
		ingress.Annotations[ClusterIssuerAnnotation] != issuerKey.Name {
		shouldUpdate = true
		if err = r.setIssuerAnnotation(ctx, ingress, operatorCR); err != nil {
			return err
		}
	}

	if shouldUpdate {
		r.setLastModifiedGen(ingress)

		if err = ctrl.SetControllerReference(operatorCR, ingress, r.Scheme); err != nil {
			return err
		}

		if err = r.Update(ctx, ingress); err != nil {
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
		ingress.Annotations[IssuerAnnotation] = key.Name
	case *cmapi.ClusterIssuer:
		ingress.Annotations[ClusterIssuerAnnotation] = key.Name
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

func (r *NginxOperatorReconciler) setLastModifiedGen(obj client.Object) {
	currentAnnotations := obj.GetAnnotations()
	if currentAnnotations == nil {
		currentAnnotations = map[string]string{}
	}
	currentAnnotations[LastModifiedGeneration] = strconv.Itoa(int(obj.GetGeneration()))
	obj.SetAnnotations(currentAnnotations)
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
