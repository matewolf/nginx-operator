package predicates

import "sigs.k8s.io/controller-runtime/pkg/event"

type NginxOpDeploymentPredicate struct {
}

func (n *NginxOpDeploymentPredicate) Create(event event.CreateEvent) bool {
	//TODO implement me
	panic("implement me")
}

func (n *NginxOpDeploymentPredicate) Delete(event event.DeleteEvent) bool {
	//TODO implement me
	panic("implement me")
}

func (n *NginxOpDeploymentPredicate) Update(event event.UpdateEvent) bool {
	//TODO implement me
	panic("implement me")
}

func (n *NginxOpDeploymentPredicate) Generic(event event.GenericEvent) bool {
	//TODO implement me
	panic("implement me")
}
