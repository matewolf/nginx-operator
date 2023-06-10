package assets

import (
	"embed"
	"errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	//go:embed manifests/*
	manifests  embed.FS
	appsScheme = runtime.NewScheme()
	appsCodecs = serializer.NewCodecFactory(appsScheme)
)

func init() {
	if err := appsv1.AddToScheme(appsScheme); err != nil {
		panic(err)
	}

	if err := corev1.AddToScheme(appsScheme); err != nil {
		panic(err)
	}

	if err := netv1.AddToScheme(appsScheme); err != nil {
		panic(err)
	}
}

func GetObjectFromFile(name string, version schema.GroupVersion) (runtime.Object, error) {
	objectBytes, err := manifests.ReadFile(name)
	if err != nil {
		return nil, err
	}
	object, err := runtime.Decode(
		appsCodecs.UniversalDecoder(version),
		objectBytes,
	)
	if err != nil {
		return nil, err
	}

	return object, nil
}

func GetDeploymentFromFile(name string) (*appsv1.Deployment, error) {
	object, err := GetObjectFromFile(name, appsv1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}

	deploymentObject, ok := object.(*appsv1.Deployment)
	if !ok {
		return nil, errors.New("object is not a deployment object")
	}

	return deploymentObject, nil
}

func GetServiceFromFile(name string) (*corev1.Service, error) {
	object, err := GetObjectFromFile(name, corev1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}

	svcObject, ok := object.(*corev1.Service)
	if !ok {
		return nil, errors.New("object is not a service object")
	}

	return svcObject, nil
}

func GetIngressFromFile(name string) (*netv1.Ingress, error) {
	object, err := GetObjectFromFile(name, netv1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}

	ingressObject, ok := object.(*netv1.Ingress)
	if !ok {
		return nil, errors.New("object is not a service object")
	}

	return ingressObject, nil
}
