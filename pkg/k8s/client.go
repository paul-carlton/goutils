package k8s

import (
	"cmp"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	kustomize "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	apimeta "github.com/fluxcd/pkg/apis/meta"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nabancard/goutils/pkg/logging"
	"github.com/nabancard/goutils/pkg/miscutils"
)

const (
	NoWait = time.Second * 0
)

type k8s struct {
	K8s
	o      *miscutils.NewObjParams
	cc     ctrlclient.Client
	client kubernetes.Interface
	config *rest.Config
}

type K8s interface {
	GetKubeConfig() *rest.Config
	SetKubeConfig(*rest.Config) error
	GetKubeClient() kubernetes.Interface
	SetKubeClient(client kubernetes.Interface) error
	GetCtrlClient() ctrlclient.Client
	SetCtrlClient(client ctrlclient.Client, ctrlScheme *runtime.Scheme) error

	DeleteDeployment(name, namespace string, gracePeriod int64, waitFor time.Duration) error
	WaitForDeploymentDeletion(name, namespace string, waitFor time.Duration) error
	waitForReplicasToScale(name, namespace, selector string, replicas int32) error
	ScaleDeployments(names []string, namespace string, replicas int32) error
	ScaleDeployment(name, namespace string, replicas int32) error
	RolloutRestartDeployment(name, namespace string) error
	GetPodsFromLabelSelector(selector, namespace string) (*corev1.PodList, error)
	GetPodNamesFromLabelSelector(selector, namespace string) ([]string, error)
	GetPodNameFromLabelSelector(selector, namespace string) (string, error)
	getPodNameFromSingleList(pods *corev1.PodList) (string, error)
	WaitForPodReadyStatus(name, namespace string) error
	WaitForPodsExist(namespace, selector string) error
	WaitForPodDeletion(name, namespace string) error
	DeletePod(name, namespace string, grace int64) error
	RestartPod(namespace, selector string, grace int64) error
	RemoveFilesFromPod(pod, namespace, container string, files ...string) error
	CopyFileToPod(pod, namespace, container, outfile string, readin io.Reader) error
	HandleExecOutputs(stdOut, stdErr string, err error) error
	ExecuteCommandWithOptions(pod, namespace, container string, commands []string, stdin io.Reader) (string, string, error)
	ExecPod(options *ExecOptions) (string, string, error)
	execute(method string, url *url.URL, stdin io.Reader, stdout, stderr io.Writer, tty bool) error
	GetSecretData(name, namespace string) (map[string][]byte, error)
	convertLabelToSelectorString(m map[string]string) string
	GetConfigMapData(name, namespace string) (map[string]string, error)
	GetKustomizations(ksFilterFunc func(ks *kustomize.Kustomization) bool) ([]*kustomize.Kustomization, error)
	GetKustomization(name, namespace string) (*kustomize.Kustomization, error)
	SuspendKustomization(kustomization *kustomize.Kustomization) error
	ResumeKustomization(kustomization *kustomize.Kustomization) error
	updateSuspendKustomization(kustomization *kustomize.Kustomization, suspend bool) (err error)
	CheckKustomzationStatus(kustomization *kustomize.Kustomization) (string, error)
	ReconcileKustomization(kustomization *kustomize.Kustomization, waitFor time.Duration) (err error)
	patchReconcileAnnotation(kustomization *kustomize.Kustomization) error
	WaitForReconciledKustomization(kustomization *kustomize.Kustomization, waitFor time.Duration) error
	NewKustomization(name, namespace, sourceRepo, ksPath string, postBuild *kustomize.PostBuild, depends []apimeta.NamespacedObjectReference) *kustomize.Kustomization
	createKustomizationSpec(ksPath, sourceRepo string, postBuild *kustomize.PostBuild, dependsOn []apimeta.NamespacedObjectReference) kustomize.KustomizationSpec
	DeleteKustomization(kustomization *kustomize.Kustomization, gracePeriod int64, waitFor time.Duration) error
	WaitForKustomizationDeletion(kustomization *kustomize.Kustomization, waitFor time.Duration) error
	getMetaV1DeleteOptions(gracePeriod int64) metav1.DeleteOptions
	getCtrlDeleteOptions(gracePeriod int64) *ctrlclient.DeleteOptions
	DeleteCronJob(name, namespace string, gracePeriod int64, waitFor time.Duration) error
	waitForCronJobDeletion(name, namespace string, waitFor time.Duration) error
}

func NewK8s(objParams *miscutils.NewObjParams, config *rest.Config, ctrlClient ctrlclient.Client, client kubernetes.Interface, scheme *runtime.Scheme) (K8s, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	k := k8s{
		o: objParams,
	}

	err := k.SetKubeConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to set kube config, error: %w", err)
	}

	err = k.SetKubeClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to set kube client, error: %w", err)
	}

	err = k.SetCtrlClient(ctrlClient, scheme)
	if err != nil {
		k.o.Log.Error("failed to set controller client, error", "error", err)
	}

	return &k, nil
}

var scheme *runtime.Scheme //nolint: gochecknoglobals

func init() {
	scheme = runtime.NewScheme()
	_ = kustomize.AddToScheme(scheme) //nolint: errcheck
	_ = corev1.AddToScheme(scheme)    //nolint: errcheck

	leveler := uzap.LevelEnablerFunc(func(level zapcore.Level) bool {
		// Set the level fairly high since it's so verbose
		return level >= zapcore.DPanicLevel
	})
	stackTraceLeveler := uzap.LevelEnablerFunc(func(_ zapcore.Level) bool {
		// Attempt to suppress the stack traces in the logs since they are so verbose.
		// The controller runtime seems to ignore this since the stack is still always printed.
		return false
	})
	logfmtEncoder := zaplogfmt.NewEncoder(uzap.NewProductionEncoderConfig())
	logger := zap.New(
		zap.Level(leveler),
		zap.StacktraceLevel(stackTraceLeveler),
		zap.UseDevMode(false),
		zap.WriteTo(os.Stdout),
		zap.Encoder(logfmtEncoder))

	ctrllog.SetLogger(logger)
}

func (k *k8s) SetCtrlClient(client ctrlclient.Client, ctrlScheme *runtime.Scheme) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if client != nil {
		k.cc = client
		return nil
	}

	if ctrlScheme == nil {
		ctrlScheme = scheme
	}

	var err error
	k.cc, err = ctrlclient.New(k.config, ctrlclient.Options{Scheme: ctrlScheme})
	return err
}

func (k *k8s) GetCtrlClient() ctrlclient.Client {
	logging.TraceCall()
	defer logging.TraceExit()

	return k.cc
}

func (k *k8s) SetKubeClient(client kubernetes.Interface) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if client != nil {
		k.client = client
		return nil
	}

	var err error
	k.client, err = kubernetes.NewForConfig(k.config)
	return err
}

func (k *k8s) GetKubeClient() kubernetes.Interface {
	logging.TraceCall()
	defer logging.TraceExit()

	return k.client
}

func (k *k8s) SetKubeConfig(config *rest.Config) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if config != nil {
		k.config = config
		return nil
	}
	config, err := rest.InClusterConfig()
	k.o.Log.Debug("Failed to find In-Cluster Config, attempting to use KUBECONFIG")
	if err != nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			k.o.Log.Error("Failed to get user's home directory")
			homeDir = "."
		}
		kubeconfig := cmp.Or(os.Getenv("KUBECONFIG"), fmt.Sprintf("%s/.kube/config", homeDir))
		var kerr error
		k.config, kerr = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if kerr != nil {
			return fmt.Errorf("failed to get In-Cluster Config, error: %w and failed to get config from KUBECONFIG: %s, error: %w",
				err, kubeconfig, kerr)
		}
	} else {
		k.config = config
	}

	return nil
}

func (k *k8s) GetKubeConfig() *rest.Config {
	logging.TraceCall()
	defer logging.TraceExit()

	return k.config
}
