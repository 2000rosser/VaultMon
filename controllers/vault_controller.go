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
	"strconv"

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

	vaultbanzaicloudcomv1alpha1 "github.com/2000rosser/FYP.git/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func (r *VaultReconciler) getOrCreateConfigMap(ctx context.Context, namespace string, name string) (*v1.ConfigMap, error) {
// 	configMap := &v1.ConfigMap{}
// 	err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configMap)
// 	if err != nil {
// 		if errors.IsNotFound(err) {
// 			// Create a new ConfigMap if it doesn't exist
// 			configMap = &v1.ConfigMap{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      name,
// 					Namespace: namespace,
// 				},
// 				Data: make(map[string]string),
// 			}
// 			if err := r.Client.Create(ctx, configMap); err != nil {
// 				return nil, err
// 			}
// 		} else {
// 			return nil, err
// 		}
// 	}
// 	return configMap, nil
// }

//var globalConfigMapData map[string]string

// VaultReconciler reconciles a Vault object
type VaultReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	GlobalConfigMapData map[string]string
}

// Constructor function for VaultReconciler
func NewVaultReconciler(client client.Client, scheme *runtime.Scheme) *VaultReconciler {
	return &VaultReconciler{
		Client:              client,
		Scheme:              scheme,
		GlobalConfigMapData: make(map[string]string),
	}
}

//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *VaultReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// configMapNamespace := u.GetNamespace()
	// configMapName := "vault-configmap"

	// configMap := &corev1.ConfigMap{}
	// err := r.Client.Get(ctx, client.ObjectKey{Namespace: configMapNamespace, Name: configMapName}, configMap)

	// // initialize the configmap if its empty
	// if len(configMap.Data) == 0 {
	// 	configMap.Data = make(map[string]string)

	// 	//get the vault info and store it in the configmap
	// 	configMap.Data["vault-name"] = u.GetName()
	// 	logger.Info("VAULT_NAME=" + string(u.GetName()))

	// 	configMap.Data["vault-uid"] = string(u.GetUID())
	// 	logger.Info("VAULT_UID=" + string(u.GetUID()))

	// 	configMap.Data["vault-namespace"] = u.GetNamespace()
	// 	logger.Info("VAULT_NAMESPACE=" + string(u.GetNamespace()))

	// 	if err := r.Client.Update(ctx, configMap); err != nil {
	// 		logger.Error(err, "Failed to update ConfigMap")
	// 		return ctrl.Result{}, err
	// 	}
	// 	logger.Info("ConfigMap initialized")
	// }

	logger.Info("Creating VaultData")

	vaultData := &vaultbanzaicloudcomv1alpha1.Vault{}
	//create a vault object if it doesn't exist
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, vaultData); err != nil {
		if errors.IsNotFound(err) {
			vaultData = &vaultbanzaicloudcomv1alpha1.Vault{
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.Name,
					Namespace: req.Namespace,
				},
				Spec: vaultbanzaicloudcomv1alpha1.VaultSpec{
					// VaultName:      configMap.Data["vault-name"],
					// VaultUID:       configMap.Data["vault-uid"],
					// VaultNamespace: configMap.Data["vault-namespace"],
				},
			}
			if err := r.Client.Create(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to create VaultData")
				return ctrl.Result{}, err
			}
			logger.Info("VaultData created")
		} else {
			logger.Error(err, "Failed to get VaultData")
			return ctrl.Result{}, err
		}
	}

	// if info is different update the configmap
	// if configMap.Data["vault-uid"] != string(u.GetUID()) {
	// 	configMap.Data["vault-uid"] = string(u.GetUID())
	// 	logger.Info("vault-uid changed, updating ConfigMap")
	// 	if err := r.Client.Update(ctx, configMap); err != nil {
	// 		logger.Error(err, "Failed to update ConfigMap")
	// 		return ctrl.Result{}, err
	// 	}
	// }

	// if configMap.Data["vault-namespace"] != u.GetNamespace() {
	// 	configMap.Data["vault-namespace"] = u.GetNamespace()
	// 	logger.Info("vault-namespace changed, updating ConfigMap")
	// 	if err := r.Client.Update(ctx, configMap); err != nil {
	// 		logger.Error(err, "Failed to update ConfigMap")
	// 		return ctrl.Result{}, err
	// 	}
	// }

	// annotations := u.GetAnnotations()
	// for key, value := range annotations {
	// 	logger.Info("VAULT_ANNOTATIONS=" + key + "=" + value)
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

	vaultData.Spec.VaultName = u.GetName()
	//log the vault name using the vaultData object
	logger.Info("VAULT_NAME=" + string(vaultData.Spec.VaultName))

	vaultData.Spec.VaultNamespace = u.GetNamespace()
	//log the vault namespace using the vaultData object
	logger.Info("VAULT_NAMESPACE=" + string(vaultData.Spec.VaultNamespace))

	vaultData.Spec.VaultIp = pod.Status.PodIP
	logger.Info("VAULT_POD_IP=" + vaultData.Spec.VaultIp)

	vaultData.Spec.VaultUid = string(u.GetUID())
	logger.Info("VAULT_UID=" + vaultData.Spec.VaultUid)

	//vault pod data directory
	dataDir, err := clientset.CoreV1().Pods("default").Get(ctx, "vault-0", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	//labels from data directory
	dataDirLabels := dataDir.GetLabels()
	for key, value := range dataDirLabels {
		logger.Info("VAULT_DATA_DIR_LABELS=" + key + "=" + value)
	}

	logger.Info("VAULT_SECRETS=" + strconv.Itoa(10))

	// nextRun := time.Now().Add(10 * time.Second)
	// return ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}, nil

	return ctrl.Result{}, nil
}

//create a function that watches for changes in the vault pod

// SetupWithManager sets up the controller with the Manager.

func (r *VaultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := unstructured.Unstructured{Object: map[string]interface{}{}}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})

	return ctrl.NewControllerManagedBy(mgr).For(&u).Complete(NewVaultReconciler(mgr.GetClient(), mgr.GetScheme()))
}
