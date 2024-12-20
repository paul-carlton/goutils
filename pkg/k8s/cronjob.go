package k8s

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"
)

func (k *k8s) DeleteCronJob(name, namespace string, gracePeriod int64, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Second*60)
	defer cancel()
	dc := k.client.BatchV1().CronJobs(namespace)
	do := k.getMetaV1DeleteOptions(gracePeriod)
	if err := dc.Delete(ctx, name, do); err != nil {
		return err
	}
	miscutils.LogInfo(k.o, "waiting for job deletion")
	return k.waitForCronJobDeletion(name, namespace, waitFor)
}

func (k *k8s) waitForCronJobDeletion(name, namespace string, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, waitFor+time.Second*30)
	defer cancel()
	jc := k.client.BatchV1().Jobs(namespace)
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, waitFor, true,
		func(context.Context) (done bool, err error) {
			_, err = jc.Get(k.o.Ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				// return error as nil as this is the desired result:
				return true, nil
			}
			return false, err
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}
