package k8s

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/paul-carlton/goutils/pkg/logging"
)

func (k *k8s) GetSecretData(name, namespace string) (map[string][]byte, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Minute*10)
	defer cancel()
	c := k.client.CoreV1().Secrets(namespace)
	s, err := c.Get(ctx, name, metav1.GetOptions{})
	return s.Data, err
}
