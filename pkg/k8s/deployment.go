package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	k8swait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"

	"k8s.io/apimachinery/pkg/api/errors"
)

func (k *k8s) DeleteDeployment(name, namespace string, gracePeriod int64, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Minute*10)
	defer cancel()
	dc := k.client.AppsV1().Deployments(namespace)
	do := k.getMetaV1DeleteOptions(gracePeriod)
	if err := dc.Delete(ctx, name, do); err != nil {
		if errors.IsNotFound(err) {
			miscutils.LogError(k.o, "deployment not found")
			return nil
		}
		return err
	}
	miscutils.LogInfo(k.o, "waiting for deployment deletion")
	return k.WaitForDeploymentDeletion(name, namespace, waitFor)
}

func (k *k8s) WaitForDeploymentDeletion(name, namespace string, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if waitFor.Seconds() == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Minute)
	defer cancel()
	dc := k.client.AppsV1().Deployments(namespace)
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, 3*time.Minute, true,
		func(context.Context) (done bool, err error) {
			_, err = dc.Get(k.o.Ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				// return error as nil as this is the desired result.
				return true, nil
			}
			return false, err
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}

func (k *k8s) waitForReplicasToScale(name, namespace, selector string, replicas int32) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Minute)
	defer cancel()
	dc := k.client.AppsV1().Deployments(namespace)
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, 3*time.Minute, true,
		func(context.Context) (done bool, err error) {
			deploy, err := dc.Get(k.o.Ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if deploy != nil &&
				deploy.Spec.Replicas != nil &&
				replicas == *deploy.Spec.Replicas &&
				replicas == deploy.Status.ReadyReplicas {
				if replicas == 0 {
					// deployment spec/status will show as 0 replicas but
					// pods wont delete until default grace period ends.
					miscutils.LogInfoBlue(k.o, fmt.Sprintf("waiting for %s pods to scale to 0", selector))
					pods, _ := k.GetPodsFromLabelSelector(selector, namespace) //nolint: errcheck // err is not needed
					if len(pods.Items) == 0 {
						return true, err
					}
					return false, err
				}
				return true, nil
			}
			return false, nil
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}

func (k *k8s) ScaleDeployments(names []string, namespace string, replicas int32) error {
	logging.TraceCall()
	defer logging.TraceExit()

	for _, n := range names {
		if err := k.ScaleDeployment(n, namespace, replicas); err != nil {
			return err
		}
	}
	return nil
}

func (k *k8s) ScaleDeployment(name, namespace string, replicas int32) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Minute*10)
	defer cancel()
	dc := k.client.AppsV1().Deployments(namespace)
	s, err := dc.GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	sc := *s
	sc.Spec.Replicas = replicas
	_, err = dc.UpdateScale(ctx,
		name, &sc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	depl, err := dc.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	labels := depl.Spec.Template.Labels
	selector := k.convertLabelToSelectorString(labels)
	return k.waitForReplicasToScale(name, namespace, selector, replicas)
}

func (k *k8s) RolloutRestartDeployment(name, namespace string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	dc := k.client.AppsV1().Deployments(namespace)
	data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
	_, err := dc.Patch(k.o.Ctx, name, k8stypes.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	return err
}
