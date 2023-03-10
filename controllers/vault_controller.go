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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	//vaultbanzaicloudcomv1alpha1 "github.com/2000rosser/FYP.git/api/v1alpha1"
)

// VaultReconciler reconciles a Vault object
type VaultReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Vault object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
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
	logger.Info("VAULT_UID=" + string(u.GetUID()))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VaultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := unstructured.Unstructured{Object: map[string]interface{}{}}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})
	return ctrl.NewControllerManagedBy(mgr).For(&u).Complete(r)
}
