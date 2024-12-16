package fake

import (
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConfigMap(name, namespace string, data map[string]string) *v1.ConfigMap {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	return configMap
}

func Secret(name, namespace string, data map[string][]byte) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	return secret
}

func Pod(name, namespace, container, image string, labels map[string]string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  container,
					Image: image,
				},
			},
		},
	}
	return pod
}

func newTestPodTemplateSpec(name, image string, labels map[string]string) v1.PodTemplateSpec {
	podTemplate := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  name,
					Image: image,
				},
			},
		},
	}
	return podTemplate
}

func Deployment(name, container, namespace string, replicas *int32, labelSelector map[string]string) *appsV1.Deployment {
	obj := metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
	podTemplate := newTestPodTemplateSpec(container, namespace, labelSelector)
	spec := appsV1.DeploymentSpec{Replicas: replicas, Selector: &metav1.LabelSelector{MatchLabels: labelSelector}, Template: podTemplate}
	dep := &appsV1.Deployment{ObjectMeta: obj, Spec: spec}
	return dep
}
