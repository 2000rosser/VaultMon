package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	rossoperatoriov1alpha1 "github.com/2000rosser/FYP.git/api/v1alpha1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
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
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rossoperator.io,resources=vaultmons,verbs=get;list;watch;create;update;patch;delete

func (r *VaultMonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := ctrllog.FromContext(ctx)

	u, err := r.fetchVaultCRD(ctx, req)
	if err != nil {
		logger.Info("Failed to fetch Vault CRD")
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling Vault")
	logger.Info("Current Vault being reconciled", "name", u.GetName(), "namespace", u.GetNamespace(), "uid", u.GetUID())
	logger.Info("Processing Vault item", "name", u.GetName(), "namespace", u.GetNamespace(), "uid", u.GetUID())

	clientset, dynamicClient, err := r.getKubeClient(ctx, u)
	if err != nil {
		return ctrl.Result{}, err
	}

	pod, err := clientset.CoreV1().Pods(u.GetNamespace()).Get(ctx, u.GetName()+"-0", metav1.GetOptions{})
	if err != nil {
		logger.Info("Vault pod not present, requeing reconcile after 10 seconds")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger.Info("Creating VaultData")
	logger.Info("Getting Vault Metrics")

	cpuUsagePercentStr, memoryUsagePercentStr, err := r.getVaultMetrics(ctx, dynamicClient, pod)
	if err != nil {
		logger.Info("Failed to get Vault metrics, requeing reconcile after 30 seconds")
		logger.Info("Is the metrics server running?")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	deployment, err := clientset.AppsV1().Deployments(u.GetNamespace()).Get(ctx, u.GetName()+"-configurer", metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to get deployment, requeing reconcile after 10 seconds")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	err = r.createOrUpdateVaultMonCRD(clientset, deployment, ctx, req, u, pod, cpuUsagePercentStr, memoryUsagePercentStr)
	if err != nil {
		logger.Error(err, "Failed to create or update VaultMon CRD")
		return ctrl.Result{}, err
	}

	err = r.manageFinalizers(ctx, u)

	// annotations := u.GetAnnotations()
	// logger.Info("Vault Annotations: " + fmt.Sprintf("%v", annotations))

	// nextRun := time.Now().Add(10 * time.Second)
	// return ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}, nil

	return ctrl.Result{}, nil
}

// a function to fetch the Vault CRD.
func (r *VaultMonReconciler) fetchVaultCRD(ctx context.Context, req ctrl.Request) (*unstructured.Unstructured, error) {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})
	if err := r.Get(ctx, req.NamespacedName, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// a function that gets the config, clientset, and dynamicClient
func (r *VaultMonReconciler) getKubeClient(ctx context.Context, vault *unstructured.Unstructured) (*kubernetes.Clientset, dynamic.Interface, error) {
	logger := ctrl.Log
	runOutsideClusterStr := os.Getenv("RUN_OUTSIDE_CLUSTER")
	runOutsideCluster, err := strconv.ParseBool(runOutsideClusterStr)
	if err != nil {
		// logger.Info("Failed to parse RUN_OUTSIDE_CLUSTER")
		// return ctrl.Result{}, err
	}

	runOutsideCluster = true

	var config *rest.Config

	if runOutsideCluster {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error(err, "Failed to create dynamic client")
		return nil, nil, err
	}

	return clientset, dynamicClient, nil
}

// a function that gets the vaultMetrics
func (r *VaultMonReconciler) getVaultMetrics(ctx context.Context, dynamicClient dynamic.Interface, pod *corev1.Pod) (string, string, error) {
	logger := ctrl.Log
	podMetricsGVR := schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}

	//fetch pod metrics
	podMetrics, err := dynamicClient.Resource(podMetricsGVR).Namespace("default").Get(ctx, "vault-0", metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to fetch pod metrics, requeing reconcile after 10 seconds")
		return "", "", nil
	}
	containerMetrics := podMetrics.Object["containers"].([]interface{})[0].(map[string]interface{})
	usage := containerMetrics["usage"].(map[string]interface{})
	memoryUsageStr := usage["memory"].(string)
	cpuUsageStr := usage["cpu"].(string)

	memoryUsage, err := resource.ParseQuantity(memoryUsageStr)
	if err != nil {
		logger.Error(err, "Failed to parse memory usage")
		return "", "", err
	}

	cpuUsage, err := resource.ParseQuantity(cpuUsageStr)
	if err != nil {
		logger.Error(err, "Failed to parse CPU usage")
		return "", "", err
	}

	cpuLimit := pod.Spec.Containers[0].Resources.Limits.Cpu()
	memoryLimit := pod.Spec.Containers[0].Resources.Limits.Memory()

	cpuLimitConv := cpuLimit.AsApproximateFloat64()
	memoryLimitConv := memoryLimit.AsApproximateFloat64()

	cpuUsageConv := cpuUsage.AsApproximateFloat64()
	memoryUsageConv := memoryUsage.AsApproximateFloat64()

	cpuUsagePercent := (cpuUsageConv / cpuLimitConv) * 100
	memoryUsagePercent := (memoryUsageConv / memoryLimitConv) * 100

	cpuUsagePercentStr := strconv.FormatFloat(cpuUsagePercent, 'f', 2, 64)
	memoryUsagePercentStr := strconv.FormatFloat(memoryUsagePercent, 'f', 2, 64)

	return cpuUsagePercentStr, memoryUsagePercentStr, nil
}

// a function that creates or updates the VaultMon CRD
func (r *VaultMonReconciler) createOrUpdateVaultMonCRD(clientset *kubernetes.Clientset, deployment *appsv1.Deployment, ctx context.Context, req ctrl.Request, u *unstructured.Unstructured, pod *corev1.Pod, cpuUsagePercentStr string, memoryUsagePercentStr string) error {
	logger := ctrl.Log
	image, err := clientset.CoreV1().Pods("default").Get(ctx, "vault-0", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	// logger.Info("VAULT_IMAGE=" + image.Spec.Containers[0].Image)

	ingressList := &v1.IngressList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": "vault"})
	if err := r.Client.List(ctx, ingressList, client.InNamespace("default"), client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		logger.Error(err, "Failed to list Ingress resources")
		return err
	}

	var vaultIngress *v1.Ingress
	if len(ingressList.Items) > 0 {
		vaultIngress = &ingressList.Items[0]
	} else {
		logger.Info("No Ingress found for the Vault instance")
		vaultIngress = nil
	}

	var vaultEndpoints []string
	if vaultIngress != nil {
		for _, rule := range vaultIngress.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				vaultEndpoint := fmt.Sprintf("%s%s", rule.Host, path.Path)
				vaultEndpoints = append(vaultEndpoints, vaultEndpoint)
			}
		}
	} else {
		logger.Info("No Ingress found for the Vault instance")
		vaultEndpoints = nil
	}

	serviceList := &corev1.ServiceList{}
	svcLabelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": "vault"})
	if err := r.Client.List(ctx, serviceList, client.InNamespace("default"), client.MatchingLabelsSelector{Selector: svcLabelSelector}); err != nil {
		logger.Error(err, "Failed to list Service resources")
		return err
	}

	var loadBalancerEndpoints []string
	for _, svc := range serviceList.Items {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, lbIP := range svc.Status.LoadBalancer.Ingress {
				for _, port := range svc.Spec.Ports {
					loadBalancerEndpoint := fmt.Sprintf("%s:%d", lbIP.IP, port.Port)
					loadBalancerEndpoints = append(loadBalancerEndpoints, loadBalancerEndpoint)
				}
			}
		}
	}

	vaultData := &rossoperatoriov1alpha1.VaultMon{}
	uniqueID := u.GetUID()
	vaultMonName := fmt.Sprintf("%s-%s", "vaultmon", uniqueID)
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: vaultMonName}, vaultData)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating new VaultMon for Vault item", "name", u.GetName(), "namespace", u.GetNamespace(), "uid", u.GetUID())
			uniqueID := u.GetUID()
			vaultMonName := fmt.Sprintf("%s-%s", "vaultmon", uniqueID)
			vaultData = &rossoperatoriov1alpha1.VaultMon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vaultMonName,
					Namespace: req.Namespace,
				},
				Spec: rossoperatoriov1alpha1.VaultMonSpec{
					VaultName: u.GetName(),

					VaultNamespace: u.GetNamespace(),

					VaultUid: string(u.GetUID()),

					VaultIp: pod.Status.PodIP,

					VaultStatus: pod.Status.ContainerStatuses,

					VaultMemUsage: memoryUsagePercentStr,

					VaultCPUUsage: cpuUsagePercentStr,

					VaultReplicas: deployment.Status.Replicas,

					VaultImage: deployment.Spec.Template.Spec.Containers[0].Image,

					VaultIngress: func() string {
						if vaultIngress != nil && len(vaultIngress.Spec.Rules) > 0 {
							return vaultIngress.Spec.Rules[0].Host
						}
						return "None"
					}(),

					VaultVolumes: pod.Spec.Volumes,

					VaultEndpoints: vaultEndpoints,
				},
			}
			if err := r.Client.Create(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to create VaultData")
				return err
			}
			logger.Info("VaultData created")
			logSimplified("VAULT_NAME=" + vaultData.Spec.VaultName)
			logSimplified("VAULT_NAMESPACE=" + vaultData.Spec.VaultNamespace)
			logSimplified("VAULT_POD_IP=" + vaultData.Spec.VaultIp)
			logSimplified("VAULT_UID=" + vaultData.Spec.VaultUid)
			for _, status := range vaultData.Spec.VaultStatus {
				logSimplified("CONTAINER_NAME=" + status.Name)
				logSimplified("CONTAINER_READY=" + fmt.Sprintf("%t", status.Ready))
				logSimplified("CONTAINER_LIVENESS=" + fmt.Sprintf("%t", status.State.Running != nil))
			}
			logSimplified("VAULT_MEMORY_USAGE=" + vaultData.Spec.VaultMemUsage)
			logSimplified("VAULT_CPU_USAGE=" + vaultData.Spec.VaultCPUUsage)
			// logSimplified("VAULT_REPLICAS=" + string(vaultData.Spec.VaultReplicas))
			logSimplified("VAULT_REPLICAS=3")
			logSimplified("VAULT_IMAGE=" + vaultData.Spec.VaultImage)
			logSimplified("VAULT_IMAGE=" + image.Spec.Containers[0].Image)
			logSimplified("VAULT_INGRESS=" + vaultData.Spec.VaultIngress)
			for _, volume := range vaultData.Spec.VaultVolumes {
				logSimplified("VOLUME_NAME=" + volume.Name)
			}
			if vaultData.Spec.VaultEndpoints != nil {
				for _, endpoint := range vaultData.Spec.VaultEndpoints {
					logSimplified("VAULT_ENDPOINT=" + endpoint)
				}
			} else {
				logSimplified("VAULT_ENDPOINTS=None")
			}

			timestamp := time.Now().Format(time.RFC3339)

			r.VaultMetrics.VaultInfo.With(prometheus.Labels{
				"vaultName":      vaultData.Spec.VaultName,
				"vaultUid":       vaultData.Spec.VaultUid,
				"vaultNamespace": vaultData.Spec.VaultNamespace,
				"vaultIp":        vaultData.Spec.VaultIp,
				"vaultImage":     vaultData.Spec.VaultImage,
				"timestamp":      timestamp,
			}).Set(1)
			logger.Info("Vault metrics set")

		} else {
			logger.Error(err, "Failed to get VaultData")
			return err
		}
	}

	if vaultData.Spec.VaultName != u.GetName() {
		vaultData.Spec.VaultName = u.GetName()
		timestamp := time.Now().Format(time.RFC3339)
		r.VaultMetrics.VaultInfo.With(prometheus.Labels{
			"vaultName":      vaultData.Spec.VaultName,
			"vaultUid":       vaultData.Spec.VaultUid,
			"vaultNamespace": vaultData.Spec.VaultNamespace,
			"vaultIp":        vaultData.Spec.VaultIp,
			"vaultImage":     vaultData.Spec.VaultImage,
			"timestamp":      timestamp,
		}).Set(1)
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData updated", "vaultMonName", vaultData.Name)
		logger.Info("VaultData Name updated to " + vaultData.Spec.VaultName)
	}

	if vaultData.Spec.VaultNamespace != u.GetNamespace() {
		vaultData.Spec.VaultNamespace = u.GetNamespace()
		timestamp := time.Now().Format(time.RFC3339)
		r.VaultMetrics.VaultInfo.With(prometheus.Labels{
			"vaultName":      vaultData.Spec.VaultName,
			"vaultUid":       vaultData.Spec.VaultUid,
			"vaultNamespace": vaultData.Spec.VaultNamespace,
			"vaultIp":        vaultData.Spec.VaultIp,
			"vaultImage":     vaultData.Spec.VaultImage,
			"timestamp":      timestamp,
		}).Set(1)
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Namespace updated to " + vaultData.Spec.VaultNamespace)
	}

	if vaultData.Spec.VaultUid != string(u.GetUID()) {
		vaultData.Spec.VaultUid = string(u.GetUID())
		timestamp := time.Now().Format(time.RFC3339)
		r.VaultMetrics.VaultInfo.With(prometheus.Labels{
			"vaultName":      vaultData.Spec.VaultName,
			"vaultUid":       vaultData.Spec.VaultUid,
			"vaultNamespace": vaultData.Spec.VaultNamespace,
			"vaultIp":        vaultData.Spec.VaultIp,
			"vaultImage":     vaultData.Spec.VaultImage,
			"timestamp":      timestamp,
		}).Set(1)
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Uid updated to " + vaultData.Spec.VaultUid)
	}

	if vaultData.Spec.VaultIp != pod.Status.PodIP {
		vaultData.Spec.VaultIp = pod.Status.PodIP
		timestamp := time.Now().Format(time.RFC3339)
		r.VaultMetrics.VaultInfo.With(prometheus.Labels{
			"vaultName":      vaultData.Spec.VaultName,
			"vaultUid":       vaultData.Spec.VaultUid,
			"vaultNamespace": vaultData.Spec.VaultNamespace,
			"vaultIp":        vaultData.Spec.VaultIp,
			"vaultImage":     vaultData.Spec.VaultImage,
			"timestamp":      timestamp,
		}).Set(1)
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData PodIp updated to " + vaultData.Spec.VaultIp)
	}

	for _, status := range vaultData.Spec.VaultStatus {
		for _, podStatus := range pod.Status.ContainerStatuses {
			if status.Name == podStatus.Name {
				if status.Ready != podStatus.Ready {
					status.Ready = podStatus.Ready
					if err := r.Client.Update(ctx, vaultData); err != nil {
						logger.Error(err, "Failed to update VaultData")
						return err
					}
					logger.Info("VaultData Container Ready updated to " + fmt.Sprintf("%t", status.Ready))
				}

				previousLiveness := status.State.Running != nil
				currentLiveness := podStatus.State.Running != nil
				if previousLiveness != currentLiveness {
					logger.Info("VaultData Container Liveness updated from " + fmt.Sprintf("%t", previousLiveness) + " to " + fmt.Sprintf("%t", currentLiveness))
					status.State.Running = podStatus.State.Running
					if err := r.Client.Update(ctx, vaultData); err != nil {
						logger.Error(err, "Failed to update VaultData")
						return err
					}
				}
			}
		}
	}

	if vaultData.Spec.VaultMemUsage != memoryUsagePercentStr {
		vaultData.Spec.VaultMemUsage = memoryUsagePercentStr
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Memory Usage updated to " + vaultData.Spec.VaultMemUsage)
	}

	if vaultData.Spec.VaultCPUUsage != cpuUsagePercentStr {
		vaultData.Spec.VaultCPUUsage = cpuUsagePercentStr
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData CPU Usage updated to " + vaultData.Spec.VaultCPUUsage)
	}

	if vaultData.Spec.VaultReplicas != deployment.Status.Replicas {
		vaultData.Spec.VaultReplicas = deployment.Status.Replicas
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Replicas updated to " + string(vaultData.Spec.VaultReplicas))
	}

	if vaultData.Spec.VaultImage != deployment.Spec.Template.Spec.Containers[0].Image {
		vaultData.Spec.VaultImage = deployment.Spec.Template.Spec.Containers[0].Image
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Image updated to " + vaultData.Spec.VaultImage)
	}

	return nil
}

// manages the finalizers for the Vault CRD.
func (r *VaultMonReconciler) manageFinalizers(ctx context.Context, u *unstructured.Unstructured) error {
	logger := ctrl.Log
	vaultFinalizer := "vault.banzaicloud.com/finalizer2"

	logger.Info("Vault finalizers" + fmt.Sprintf("%v", u.GetFinalizers()))

	if !containsString(u.GetFinalizers(), vaultFinalizer) {
		u.SetFinalizers(append(u.GetFinalizers(), vaultFinalizer))
		if err := r.Update(ctx, u); err != nil {
			logger.Error(err, "Failed to add finalizer to Vault CRD")
			return err
		}
		logger.Info("Added finalizer to Vault CRD")
	}

	if !u.GetDeletionTimestamp().IsZero() {
		if containsString(u.GetFinalizers(), vaultFinalizer) {
			u.SetFinalizers(removeString(u.GetFinalizers(), vaultFinalizer))
			if err := r.Update(ctx, u); err != nil {
				logger.Error(err, "Failed to remove finalizer from Vault CRD")
				return err
			}
			logger.Info("Vault CRD has been destroyed!")

			// delete the VaultMon CRD
			uniqueID := u.GetUID()
			vaultMonName := fmt.Sprintf("%s-%s", "vaultmon", uniqueID)
			vaultData := &rossoperatoriov1alpha1.VaultMon{}
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: u.GetNamespace(), Name: vaultMonName}, vaultData)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info("VaultMon CRD not found, no need to delete")
				} else {
					logger.Error(err, "Failed to get VaultMon for deletion")
					return err
				}
			} else {
				if err := r.Client.Delete(ctx, vaultData); err != nil {
					logger.Error(err, "Failed to delete VaultMon CRD")
					return err
				}
				logger.Info("VaultMon CRD has been deleted!")
			}
		}
		return nil
	}

	return nil
}

func logSimplified(message string, keyValuePairs ...interface{}) {
	logger := ctrl.Log

	keys := make([]interface{}, 0, len(keyValuePairs)/2)
	values := make([]interface{}, 0, len(keyValuePairs)/2)

	for i := 0; i < len(keyValuePairs); i += 2 {
		keys = append(keys, keyValuePairs[i])
		values = append(values, keyValuePairs[i+1])
	}

	msg := message
	for i, key := range keys {
		msg = fmt.Sprintf("%s %s=%v", msg, key, values[i])
	}

	logger.Info(msg)
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
	//wait for the Vault CRD to be created if it hasnt been already
	for {
		u := unstructured.Unstructured{Object: map[string]interface{}{}}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    "Vault",
			Group:   "vault.banzaicloud.com",
			Version: "v1alpha1",
		})
		err := mgr.GetClient().Get(context.Background(), client.ObjectKey{
			Name:      "vault",
			Namespace: "default",
		}, &u)
		if err != nil {
			logSimplified("Vault CRD not found, waiting for it to be created")
			time.Sleep(5 * time.Second)
		} else {
			logSimplified("Vault CRD found")
			break
		}
	}

	u := unstructured.Unstructured{Object: map[string]interface{}{}}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Vault",
		Group:   "vault.banzaicloud.com",
		Version: "v1alpha1",
	})

	return ctrl.NewControllerManagedBy(mgr).For(&u).Complete(NewVaultReconciler(mgr.GetClient(), mgr.GetScheme(), r.VaultMetrics))
}
