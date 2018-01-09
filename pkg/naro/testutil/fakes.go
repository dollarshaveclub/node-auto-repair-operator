package testutil

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FakeKubeNode returns a *v1.Node.
func FakeKubeNode(t *testing.T) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "minikube",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: "minikube",
			},
		},
	}
}

// FakeKubeNodeEvent returns a fake *v1.Event.
func FakeKubeNodeEvent(t *testing.T) *v1.Event {
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			UID: "uid",
		},
		LastTimestamp: metav1.NewTime(time.Now()),
	}
}
