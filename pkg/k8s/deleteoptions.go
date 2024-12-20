package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/paul-carlton/goutils/pkg/logging"
)

const (
	DeploymentGracePeriod    int64 = 30
	KustomizationGracePeriod int64 = 30
	JobGracePeriod           int64 = 0
	ImmediateGracePeriod     int64 = 0
)

func (k *k8s) getMetaV1DeleteOptions(gracePeriod int64) metav1.DeleteOptions {
	logging.TraceCall()
	defer logging.TraceExit()

	policy := metav1.DeletePropagationForeground
	options := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
		PropagationPolicy:  &policy,
	}
	return options
}

func (k *k8s) getCtrlDeleteOptions(gracePeriod int64) *ctrlclient.DeleteOptions {
	logging.TraceCall()
	defer logging.TraceExit()

	policy := metav1.DeletePropagationForeground
	options := &ctrlclient.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
		PropagationPolicy:  &policy,
	}
	return options
}
