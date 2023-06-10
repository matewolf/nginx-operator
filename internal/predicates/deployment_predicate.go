package predicates

import (
	"fmt"
	"github.com/matewolf/nginx-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"strconv"
)

var (
	LastModifiedGeneration = "nginxoperator/last-modified-generation"
)

type NginxOperatorPredicate struct {
}

func NewNginxOperatorPredicate() *NginxOperatorPredicate {
	return &NginxOperatorPredicate{}
}

func (n *NginxOperatorPredicate) Create(event event.CreateEvent) bool {
	fmt.Println("Create")
	_, isCr := event.Object.(*v1alpha1.NginxOperator)
	if isCr {
		return true
	}
	return false
}

func (n *NginxOperatorPredicate) Delete(event event.DeleteEvent) bool {
	fmt.Println("Delete")

	_, isCr := event.Object.(*v1alpha1.NginxOperator)
	if isCr {
		return false
	}

	return !event.DeleteStateUnknown
}

func (n *NginxOperatorPredicate) Update(event event.UpdateEvent) bool {
	fmt.Println("Update")

	if event.ObjectNew.GetGeneration() == n.getLastModifiedAnnotation(event.ObjectOld)+1 ||
		event.ObjectNew.GetGeneration() == event.ObjectOld.GetGeneration() {
		return false
	}
	return true
}

func (n *NginxOperatorPredicate) Generic(event event.GenericEvent) bool {
	fmt.Println("Generic")

	return true
}

func (n *NginxOperatorPredicate) getLastModifiedAnnotation(object client.Object) int64 {
	lastModifiedGen, ok := object.GetAnnotations()[LastModifiedGeneration]
	if !ok {
		return 0
	}

	iLastModifiedGen, err := strconv.Atoi(lastModifiedGen)
	if err != nil {
		return 0
	}

	return int64(iLastModifiedGen)
}
