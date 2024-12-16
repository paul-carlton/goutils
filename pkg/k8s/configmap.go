package k8s

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nabancard/goutils/pkg/logging"
)

func (k *k8s) GetConfigMapData(name, namespace string) (map[string]string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	c := k.client.CoreV1().ConfigMaps(namespace)
	cm, err := c.Get(ctx, name, metav1.GetOptions{})
	return cm.Data, err
}
