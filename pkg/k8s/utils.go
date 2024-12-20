package k8s

import (
	"fmt"

	"github.com/paul-carlton/goutils/pkg/logging"
)

// convertLabelsToSelectorString converts map app: testapp to string app=testapp.
func (k *k8s) convertLabelToSelectorString(m map[string]string) string {
	logging.TraceCall()
	defer logging.TraceExit()

	var selector string
	for key, value := range m {
		selector = fmt.Sprintf("%s=%s", key, value)
		break
	}
	return selector
}
