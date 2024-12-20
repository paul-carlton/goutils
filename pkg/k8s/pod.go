package k8s

import (
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	v1pod "k8s.io/kubernetes/pkg/api/v1/pod"

	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"
)

func (k *k8s) GetPodsFromLabelSelector(selector, namespace string) (*corev1.PodList, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	listOptions := metav1.ListOptions{LabelSelector: selector}
	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Minute*10)
	defer cancel()
	return k.client.CoreV1().Pods(namespace).List(ctx, listOptions)
}

func (k *k8s) GetPodNamesFromLabelSelector(selector, namespace string) ([]string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	pods, err := k.GetPodsFromLabelSelector(selector, namespace)
	if err != nil {
		return []string{}, err
	}
	names := []string{}
	for _, pod := range pods.Items {
		names = append(names, pod.Name)
	}
	return names, nil
}

func (k *k8s) GetPodNameFromLabelSelector(selector, namespace string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	pods, err := k.GetPodsFromLabelSelector(selector, namespace)
	if err != nil {
		return "", err
	}
	return k.getPodNameFromSingleList(pods)
}

// getPodNameFromList will only work for single element lists.
func (k *k8s) getPodNameFromSingleList(pods *corev1.PodList) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	if len(pods.Items) > 1 {
		return "", fmt.Errorf("expecting 1 pod from list but got %d", len(pods.Items)) //nolint:err113 // capturing number of pods needed
	}
	name := pods.Items[0].Name
	return name, nil
}

func (k *k8s) WaitForPodReadyStatus(name, namespace string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Minute)
	defer cancel()
	pc := k.client.CoreV1().Pods(namespace)
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, 3*time.Minute, true,
		func(context.Context) (done bool, err error) {
			pod, err := pc.Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return v1pod.IsPodReady(pod), nil
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}

// WaitForPodsExist checks only if a pod resource exists that match labels, it doesnt about ready status.
func (k *k8s) WaitForPodsExist(namespace, selector string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Minute)
	defer cancel()
	pc := k.client.CoreV1().Pods(namespace)
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, 3*time.Minute, true,
		func(context.Context) (done bool, err error) {
			pods, err := pc.List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil || len(pods.Items) == 0 {
				return false, err
			}
			return true, nil
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}

func (k *k8s) WaitForPodDeletion(name, namespace string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Minute)
	defer cancel()
	pc := k.client.CoreV1().Pods(namespace)
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, 3*time.Minute, true,
		func(context.Context) (done bool, err error) {
			_, err = pc.Get(ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false, err
			}
			return true, nil
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}

func (k *k8s) DeletePod(name, namespace string, grace int64) error {
	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Minute*10)
	defer cancel()
	pc := k.client.CoreV1().Pods(namespace)
	err := pc.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &grace})

	// add wait.
	return err
}

func (k *k8s) RestartPod(namespace, selector string, grace int64) error {
	logging.TraceCall()
	defer logging.TraceExit()

	podName, err := k.GetPodNameFromLabelSelector(selector, namespace)
	if err != nil {
		return err
	}
	if err := k.DeletePod(podName, namespace, grace); err != nil {
		return err
	}
	num := time.Duration(grace)
	miscutils.LogInfoBlue(k.o, "waiting for pod to delete")
	time.Sleep(num * time.Second)
	// once grace period ends, just sleep a bit more so we dont catch the deleted pod.
	time.Sleep(5 * time.Second)
	if err := k.WaitForPodsExist(namespace, selector); err != nil {
		return err
	}
	// GetPodNameFromLabelSelector only works if the pod resource has been created, otherwise will fail.
	_, err = k.GetPodNameFromLabelSelector(selector, namespace)
	return err
}

func (k *k8s) RemoveFilesFromPod(pod, namespace, container string, files ...string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	for _, f := range files {
		command := fmt.Sprintf("rm -f %s", f)
		stdOut, stdErr, err := k.ExecuteCommandWithOptions(pod, namespace, container, []string{"bash", "-c", command}, nil)
		if err := k.HandleExecOutputs(stdOut, stdErr, err); err != nil {
			return err
		}
	}
	return nil
}

func (k *k8s) CopyFileToPod(pod, namespace, container, outfile string, readin io.Reader) error {
	logging.TraceCall()
	defer logging.TraceExit()

	command := fmt.Sprintf("cp /dev/stdin %s", outfile)
	stdOut, stdErr, err := k.ExecuteCommandWithOptions(pod, namespace, container, []string{"bash", "-c", command}, readin)
	return k.HandleExecOutputs(stdOut, stdErr, err)
}
