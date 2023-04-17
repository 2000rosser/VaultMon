/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	rossoperatoriov1alpha1 "github.com/2000rosser/FYP.git/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	// corev1 "k8s.io/api/core/v1"

	"github.com/prometheus/client_golang/prometheus"
	//import the metrics package
)

// VaultMonReconciler reconciles a VaultMon object
type VaultMonReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	VaultMetrics *VaultMonMetrics
}

func NewVaultReconciler(client client.Client, scheme *runtime.Scheme, vaultMetrics *VaultMonMetrics) *VaultMonReconciler {
	return &VaultMonReconciler{
		Client:       client,
		Scheme:       scheme,
		VaultMetrics: vaultMetrics,
	}
}

//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="metrics.k8s.io",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts;deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *VaultMonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := ctrllog.FromContext(ctx)
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})
	if err := r.Get(ctx, req.NamespacedName, &u); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Reconciling Vault")

	// ingressList := &v1.IngressList{}
	// labelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": "vault"})
	// if err := r.Client.List(ctx, ingressList, client.InNamespace("default"), client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
	// 	logger.Error(err, "Failed to list Ingress resources")
	// 	return ctrl.Result{}, err
	// }
	// logger.Info("Ingress list fetched", "ingressList", ingressList)

	// var vaultIngress *v1.Ingress
	// if len(ingressList.Items) > 0 {
	// 	vaultIngress = &ingressList.Items[0]
	// } else {
	// 	logger.Info("No Ingress found for the Vault instance")
	// 	return ctrl.Result{}, nil
	// }

	//*****************************use this if the operator is ran outside the cluster******************************

	//path to the kubeconfig file
	kubeconfig := filepath.Join(
		os.Getenv("HOME"), ".kube", "config",
	)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	pod, err := clientset.CoreV1().Pods("default").Get(ctx, "vault-0", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	//******************************use this if the operator is ran outside the cluster******************************

	//*****************************use this if the operator is ran inside the cluster******************************

	// //instance of the kubernetes client
	// config, err := rest.InClusterConfig()
	// if err != nil {
	// 	panic(err.Error())
	// }

	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	panic(err.Error())
	// }

	// pod, err := clientset.CoreV1().Pods("default").Get(ctx, "vault-0", metav1.GetOptions{})
	// if err != nil {
	// 	panic(err.Error())
	// }

	//******************************use this if the operator is ran inside the cluster******************************

	logger.Info("Creating VaultData")
	logger.Info("Getting Vault Metrics")

	// metricsClientset, err := metricsclientset.NewForConfig(config)
	// if err != nil {
	// 	logger.Info("Error creating metrics clientset: " + err.Error())
	// }

	// vaultMetrics, err := metricsClientset.MetricsV1beta1().PodMetricses("default").Get(ctx, "vault-0", metav1.GetOptions{})
	// if err != nil {
	// 	logger.Info("Error getting vault metrics: " + err.Error())
	// }

	deployment, err := clientset.AppsV1().Deployments(u.GetNamespace()).Get(ctx, "vault-configurer", metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to get deployment")
		return ctrl.Result{}, err
	}

	//other image
	image, err := clientset.CoreV1().Pods("default").Get(ctx, "vault-0", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	logger.Info("VAULT_IMAGE=" + image.Spec.Containers[0].Image)

	vaultData := &rossoperatoriov1alpha1.VaultMon{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, vaultData); err != nil {
		if errors.IsNotFound(err) {
			vaultData = &rossoperatoriov1alpha1.VaultMon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.Name,
					Namespace: req.Namespace,
				},
				Spec: rossoperatoriov1alpha1.VaultMonSpec{
					VaultName: u.GetName(),

					VaultNamespace: u.GetNamespace(),

					VaultUid: string(u.GetUID()),

					VaultIp: pod.Status.PodIP,

					VaultStatus: pod.Status.ContainerStatuses,

					// VaultMemUsage: vaultMetrics.Containers[0].Usage.Memory().String(),

					// VaultCPUUsage: vaultMetrics.Containers[0].Usage.Cpu().String(),

					VaultReplicas: deployment.Status.Replicas,

					VaultImage: deployment.Spec.Template.Spec.Containers[0].Image,
				},
			}
			if err := r.Client.Create(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to create VaultData")
				return ctrl.Result{}, err
			}
			logger.Info("VaultData created")
			logger.Info("VAULT_NAME=" + string(vaultData.Spec.VaultName))
			logger.Info("VAULT_NAMESPACE=" + string(vaultData.Spec.VaultNamespace))
			logger.Info("VAULT_POD_IP=" + vaultData.Spec.VaultIp)
			logger.Info("VAULT_UID=" + vaultData.Spec.VaultUid)
			for _, status := range vaultData.Spec.VaultStatus {
				logger.Info("Container Name: " + status.Name)
				logger.Info("Container Ready: " + fmt.Sprintf("%t", status.Ready))
				logger.Info("Container Liveness: " + fmt.Sprintf("%t", status.State.Running != nil))
			}
			logger.Info("VAULT_MEMORY_USAGE=" + vaultData.Spec.VaultMemUsage)
			logger.Info("VAULT_CPU_USAGE=" + vaultData.Spec.VaultCPUUsage)
			logger.Info("VAULT_REPLICAS=" + string(vaultData.Spec.VaultReplicas))
			logger.Info("VAULT_IMAGE=" + vaultData.Spec.VaultImage)

			//log the value of vaultmetrics.vaultinfo
			logger.Info("Vault metrics value: " + fmt.Sprintf("%v", r.VaultMetrics.VaultInfo))

			r.VaultMetrics.VaultInfo.With(prometheus.Labels{
				"vaultName":      vaultData.Spec.VaultName,
				"vaultUid":       vaultData.Spec.VaultUid,
				"vaultNamespace": vaultData.Spec.VaultNamespace,
				"vaultIp":        vaultData.Spec.VaultIp,
				// "vaultReplicas": string(vaultData.Spec.VaultReplicas),
				"vaultImage": vaultData.Spec.VaultImage,
			}).Set(1)
			logger.Info("Vault metrics set")

		} else {
			logger.Error(err, "Failed to get VaultData")
			return ctrl.Result{}, err
		}
	}

	vaultFinalizer := "vault.banzaicloud.com/finalizer2"

	logger.Info("Vault finalizers" + fmt.Sprintf("%v", u.GetFinalizers()))

	if !containsString(u.GetFinalizers(), vaultFinalizer) {
		u.SetFinalizers(append(u.GetFinalizers(), vaultFinalizer))
		if err := r.Update(ctx, &u); err != nil {
			logger.Error(err, "Failed to add finalizer to Vault CRD")
			return ctrl.Result{}, err
		}
		logger.Info("Added finalizer to Vault CRD")
	}

	if !u.GetDeletionTimestamp().IsZero() {
		if containsString(u.GetFinalizers(), vaultFinalizer) {
			logger.Info("Deleting VaultData")
			u.SetFinalizers(removeString(u.GetFinalizers(), vaultFinalizer))
			if err := r.Update(ctx, &u); err != nil {
				logger.Error(err, "Failed to remove finalizer from Vault CRD")
				return ctrl.Result{}, err
			}
			logger.Info("Removed finalizer from Vault CRD")
		}
		return ctrl.Result{}, nil
	}

	if vaultData.Spec.VaultName != u.GetName() {
		vaultData.Spec.VaultName = u.GetName()
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData Name updated to " + vaultData.Spec.VaultName)
	}

	if vaultData.Spec.VaultNamespace != u.GetNamespace() {
		vaultData.Spec.VaultNamespace = u.GetNamespace()
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData Namespace updated to " + vaultData.Spec.VaultNamespace)
	}

	if vaultData.Spec.VaultUid != string(u.GetUID()) {
		vaultData.Spec.VaultUid = string(u.GetUID())
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData Uid updated to " + vaultData.Spec.VaultUid)
	}

	if vaultData.Spec.VaultIp != pod.Status.PodIP {
		vaultData.Spec.VaultIp = pod.Status.PodIP
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData PodIp updated to " + vaultData.Spec.VaultIp)
	}

	for _, status := range vaultData.Spec.VaultStatus {
		if status.Name != pod.Status.ContainerStatuses[0].Name {
			status.Name = pod.Status.ContainerStatuses[0].Name
			if err := r.Client.Update(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to update VaultData")
				return ctrl.Result{}, err
			}
			logger.Info("VaultData Container Name updated to " + status.Name)
		}
		if status.Ready != pod.Status.ContainerStatuses[0].Ready {
			status.Ready = pod.Status.ContainerStatuses[0].Ready
			if err := r.Client.Update(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to update VaultData")
				return ctrl.Result{}, err
			}
			logger.Info("VaultData Container Ready updated to " + fmt.Sprintf("%t", status.Ready))
		}
		if status.State.Running != pod.Status.ContainerStatuses[0].State.Running {
			status.State.Running = pod.Status.ContainerStatuses[0].State.Running
			if err := r.Client.Update(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to update VaultData")
				return ctrl.Result{}, err
			}
			logger.Info("VaultData Container Liveness updated to " + fmt.Sprintf("%t", status.State.Running != nil))
		}
	}

	// if vaultData.Spec.VaultMemUsage != vaultMetrics.Containers[0].Usage.Memory().String() {
	// 	vaultData.Spec.VaultMemUsage = vaultMetrics.Containers[0].Usage.Memory().String()
	// 	if err := r.Client.Update(ctx, vaultData); err != nil {
	// 		logger.Error(err, "Failed to update VaultData")
	// 		return ctrl.Result{}, err
	// 	}
	// 	logger.Info("VaultData MemoryUsage updated to " + vaultData.Spec.VaultMemUsage)
	// }

	// if vaultData.Spec.VaultCPUUsage != vaultMetrics.Containers[0].Usage.Cpu().String() {
	// 	vaultData.Spec.VaultCPUUsage = vaultMetrics.Containers[0].Usage.Cpu().String()
	// 	if err := r.Client.Update(ctx, vaultData); err != nil {
	// 		logger.Error(err, "Failed to update VaultData")
	// 		return ctrl.Result{}, err
	// 	}
	// 	logger.Info("VaultData CPUUsage updated to " + vaultData.Spec.VaultCPUUsage)
	// }

	if vaultData.Spec.VaultReplicas != deployment.Status.Replicas {
		vaultData.Spec.VaultReplicas = deployment.Status.Replicas
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData Replicas updated to " + string(vaultData.Spec.VaultReplicas))
	}

	if vaultData.Spec.VaultImage != deployment.Spec.Template.Spec.Containers[0].Image {
		vaultData.Spec.VaultImage = deployment.Spec.Template.Spec.Containers[0].Image
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData Image updated to " + vaultData.Spec.VaultImage)
	}

	// annotations := u.GetAnnotations()
	// logger.Info("Vault Annotations: " + fmt.Sprintf("%v", annotations))

	// nextRun := time.Now().Add(10 * time.Second)
	// return ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}, nil

	return ctrl.Result{}, nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	for i, item := range slice {
		if item == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// SetupWithManager sets up the controller with the Manager.
func (r *VaultMonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := unstructured.Unstructured{Object: map[string]interface{}{}}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})

	return ctrl.NewControllerManagedBy(mgr).For(&u).Complete(NewVaultReconciler(mgr.GetClient(), mgr.GetScheme(), r.VaultMetrics))
}
