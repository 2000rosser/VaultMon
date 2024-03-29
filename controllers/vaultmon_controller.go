package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

	//get the vault client
	clientset, dynamicClient, err := r.getKubeClient(ctx, u)
	if err != nil {
		return ctrl.Result{}, err
	}

	//get the vault pod
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": u.GetName()})
	if err := r.Client.List(ctx, podList, client.InNamespace(u.GetNamespace()), client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		logger.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	var vaultPodName string
	for _, p := range podList.Items {
		vaultPodName = p.Name
		break
	}

	//check if the vault pod is present
	if vaultPodName == "" {
		logger.Info("Vault pod not present, requeing reconcile after 10 seconds")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	pod, err := clientset.CoreV1().Pods(u.GetNamespace()).Get(ctx, vaultPodName, metav1.GetOptions{})
	if err != nil {
		logger.Info("Vault pod not present, requeing reconcile after 10 seconds")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger.Info("Creating VaultData")
	logger.Info("Getting Vault Metrics")

	//get the vault metrics
	cpuUsagePercentStr, memoryUsagePercentStr, err := r.getVaultMetrics(ctx, dynamicClient, pod, u)
	if err != nil {
		logger.Info("Failed to get Vault metrics, requeing reconcile after 30 seconds")
		logger.Info("Is the metrics server running?")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	//get the deployment
	deployment, err := clientset.AppsV1().Deployments(u.GetNamespace()).Get(ctx, u.GetName()+"-configurer", metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to get deployment, requeing reconcile after 10 seconds")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	//create or update the VaultMon CRD
	err = r.createOrUpdateVaultMonCRD(clientset, deployment, ctx, req, u, pod, cpuUsagePercentStr, memoryUsagePercentStr)
	if err != nil {
		logger.Error(err, "Failed to create or update VaultMon CRD")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	//add/remove the finalizers
	err = r.manageFinalizers(ctx, u)
	if err != nil {
		logger.Error(err, "Failed to manage finalizers")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	//requeue the reconcile after 10 seconds
	nextRun := time.Now().Add(10 * time.Second)
	return ctrl.Result{RequeueAfter: nextRun.Sub(time.Now())}, nil

	// return ctrl.Result{}, nil
}

// a function to fetch the Vault CRD.
func (r *VaultMonReconciler) fetchVaultCRD(ctx context.Context, req ctrl.Request) (*unstructured.Unstructured, error) {
	//get the vault crd
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

	//set runOutsideCluster to true for now
	runOutsideCluster = true

	var config *rest.Config

	//get the config file if running outside the cluster
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

	//create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	//create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error(err, "Failed to create dynamic client")
		return nil, nil, err
	}

	return clientset, dynamicClient, nil
}

// a function that gets the vaultMetrics
func (r *VaultMonReconciler) getVaultMetrics(ctx context.Context, dynamicClient dynamic.Interface, pod *corev1.Pod, u *unstructured.Unstructured) (string, string, error) {
	logger := ctrl.Log

	//get the pod metrics from the metrics server
	podMetricsGVR := schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}

	//fetch pod metrics for vault pod
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": u.GetName()})
	if err := r.Client.List(ctx, podList, client.InNamespace(u.GetNamespace()), client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		logger.Error(err, "Failed to list pods")
		return "", "", err
	}

	var vaultPod *corev1.Pod
	for _, p := range podList.Items {
		vaultPod = &p
		break
	}

	if vaultPod == nil {
		return "", "", fmt.Errorf("No Vault pod found")
	}

	podMetrics, err := dynamicClient.Resource(podMetricsGVR).Namespace(u.GetNamespace()).Get(ctx, vaultPod.Name, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to fetch pod metrics, requeing reconcile after 10 seconds")
		return "", "", nil
	}

	//get the memory and cpu usage
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

	//convert the memory and cpu usage to percentages
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

	//get the image
	image, err := clientset.CoreV1().Pods(u.GetNamespace()).Get(ctx, u.GetName()+"-0", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	// logger.Info("VAULT_IMAGE=" + image.Spec.Containers[0].Image)

	//retrieve the ingress of vault
	ingressList := &v1.IngressList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": u.GetName()})
	if err := r.Client.List(ctx, ingressList, client.InNamespace(u.GetNamespace()), client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		logger.Error(err, "Failed to list Ingress resources")
		return err
	}

	var vaultIngress *v1.Ingress

	//store the ingress in a vaultIngress variable
	if len(ingressList.Items) > 0 {
		vault2Ingress := &ingressList.Items[0]
		vaultIngress = &v1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vault2Ingress.Name,
				Namespace: vault2Ingress.Namespace,
			},
			Spec: v1.IngressSpec{
				DefaultBackend: vault2Ingress.Spec.DefaultBackend,
			},
		}
	} else {
		logger.Info("No Ingress found for the Vault instance")
		vaultIngress = nil
	}

	//retrieve the endpoints of vault
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

	//retrieve the load balancer endpoints of vault
	serviceList := &corev1.ServiceList{}
	svcLabelSelector := labels.SelectorFromSet(map[string]string{"vault_cr": u.GetName()})
	if err := r.Client.List(ctx, serviceList, client.InNamespace("default"), client.MatchingLabelsSelector{Selector: svcLabelSelector}); err != nil {
		logger.Error(err, "Failed to list Service resources")
		return err
	}

	//store the load balancer endpoints in a loadBalancerEndpoints variable
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

	//create a new VaultMon CRD
	vaultData := &rossoperatoriov1alpha1.VaultMon{}
	uniqueID := u.GetUID()
	vaultMonName := fmt.Sprintf("%s-%s", "vaultmon", uniqueID)
	//check if the VaultMon CRD already exists
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: vaultMonName}, vaultData)
	if err != nil {
		//if the VaultMon CRD does not exist, create a new one
		if errors.IsNotFound(err) {
			logger.Info("Creating new VaultMon for Vault item", "name", u.GetName(), "namespace", u.GetNamespace(), "uid", u.GetUID())
			uniqueID := u.GetUID()
			vaultMonName := fmt.Sprintf("%s-%s", "vaultmon", uniqueID)
			//use the UID of the Vault item as the name of the VaultMon CRD and the namespace of the Vault item as the namespace of the VaultMon CRD
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

					VaultIngress: vaultIngress,

					VaultVolumes: pod.Spec.Volumes,

					VaultEndpoints: vaultEndpoints,

					VaultVersion: image.Spec.Containers[0].Image,

					VaultLabels: deployment.Labels,

					VaultAnnotations: deployment.Spec.Template.Annotations,
				},
			}
			if err := r.Client.Create(ctx, vaultData); err != nil {
				logger.Error(err, "Failed to create VaultData")
				return err
			}
			//log all the information about the VaultMon CRD
			logger.Info("VaultData created")
			logSimplified("VAULT_NAME=" + vaultData.Spec.VaultName)
			logSimplified("VAULT_NAMESPACE=" + vaultData.Spec.VaultNamespace)
			logSimplified("VAULT_POD_IP=" + vaultData.Spec.VaultIp)
			logSimplified("VAULT_UID=" + vaultData.Spec.VaultUid)
			for _, status := range vaultData.Spec.VaultStatus {
				logSimplified("CONTAINER_NAME=" + status.Name)
				logSimplified("CONTAINER_READY=" + fmt.Sprintf("%t", status.Ready))
				logSimplified("CONTAINER_LIVENESS=" + fmt.Sprintf("%t", status.State.Running != nil))
				logSimplified("CONTAINER_RESTARTS=" + fmt.Sprintf("%d", status.RestartCount))
				logSimplified("CONTAINER_TERMINATION=" + fmt.Sprintf("%t", status.State.Terminated != nil))
				logSimplified("CONTAINER_WAITING=" + fmt.Sprintf("%t", status.State.Waiting != nil))
			}
			logSimplified("VAULT_MEMORY_USAGE=" + vaultData.Spec.VaultMemUsage)
			logSimplified("VAULT_CPU_USAGE=" + vaultData.Spec.VaultCPUUsage)
			logSimplified("VAULT_REPLICAS=" + strconv.Itoa(int(vaultData.Spec.VaultReplicas)))
			logSimplified("VAULT_IMAGE=" + vaultData.Spec.VaultImage)
			logSimplified("VAULT_VERSION=" + vaultData.Spec.VaultVersion)
			if vaultData.Spec.VaultIngress.Spec.DefaultBackend != nil {
				serviceName := vaultData.Spec.VaultIngress.Spec.DefaultBackend.Service.Name
				servicePort := vaultData.Spec.VaultIngress.Spec.DefaultBackend.Service.Port.Number

				logSimplified("VAULT_INGRESS_NAME=" + serviceName)
				logSimplified("VAULT_INGRESS_SERVICE_PORT=" + strconv.Itoa(int(servicePort)))
			}

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

			for key, value := range vaultData.Spec.VaultLabels {
				logSimplified("VAULT_LABELS_" + key + "=" + value)
			}
			for key, value := range vaultData.Spec.VaultAnnotations {
				//check if the value is over 100 characters
				if len(value) > 100 {
					value = value[:100] + "..."
				}
				logSimplified("VAULT_ANNOTATIONS_" + key + "=" + value)
			}
			r.updatePrometheusLabels(vaultData)
			logger.Info("Vault metrics set")

		} else {
			logger.Error(err, "Failed to get VaultData")
			return err
		}
	}

	//check for differences between the VaultMon CRD and the Vault item for each value
	if vaultData.Spec.VaultName != u.GetName() {
		vaultData.Spec.VaultName = u.GetName()
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData updated", "vaultMonName", vaultData.Name)
		logger.Info("VaultData Name updated to " + vaultData.Spec.VaultName)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultNamespace != u.GetNamespace() {
		vaultData.Spec.VaultNamespace = u.GetNamespace()
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Namespace updated to " + vaultData.Spec.VaultNamespace)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultUid != string(u.GetUID()) {
		vaultData.Spec.VaultUid = string(u.GetUID())
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData Uid updated to " + vaultData.Spec.VaultUid)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultIp != pod.Status.PodIP {
		vaultData.Spec.VaultIp = pod.Status.PodIP
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logger.Info("VaultData PodIp updated to " + vaultData.Spec.VaultIp)
		r.updatePrometheusLabels(vaultData)
	}

	vaultStatusMap := make(map[string]corev1.ContainerStatus)
	for _, status := range vaultData.Spec.VaultStatus {
		vaultStatusMap[status.Name] = status
	}

	podStatusMap := make(map[string]corev1.ContainerStatus)
	for _, status := range pod.Status.ContainerStatuses {
		podStatusMap[status.Name] = status
	}

	//compare the two maps
	if !reflect.DeepEqual(vaultStatusMap, podStatusMap) {
		logSimplified("Container statuses have changed")

		//check if a container has been removed
		if len(vaultStatusMap) > len(podStatusMap) {
			for name, status := range vaultStatusMap {
				if _, ok := podStatusMap[name]; !ok {
					logSimplified("Container removed", "Name", name, "Ready", status.Ready, "Running", status.State.Running != nil)
				}
			}
		}

		for name, status := range podStatusMap {
			if oldStatus, ok := vaultStatusMap[name]; ok {
				// check if any status attribute has changed
				if oldStatus.Ready != status.Ready ||
					(oldStatus.State.Running != nil) != (status.State.Running != nil) ||
					(oldStatus.State.Terminated != nil) != (status.State.Terminated != nil) ||
					(oldStatus.State.Waiting != nil) != (status.State.Waiting != nil) {

					// log old and new status if any status attribute has changed
					logSimplified("Old Status", "Name", name, "Ready", oldStatus.Ready, "Running", oldStatus.State.Running != nil, "Terminated", oldStatus.State.Terminated != nil, "Waiting", oldStatus.State.Waiting != nil)
					logSimplified("New Status", "Name", name, "Ready", status.Ready, "Running", status.State.Running != nil, "Terminated", status.State.Terminated != nil, "Waiting", status.State.Waiting != nil)

					if oldStatus.Ready && !status.Ready {
						logSimplified("Health Deterioration: Container not ready", "Name", name)
					} else if !oldStatus.Ready && status.Ready {
						logSimplified("Health Improvement: Container ready", "Name", name)
					}

					if oldStatus.State.Running != nil && status.State.Running == nil {
						logSimplified("Health Deterioration: Container not running", "Name", name)
					} else if oldStatus.State.Running == nil && status.State.Running != nil {
						logSimplified("Health Improvement: Container running", "Name", name)
					}

					if oldStatus.State.Terminated == nil && status.State.Terminated != nil {
						logSimplified("Health Deterioration: Container not terminated", "Name", name)
					}

					if oldStatus.State.Waiting != nil && status.State.Waiting == nil {
						logSimplified("Health Improvement: Container no longer waiting", "Name", name)
					} else if oldStatus.State.Waiting == nil && status.State.Waiting != nil {
						logSimplified("Health Deterioration: Container waiting", "Name", name)
					}

					if oldStatus.RestartCount != status.RestartCount {
						logSimplified("Container restart count changed", "Container Name", name, "New Restart Count", int(status.RestartCount))
					}

				}
			} else {
				logSimplified("Container added", "Name", name)
			}
		}

		vaultData.Spec.VaultStatus = pod.Status.ContainerStatuses

		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Container Statuses updated")
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultMemUsage != memoryUsagePercentStr {
		memoryUsageThreshold := 80
		prevMemUsage, _ := strconv.ParseFloat(vaultData.Spec.VaultMemUsage, 64)
		currentMemUsage, _ := strconv.ParseFloat(memoryUsagePercentStr, 64)

		if prevMemUsage > float64(memoryUsageThreshold) && currentMemUsage < float64(memoryUsageThreshold) {
			logSimplified("Health Improvement: Memory usage is below threshold", "Current Usage", memoryUsagePercentStr)
		}

		vaultData.Spec.VaultMemUsage = memoryUsagePercentStr

		memoryUsageFloat, err := strconv.ParseFloat(memoryUsagePercentStr, 64)
		if err != nil {
			logger.Error(err, "Failed to convert memoryUsagePercentStr to float")
			return err
		}

		memoryUsagePercent := int(memoryUsageFloat)

		if memoryUsagePercent > memoryUsageThreshold {
			logSimplified("Health Deterioration: Memory usage exceeded threshold", "Current Usage", memoryUsagePercentStr)
		}
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Memory Usage updated to " + vaultData.Spec.VaultMemUsage)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultCPUUsage != cpuUsagePercentStr {

		cpuUsageThreshold := 80
		prevCPUUsage, _ := strconv.ParseFloat(vaultData.Spec.VaultCPUUsage, 64)
		currentCPUUsage, _ := strconv.ParseFloat(cpuUsagePercentStr, 64)

		if prevCPUUsage > float64(cpuUsageThreshold) && currentCPUUsage < float64(cpuUsageThreshold) {
			logSimplified("Health Improvement: CPU usage is below threshold", "Current Usage", cpuUsagePercentStr)
		}

		vaultData.Spec.VaultCPUUsage = cpuUsagePercentStr

		cpuUsageFloat, err := strconv.ParseFloat(cpuUsagePercentStr, 64)
		if err != nil {
			logger.Error(err, "Failed to convert cpuUsagePercentStr to float")
			return err
		}

		cpuUsagePercent := int(cpuUsageFloat)

		if cpuUsagePercent > cpuUsageThreshold {
			logSimplified("Health Deterioration: CPU usage exceeded threshold", "Current Usage", cpuUsagePercentStr)
		}

		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData CPU Usage updated to " + vaultData.Spec.VaultCPUUsage)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultReplicas != deployment.Status.Replicas {
		vaultData.Spec.VaultReplicas = deployment.Status.Replicas
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Replicas updated to " + string(vaultData.Spec.VaultReplicas))
		r.updatePrometheusLabels(vaultData)

	}

	if vaultData.Spec.VaultImage != deployment.Spec.Template.Spec.Containers[0].Image {
		vaultData.Spec.VaultImage = deployment.Spec.Template.Spec.Containers[0].Image
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Image updated to " + vaultData.Spec.VaultImage)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultVersion != image.Spec.Containers[0].Image {
		vaultData.Spec.VaultVersion = image.Spec.Containers[0].Image
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Version updated to " + vaultData.Spec.VaultVersion)
		r.updatePrometheusLabels(vaultData)
	}

	if vaultData.Spec.VaultIngress != vaultIngress {
		vaultData.Spec.VaultIngress = vaultIngress
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}

		r.updatePrometheusLabels(vaultData)
	}

	if !reflect.DeepEqual(vaultData.Spec.VaultLabels, deployment.Labels) {
		//check if labels are added or removed or modified
		for key, value := range vaultData.Spec.VaultLabels {
			if depValue, ok := deployment.Labels[key]; ok {
				if depValue != value {
					logSimplified("Label modified", "Old Label", key+"="+value, "New Label", key+"="+depValue)
				}
			} else {
				logSimplified("Label removed", "Label", key+"="+value)
			}
		}
		for key, value := range deployment.Labels {
			if vaultValue, ok := vaultData.Spec.VaultLabels[key]; !ok {
				logSimplified("Label added", "Label", key+"="+value)
			} else {
				if vaultValue != value {
					logSimplified("Label modified", "Old Label", key+"="+vaultValue, "New Label", key+"="+value)
				}
			}
		}

		vaultData.Spec.VaultLabels = deployment.Labels
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Labels updated to:")
		for key, value := range vaultData.Spec.VaultLabels {
			logSimplified(key + "=" + value)
		}
		r.updatePrometheusLabels(vaultData)
	}

	if !reflect.DeepEqual(vaultData.Spec.VaultAnnotations, deployment.Spec.Template.Annotations) {
		for key, value := range vaultData.Spec.VaultAnnotations {
			if depValue, ok := deployment.Spec.Template.Annotations[key]; ok {
				if depValue != value {
					logSimplified("Annotation modified", "Old Annotation", key+"="+value, "New Annotation", key+"="+depValue)
				}
			} else {
				logSimplified("Annotation removed", "Annotation", key+"="+value)
			}
		}
		for key, value := range deployment.Spec.Template.Annotations {
			if vaultValue, ok := vaultData.Spec.VaultAnnotations[key]; !ok {
				logSimplified("Annotation added", "Annotation", key+"="+value)
			} else {
				if vaultValue != value {
					logSimplified("Annotation modified", "Old Annotation", key+"="+vaultValue, "New Annotation", key+"="+value)
				}
			}
		}

		vaultData.Spec.VaultAnnotations = deployment.Spec.Template.Annotations
		if err := r.Client.Update(ctx, vaultData); err != nil {
			logger.Error(err, "Failed to update VaultData")
			return err
		}
		logSimplified("VaultData Annotations updated to:")
		for key, value := range vaultData.Spec.VaultAnnotations {
			logSimplified(key + "=" + value)
		}
		r.updatePrometheusLabels(vaultData)
	}

	return nil
}

func (r *VaultMonReconciler) updatePrometheusLabels(vaultData *rossoperatoriov1alpha1.VaultMon) {

	//Update Prometheus Labels
	vaultStatuses := make([]string, len(vaultData.Spec.VaultStatus))
	for i, status := range vaultData.Spec.VaultStatus {
		vaultStatuses[i] = fmt.Sprintf("ContainerName=%s,Ready=%t,Liveness=%t", status.Name, status.Ready, status.State.Running != nil)
	}
	serviceName := ""
	servicePort := 0
	if vaultData.Spec.VaultIngress.Spec.DefaultBackend != nil {
		serviceName = vaultData.Spec.VaultIngress.Spec.DefaultBackend.Service.Name
		servicePort = int(vaultData.Spec.VaultIngress.Spec.DefaultBackend.Service.Port.Number)
	}

	vaultStatusStr := strings.Join(vaultStatuses, ";")

	timestamp := time.Now().Format(time.RFC3339)
	r.VaultMetrics.VaultInfo.With(prometheus.Labels{
		"vaultName":      vaultData.Spec.VaultName,
		"vaultUid":       vaultData.Spec.VaultUid,
		"vaultNamespace": vaultData.Spec.VaultNamespace,
		"vaultIp":        vaultData.Spec.VaultIp,
		"vaultImage":     vaultData.Spec.VaultImage,
		"vaultStatus":    vaultStatusStr,
		"vaultReplicas":  strconv.Itoa(int(vaultData.Spec.VaultReplicas)),
		"vaultIngress":   serviceName + ":" + strconv.Itoa(int(servicePort)),
		"vaultVolumes":   volumeSliceToString(vaultData.Spec.VaultVolumes),
		"vaultEndpoints": fmt.Sprintf("%v", vaultData.Spec.VaultEndpoints),
		"timestamp":      timestamp,
	}).Set(1)

	vaultCPU, err := strconv.ParseFloat(vaultData.Spec.VaultCPUUsage, 64)
	if err != nil {
		logSimplified("Error parsing VaultCPUUsage")
	}

	vaultMem, err := strconv.ParseFloat(vaultData.Spec.VaultMemUsage, 64)
	if err != nil {
		logSimplified("Error parsing VaultMemUsage")
	}

	r.VaultMetrics.VaultCPUUsage.With(prometheus.Labels{
		"vaultName":      vaultData.Spec.VaultName,
		"vaultNamespace": vaultData.Spec.VaultNamespace,
	}).Set(vaultCPU)

	r.VaultMetrics.VaultMemUsage.With(prometheus.Labels{
		"vaultName":      vaultData.Spec.VaultName,
		"vaultNamespace": vaultData.Spec.VaultNamespace,
	}).Set(vaultMem)

}

// converts a slice of corev1.Volume to a string
func volumeSliceToString(volumes []corev1.Volume) string {
	volumeStrings := make([]string, len(volumes))
	for i, v := range volumes {
		volumeStrings[i] = fmt.Sprintf("Name=%s", v.Name)
	}
	return strings.Join(volumeStrings, ";")
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

// simplifies the logging of key value pairs
func logSimplified(message string, keyValuePairs ...interface{}) {
	logger := ctrl.Log

	if len(keyValuePairs) == 0 {
		logger.Info(message)
		return
	}

	keys := make([]interface{}, 0, len(keyValuePairs)/2)
	values := make([]interface{}, 0, len(keyValuePairs)/2)

	for i := 0; i < len(keyValuePairs); i += 2 {
		keys = append(keys, keyValuePairs[i])
		if i+1 < len(keyValuePairs) {
			values = append(values, keyValuePairs[i+1])
		} else {
			values = append(values, nil)
		}
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
