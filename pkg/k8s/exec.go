package k8s

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strings"

	v1 "k8s.io/api/core/v1"
	ksScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"
)

// ExecOptions passed to ExecWithOptions.
type ExecOptions struct {
	Command            []string
	Namespace          string
	PodName            string
	ContainerName      string
	Stdin              io.Reader
	CaptureStdout      bool
	CaptureStderr      bool
	PreserveWhitespace bool
}

func (k *k8s) HandleExecOutputs(stdOut, stdErr string, err error) error {
	logging.TraceCall()
	defer logging.TraceExit()

	miscutils.LogInfo(k.o, stdOut)
	if stdErr != "" {
		return fmt.Errorf("standard error return: %s", stdErr) //nolint:err113 // important we catch stdErr dynamically
	}
	return err
}

func (k *k8s) ExecuteCommandWithOptions(pod, namespace, container string, commands []string, stdin io.Reader) (string, string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	options := ExecOptions{
		Command:       commands,
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
		CaptureStdout: true,
		CaptureStderr: true,
		Stdin:         stdin,
	}
	return k.ExecPod(&options)
}

func (k *k8s) ExecPod(options *ExecOptions) (string, string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	req := k.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)

	req.VersionedParams(&v1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       false,
	}, ksScheme.ParameterCodec)

	var stdout, stderr bytes.Buffer
	err := k.execute("POST", req.URL(), options.Stdin, &stdout, &stderr, false)
	if options.PreserveWhitespace {
		return stdout.String(), stderr.String(), err
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func (k *k8s) execute(method string, url *url.URL, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	logging.TraceCall()
	defer logging.TraceExit()

	exec, err := remotecommand.NewSPDYExecutor(k.config, method, url)
	if err != nil {
		return err
	}
	return exec.StreamWithContext(k.o.Ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}
