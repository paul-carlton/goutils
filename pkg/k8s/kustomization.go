package k8s

import (
	"context"
	"fmt"
	"time"

	kustomize "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	apimeta "github.com/fluxcd/pkg/apis/meta"
	gitrepo "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nabancard/goutils/pkg/logging"
	"github.com/nabancard/goutils/pkg/miscutils"
)

const (
	notFoundStatus  = "notFound"
	notReadyStatus  = "notReady"
	suspendedStatus = "suspended"
)

func ksDefaultFilterFunc(_ *kustomize.Kustomization) bool {
	logging.TraceCall()
	defer logging.TraceExit()

	return true
}

func (k *k8s) GetKustomizations(ksFilterFunc func(ks *kustomize.Kustomization) bool) ([]*kustomize.Kustomization, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	if ksFilterFunc == nil {
		ksFilterFunc = ksDefaultFilterFunc
	}

	ksList := &kustomize.KustomizationList{}
	err := k.cc.List(k.o.Ctx, ksList)
	if err != nil {
		return nil, err
	}

	matchList := []*kustomize.Kustomization{}
	for _, ks := range ksList.Items {
		k.o.Log.Debug("kustomization found", "name", ks.Name)
		if ksFilterFunc(&ks) {
			matchList = append(matchList, &ks)
		}
	}
	return matchList, nil
}

func (k *k8s) GetKustomization(name, namespace string) (*kustomize.Kustomization, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Second*30)
	defer cancel()
	kustomization := &kustomize.Kustomization{}
	ksKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	err := k.cc.Get(ctx, ksKey, kustomization)
	return kustomization, err
}

func (k *k8s) SuspendKustomization(kustomization *kustomize.Kustomization) error {
	logging.TraceCall()
	defer logging.TraceExit()

	return k.updateSuspendKustomization(kustomization, true)
}

func (k *k8s) ResumeKustomization(kustomization *kustomize.Kustomization) error {
	logging.TraceCall()
	defer logging.TraceExit()

	return k.updateSuspendKustomization(kustomization, false)
}

func (k *k8s) updateSuspendKustomization(kustomization *kustomize.Kustomization, suspend bool) (err error) {
	logging.TraceCall()
	defer logging.TraceExit()

	if suspend {
		miscutils.LogWarning(k.o, fmt.Sprintf("suspending Kustomization: %s, in namespace: %s",
			kustomization.Name, kustomization.Namespace))
	} else {
		miscutils.LogInfo(k.o, fmt.Sprintf("resuming Kustomization: %s, in namespace: %s",
			kustomization.Name, kustomization.Namespace))
	}
	kustomization.Spec.Suspend = suspend
	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Second*30)
	defer cancel()
	return k.cc.Update(ctx, kustomization)
}

func (k *k8s) CheckKustomzationStatus(kustomization *kustomize.Kustomization) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	// Check if kustomization is ready, if it is, return early.
	if meta.IsStatusConditionTrue(kustomization.Status.Conditions, apimeta.ReadyCondition) {
		miscutils.LogInfo(k.o, "Kustomization is already showing as ready")
		return apimeta.ReadyCondition, nil
	}
	// if the kustomization is not suspended, attempt to reconcile.
	if kustomization.Spec.Suspend {
		miscutils.LogWarning(k.o, "Kustomization is suspended!")
		return suspendedStatus, nil
	}
	return notReadyStatus, nil
}

func (k *k8s) ReconcileKustomization(kustomization *kustomize.Kustomization, waitFor time.Duration) (err error) {
	logging.TraceCall()
	defer logging.TraceExit()

	if err := k.patchReconcileAnnotation(kustomization); err != nil {
		return err
	}
	return k.WaitForReconciledKustomization(kustomization, waitFor)
}

func (k *k8s) patchReconcileAnnotation(kustomization *kustomize.Kustomization) error {
	logging.TraceCall()
	defer logging.TraceExit()

	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"reconcile.fluxcd.io/requestedAt": %q}}}`, time.Now().Format(time.RFC3339)))
	err := k.cc.Patch(k.o.Ctx, kustomization, ctrlclient.RawPatch(k8stypes.MergePatchType, patch))
	if err != nil {
		miscutils.LogError(k.o, fmt.Sprintf("Error patching annotation for Kustomization: %s", kustomization.Name))
		miscutils.LogError(k.o, fmt.Sprintf("Error: %s", err))
		return err
	}
	return nil
}

func (k *k8s) WaitForReconciledKustomization(kustomization *kustomize.Kustomization, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if waitFor.Seconds() == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(k.o.Ctx, waitFor+time.Second*10)
	defer cancel()

	key := ctrlclient.ObjectKey{
		Namespace: kustomization.Namespace,
		Name:      kustomization.Name,
	}
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, waitFor, true,
		func(context.Context) (done bool, err error) {
			if err := k.cc.Get(k.o.Ctx, key, kustomization); err != nil {
				return false, err
			}
			return meta.IsStatusConditionTrue(kustomization.Status.Conditions, apimeta.ReadyCondition), nil
		}); err != nil {
		miscutils.LogError(k.o, fmt.Sprint(err.Error()))
		return err
	}
	return nil
}

func (k *k8s) NewKustomization(name, namespace, sourceRepo, ksPath string, postBuild *kustomize.PostBuild, depends []apimeta.NamespacedObjectReference) *kustomize.Kustomization {
	logging.TraceCall()
	defer logging.TraceExit()

	ks := &kustomize.Kustomization{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: k.createKustomizationSpec(ksPath, sourceRepo, postBuild, depends),
	}
	return ks
}

func (k *k8s) createKustomizationSpec(ksPath, sourceRepo string, postBuild *kustomize.PostBuild, dependsOn []apimeta.NamespacedObjectReference) kustomize.KustomizationSpec {
	logging.TraceCall()
	defer logging.TraceExit()

	spec := kustomize.KustomizationSpec{
		DependsOn:     dependsOn,
		Interval:      metav1.Duration{Duration: 3 * time.Minute},
		RetryInterval: &metav1.Duration{Duration: 30 * time.Second},
		Timeout:       &metav1.Duration{Duration: 1 * time.Minute},
		Prune:         true,
		Path:          ksPath,
		Wait:          true,
		SourceRef:     kustomize.CrossNamespaceSourceReference{Kind: gitrepo.GitRepositoryKind, Name: sourceRepo},
		PostBuild:     postBuild,
	}
	return spec
}

func (k *k8s) DeleteKustomization(kustomization *kustomize.Kustomization, gracePeriod int64, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(k.o.Ctx, time.Minute*10)
	defer cancel()

	options := k.getCtrlDeleteOptions(gracePeriod)
	if err := k.cc.Delete(ctx, kustomization, options); err != nil {
		return err
	}
	return k.WaitForKustomizationDeletion(kustomization, waitFor)
}

func (k *k8s) WaitForKustomizationDeletion(kustomization *kustomize.Kustomization, waitFor time.Duration) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if waitFor.Seconds() == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(k.o.Ctx, waitFor)
	defer cancel()
	key := k8stypes.NamespacedName{
		Namespace: kustomization.Namespace,
		Name:      kustomization.Name,
	}
	obj := metav1.ObjectMeta{
		Namespace: kustomization.Namespace,
		Name:      kustomization.Name,
	}
	if err := k8swait.PollUntilContextTimeout(ctx, 3*time.Second, waitFor, true,
		func(context.Context) (done bool, err error) {
			err = k.cc.Get(k.o.Ctx, key, &kustomize.Kustomization{ObjectMeta: obj})
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
