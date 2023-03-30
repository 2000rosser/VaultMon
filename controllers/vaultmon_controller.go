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

	"os"
	"path/filepath"

	//	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/api/errors"

	rossoperatoriov1alpha1 "github.com/2000rosser/FYP.git/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VaultMonReconciler reconciles a VaultMon object
type VaultMonReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewVaultReconciler(client client.Client, scheme *runtime.Scheme) *VaultMonReconciler {
	return &VaultMonReconciler{
		Client: client,
		Scheme: scheme,
	}
}

//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VaultMon object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
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

	vaultData := &rossoperatoriov1alpha1.VaultMon{}
	//create a vault object if it doesn't exist
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

					VaultStatus: string(pod.Status.Conditions[0].Type),
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
			logger.Info("VAULT_STATUS=" + vaultData.Spec.VaultStatus)
		} else {
			logger.Error(err, "Failed to get VaultData")
			return ctrl.Result{}, err
		}
	}

	//check if the current value of name is the same as the one in the vault object
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

	if vaultData.Spec.VaultStatus != string(pod.Status.Conditions[0].Type) {
		vaultData.Spec.VaultStatus = string(pod.Status.Conditions[0].Type)
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return ctrl.Result{}, err
		}
		logger.Info("VaultData PodStatus updated to " + vaultData.Spec.VaultStatus)
	}

	//vault pod data directory
	// dataDir, err := clientset.CoreV1().Pods("default").Get(ctx, "vault-0", metav1.GetOptions{})
	// if err != nil {
	// 	panic(err.Error())
	// }

	//labels from data directory
	// dataDirLabels := dataDir.GetLabels()
	// for key, value := range dataDirLabels {
	// 	logger.Info("VAULT_DATA_DIR_LABELS=" + key + "=" + value)
	// }

	// logger.Info("VAULT_SECRETS=" + strconv.Itoa(10))

	// nextRun := time.Now().Add(10 * time.Second)
	// return ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}, nil

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VaultMonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := unstructured.Unstructured{Object: map[string]interface{}{}}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})

	return ctrl.NewControllerManagedBy(mgr).For(&u).Complete(NewVaultReconciler(mgr.GetClient(), mgr.GetScheme()))
}
