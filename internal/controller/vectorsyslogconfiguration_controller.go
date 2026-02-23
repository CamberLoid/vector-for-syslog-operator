/*
Copyright 2026.

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

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vectorsyslogv1alpha1 "lab.camber.moe/VectorSyslogOperator/api/v1alpha1"
)

const (
	vectorFinalizer = "vectorsyslog.lab.camber.moe/finalizer"

	TypeAvailable   = "Available"
	TypeProgressing = "Progressing"
	TypeDegraded    = "Degraded"

	SourcesPlaceholder = "$$VectorSyslogOperatorSources$$"
)

type VectorSyslogConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vectorsyslog.lab.camber.moe,resources=vectorsyslogconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vectorsyslog.lab.camber.moe,resources=vectorsyslogconfigurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vectorsyslog.lab.camber.moe,resources=vectorsyslogconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=vectorsyslog.lab.camber.moe,resources=vectorsocketsources,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *VectorSyslogConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	config := &vectorsyslogv1alpha1.VectorSyslogConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling VectorSyslogConfiguration", "name", config.Name, "namespace", config.Namespace)

	if config.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(config, vectorFinalizer) {
			controllerutil.AddFinalizer(config, vectorFinalizer)
			if err := r.Update(ctx, config); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(config, vectorFinalizer) {
			if err := r.cleanup(ctx, config); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(config, vectorFinalizer)
			if err := r.Update(ctx, config); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	sources, err := r.selectSources(ctx, config)
	if err != nil {
		return ctrl.Result{}, err
	}

	selectedSourceNames := make([]string, 0, len(sources))
	for _, s := range sources {
		selectedSourceNames = append(selectedSourceNames, s.Name)
	}

	if conflict, conflictMsg := r.checkPortConflicts(sources); conflict {
		log.Error(fmt.Errorf("%s", conflictMsg), "Port conflict detected")
		meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
			Type:    TypeDegraded,
			Status:  metav1.ConditionTrue,
			Reason:  "PortConflict",
			Message: conflictMsg,
		})
		config.Status.Phase = "Failed"
		config.Status.SelectedSources = selectedSourceNames
		return ctrl.Result{}, r.Status().Update(ctx, config)
	}

	if err := r.validatePlaceholders(config); err != nil {
		log.Error(err, "Invalid placeholder in config")
		meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
			Type:    TypeDegraded,
			Status:  metav1.ConditionTrue,
			Reason:  "InvalidPlaceholder",
			Message: err.Error(),
		})
		config.Status.Phase = "Failed"
		config.Status.SelectedSources = selectedSourceNames
		return ctrl.Result{}, r.Status().Update(ctx, config)
	}

	vectorConfig, err := r.renderVectorConfig(config, sources)
	if err != nil {
		log.Error(err, "Failed to render Vector configuration")
		meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
			Type:    TypeDegraded,
			Status:  metav1.ConditionTrue,
			Reason:  "ConfigRenderFailed",
			Message: err.Error(),
		})
		config.Status.Phase = "Failed"
		return ctrl.Result{}, r.Status().Update(ctx, config)
	}
	configHash := fmt.Sprintf("%x", sha256.Sum256([]byte(vectorConfig)))

	if err := r.reconcileConfigMap(ctx, config, vectorConfig); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, config, sources); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileDeployment(ctx, config); err != nil {
		return ctrl.Result{}, err
	}

	config.Status.ObservedGeneration = config.Generation
	config.Status.ConfigHash = configHash
	config.Status.SelectedSources = selectedSourceNames
	config.Status.ExposedPorts = r.getExposedPorts(sources)
	config.Status.Phase = "Active"

	meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
		Type:    TypeAvailable,
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "Configuration successfully reconciled",
	})

	return ctrl.Result{}, r.Status().Update(ctx, config)
}

func (r *VectorSyslogConfigurationReconciler) validatePlaceholders(config *vectorsyslogv1alpha1.VectorSyslogConfiguration) error {
	// 检查 globalPipeline.sinks（现在这是必需的）
	if len(config.Spec.GlobalPipeline.Sinks) == 0 {
		return fmt.Errorf("globalPipeline.sinks must contain at least one sink")
	}

	for name, rawExt := range config.Spec.GlobalPipeline.Sinks {
		if rawExt.Raw == nil {
			return fmt.Errorf("globalPipeline.sinks.%s config is empty", name)
		}
		count := countPlaceholders(string(rawExt.Raw))
		if count != 1 {
			return fmt.Errorf("globalPipeline.sinks.%s must contain exactly one %s, found %d", name, SourcesPlaceholder, count)
		}
	}

	// 检查 globalPipeline.transforms
	for name, rawExt := range config.Spec.GlobalPipeline.Transforms {
		if rawExt.Raw != nil {
			count := countPlaceholders(string(rawExt.Raw))
			if count != 1 {
				return fmt.Errorf("globalPipeline.transforms.%s must contain exactly one %s, found %d", name, SourcesPlaceholder, count)
			}
		}
	}

	return nil
}

func countPlaceholders(s string) int {
	placeholderPattern := regexp.MustCompile(regexp.QuoteMeta(SourcesPlaceholder))
	return len(placeholderPattern.FindAllString(s, -1))
}

func (r *VectorSyslogConfigurationReconciler) selectSources(ctx context.Context, config *vectorsyslogv1alpha1.VectorSyslogConfiguration) ([]vectorsyslogv1alpha1.VectorSocketSource, error) {
	allSources := &vectorsyslogv1alpha1.VectorSocketSourceList{}
	if err := r.List(ctx, allSources, client.InNamespace(config.Namespace)); err != nil {
		return nil, err
	}

	var sources []vectorsyslogv1alpha1.VectorSocketSource
	selector := config.Spec.SourceSelector

	if selector == nil {
		sources = allSources.Items
	} else {
		labelSelector, err := metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			return nil, err
		}
		for _, s := range allSources.Items {
			if labelSelector.Matches(labels.Set(s.Labels)) {
				sources = append(sources, s)
			}
		}
	}

	return sources, nil
}

func (r *VectorSyslogConfigurationReconciler) checkPortConflicts(sources []vectorsyslogv1alpha1.VectorSocketSource) (bool, string) {
	seen := make(map[string]bool)
	for _, s := range sources {
		key := fmt.Sprintf("%s-%d", s.Spec.Mode, s.Spec.Port)
		if seen[key] {
			return true, fmt.Sprintf("Port conflict detected: %s/%d", s.Spec.Mode, s.Spec.Port)
		}
		seen[key] = true
	}
	return false, ""
}

func (r *VectorSyslogConfigurationReconciler) renderVectorConfig(
	config *vectorsyslogv1alpha1.VectorSyslogConfiguration,
	sources []vectorsyslogv1alpha1.VectorSocketSource,
) (string, error) {
	// 构建完整的配置结构
	vectorConfig := make(map[string]interface{})

	// 1. 计算 base inputs
	baseInputs := r.calculateBaseInputs(sources, config)

	// 2. 处理 overwriteConfig 中的 sources
	sourcesMap := make(map[string]interface{})
	if len(config.Spec.OverwriteConfig) > 0 {
		for sectionName, rawExt := range config.Spec.OverwriteConfig {
			if strings.HasPrefix(sectionName, "sources.") {
				sourceName := strings.TrimPrefix(sectionName, "sources.")
				var sourceConfig map[string]interface{}
				if err := json.Unmarshal(rawExt.Raw, &sourceConfig); err != nil {
					return "", err
				}
				sourcesMap[sourceName] = replaceInputsPlaceholder(sourceConfig, baseInputs)
			}
		}
	}

	// 3. 添加自动生成的 sources
	for _, s := range sources {
		sourceName := fmt.Sprintf("socket_%s_%d", string(s.Spec.Mode), s.Spec.Port)
		sourcesMap[sourceName] = map[string]interface{}{
			"type":    "socket",
			"address": fmt.Sprintf("0.0.0.0:%d", s.Spec.Port),
			"mode":    string(s.Spec.Mode),
		}
	}

	if len(sourcesMap) > 0 {
		vectorConfig["sources"] = sourcesMap
	}

	// 4. 处理 transforms
	transformsMap := make(map[string]interface{})

	// source-specific enrich transforms
	enrichEnabled := true
	if config.Spec.GlobalPipeline.EnrichEnabled != nil {
		enrichEnabled = *config.Spec.GlobalPipeline.EnrichEnabled
	}

	if enrichEnabled {
		for _, s := range sources {
			if len(s.Spec.Labels) > 0 {
				sourceName := fmt.Sprintf("socket_%s_%d", string(s.Spec.Mode), s.Spec.Port)
				transformName := fmt.Sprintf("enrich_%s_%d", string(s.Spec.Mode), s.Spec.Port)

				var vrlParts []string
				for k, v := range s.Spec.Labels {
					vrlParts = append(vrlParts, fmt.Sprintf(".source_%s = \"%s\"", k, v))
				}

				transformsMap[transformName] = map[string]interface{}{
					"type":   "remap",
					"inputs": []string{sourceName},
					"source": strings.Join(vrlParts, "\n"),
				}
			}
		}
	}

	// overwriteConfig transforms
	if len(config.Spec.OverwriteConfig) > 0 {
		for sectionName, rawExt := range config.Spec.OverwriteConfig {
			if strings.HasPrefix(sectionName, "transforms.") {
				transformName := strings.TrimPrefix(sectionName, "transforms.")
				var transformConfig map[string]interface{}
				if err := json.Unmarshal(rawExt.Raw, &transformConfig); err != nil {
					return "", err
				}
				transformsMap[transformName] = replaceInputsPlaceholder(transformConfig, baseInputs)
			}
		}
	}

	// globalPipeline.transforms
	for name, rawExt := range config.Spec.GlobalPipeline.Transforms {
		if rawExt.Raw != nil {
			var transformConfig map[string]interface{}
			if err := json.Unmarshal(rawExt.Raw, &transformConfig); err != nil {
				return "", err
			}
			transformsMap[name] = replaceInputsPlaceholder(transformConfig, baseInputs)
		}
	}

	if len(transformsMap) > 0 {
		vectorConfig["transforms"] = transformsMap
	}

	// 5. 处理 sinks
	sinksMap := make(map[string]interface{})

	// overwriteConfig sinks
	if len(config.Spec.OverwriteConfig) > 0 {
		for sectionName, rawExt := range config.Spec.OverwriteConfig {
			if strings.HasPrefix(sectionName, "sinks.") {
				sinkName := strings.TrimPrefix(sectionName, "sinks.")
				var sinkConfig map[string]interface{}
				if err := json.Unmarshal(rawExt.Raw, &sinkConfig); err != nil {
					return "", err
				}
				sinksMap[sinkName] = replaceInputsPlaceholder(sinkConfig, baseInputs)
			}
		}
	}

	// globalPipeline.sinks（主 sinks）
	for name, rawExt := range config.Spec.GlobalPipeline.Sinks {
		if rawExt.Raw != nil {
			var sinkConfig map[string]interface{}
			if err := json.Unmarshal(rawExt.Raw, &sinkConfig); err != nil {
				return "", err
			}
			sinksMap[name] = replaceInputsPlaceholder(sinkConfig, baseInputs)
		}
	}

	if len(sinksMap) > 0 {
		vectorConfig["sinks"] = sinksMap
	}

	// 6. 处理 overwriteConfig 中的其他部分（tests 等）
	if len(config.Spec.OverwriteConfig) > 0 {
		for sectionName, rawExt := range config.Spec.OverwriteConfig {
			if !strings.HasPrefix(sectionName, "sources.") &&
				!strings.HasPrefix(sectionName, "transforms.") &&
				!strings.HasPrefix(sectionName, "sinks.") {
				// 直接添加到根级别
				var otherConfig interface{}
				if err := json.Unmarshal(rawExt.Raw, &otherConfig); err != nil {
					return "", err
				}
				vectorConfig[sectionName] = otherConfig
			}
		}
	}

	// 转换为 YAML
	yamlBytes, err := yaml.Marshal(vectorConfig)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

// replaceInputsPlaceholder 替换 config 中的 inputs 占位符
func replaceInputsPlaceholder(config map[string]interface{}, inputs []string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		switch val := v.(type) {
		case string:
			if val == SourcesPlaceholder {
				result[k] = inputs
			} else {
				result[k] = val
			}
		case map[string]interface{}:
			result[k] = replaceInputsPlaceholder(val, inputs)
		case []interface{}:
			var newArr []interface{}
			for _, item := range val {
				if str, ok := item.(string); ok && str == SourcesPlaceholder {
					for _, input := range inputs {
						newArr = append(newArr, input)
					}
				} else {
					newArr = append(newArr, item)
				}
			}
			result[k] = newArr
		default:
			result[k] = val
		}
	}
	return result
}

func (r *VectorSyslogConfigurationReconciler) calculateBaseInputs(
	sources []vectorsyslogv1alpha1.VectorSocketSource,
	config *vectorsyslogv1alpha1.VectorSyslogConfiguration,
) []string {
	var inputs []string
	enrichEnabled := true
	if config.Spec.GlobalPipeline.EnrichEnabled != nil {
		enrichEnabled = *config.Spec.GlobalPipeline.EnrichEnabled
	}

	for _, s := range sources {
		sourceName := fmt.Sprintf("socket_%s_%d", string(s.Spec.Mode), s.Spec.Port)
		if enrichEnabled && len(s.Spec.Labels) > 0 {
			transformName := fmt.Sprintf("enrich_%s_%d", string(s.Spec.Mode), s.Spec.Port)
			inputs = append(inputs, transformName)
		} else {
			inputs = append(inputs, sourceName)
		}
	}

	return inputs
}

func (r *VectorSyslogConfigurationReconciler) reconcileConfigMap(ctx context.Context, config *vectorsyslogv1alpha1.VectorSyslogConfiguration, vectorConfig string) error {
	cm := &corev1.ConfigMap{}
	cm.Name = config.Name + "-vector-config"
	cm.Namespace = config.Namespace

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := ctrl.SetControllerReference(config, cm, r.Scheme); err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data["vector.yaml"] = vectorConfig
		return nil
	})

	return err
}

func (r *VectorSyslogConfigurationReconciler) reconcileService(ctx context.Context, config *vectorsyslogv1alpha1.VectorSyslogConfiguration, sources []vectorsyslogv1alpha1.VectorSocketSource) error {
	svc := &corev1.Service{}
	svc.Name = config.Name + "-vector"
	svc.Namespace = config.Namespace

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := ctrl.SetControllerReference(config, svc, r.Scheme); err != nil {
			return err
		}

		svcType := corev1.ServiceTypeLoadBalancer
		if config.Spec.Service.Type == vectorsyslogv1alpha1.NodePortService {
			svcType = corev1.ServiceTypeNodePort
		}
		svc.Spec.Type = svcType

		// Set LoadBalancerIP if specified
		if config.Spec.Service.LoadBalancerIP != "" {
			svc.Spec.LoadBalancerIP = config.Spec.Service.LoadBalancerIP
		}

		svc.Spec.Selector = map[string]string{
			"app.kubernetes.io/name":      "vector",
			"app.kubernetes.io/instance":  config.Name,
			"app.kubernetes.io/component": "aggregator",
		}

		var ports []corev1.ServicePort
		for _, s := range sources {
			portName := fmt.Sprintf("%s-%d", string(s.Spec.Mode), s.Spec.Port)
			protocol := corev1.ProtocolTCP
			if s.Spec.Mode == vectorsyslogv1alpha1.UDPMode {
				protocol = corev1.ProtocolUDP
			}

			svcPort := corev1.ServicePort{
				Name:       portName,
				Port:       s.Spec.Port,
				TargetPort: toIntstr(s.Spec.Port),
				Protocol:   protocol,
			}

			if svcType == corev1.ServiceTypeNodePort && s.Spec.NodePort != nil {
				svcPort.NodePort = *s.Spec.NodePort
			}

			ports = append(ports, svcPort)
		}
		svc.Spec.Ports = ports

		if config.Spec.Service.Annotations != nil {
			svc.Annotations = config.Spec.Service.Annotations
		}

		return nil
	})

	return err
}

func (r *VectorSyslogConfigurationReconciler) reconcileDeployment(ctx context.Context, config *vectorsyslogv1alpha1.VectorSyslogConfiguration) error {
	deployment := &appsv1.Deployment{}
	deployment.Name = config.Name + "-vector"
	deployment.Namespace = config.Namespace

	image := config.Spec.Image
	if image == "" {
		image = "timberio/vector:latest"
	}

	replicas := config.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		if err := ctrl.SetControllerReference(config, deployment, r.Scheme); err != nil {
			return err
		}

		deployment.Spec.Replicas = ptr.To(int32(replicas))
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name":      "vector",
				"app.kubernetes.io/instance":  config.Name,
				"app.kubernetes.io/component": "aggregator",
			},
		}

		deployment.Spec.Template.ObjectMeta.Labels = map[string]string{
			"app.kubernetes.io/name":      "vector",
			"app.kubernetes.io/instance":  config.Name,
			"app.kubernetes.io/component": "aggregator",
		}

		container := corev1.Container{
			Name:  "vector",
			Image: image,
			Args:  []string{"--config", "/etc/vector/vector.yaml", "--watch-config"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "config",
					MountPath: "/etc/vector",
				},
			},
		}

		if config.Spec.Resources != nil {
			container.Resources = *config.Spec.Resources
		}

		deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: config.Name + "-vector-config",
						},
					},
				},
			},
		}

		return nil
	})

	return err
}

func (r *VectorSyslogConfigurationReconciler) getExposedPorts(sources []vectorsyslogv1alpha1.VectorSocketSource) []vectorsyslogv1alpha1.ExposedPort {
	var ports []vectorsyslogv1alpha1.ExposedPort
	for _, s := range sources {
		portName := fmt.Sprintf("%s-%d", string(s.Spec.Mode), s.Spec.Port)
		ep := vectorsyslogv1alpha1.ExposedPort{
			Name: portName,
			Mode: string(s.Spec.Mode),
			Port: s.Spec.Port,
		}
		if s.Spec.NodePort != nil {
			ep.NodePort = *s.Spec.NodePort
		}
		ports = append(ports, ep)
	}
	return ports
}

func (r *VectorSyslogConfigurationReconciler) cleanup(ctx context.Context, config *vectorsyslogv1alpha1.VectorSyslogConfiguration) error {
	return nil
}

func (r *VectorSyslogConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vectorsyslogv1alpha1.VectorSyslogConfiguration{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Named("vectorsyslogconfiguration").
		Complete(r)
}

func toIntstr(val int32) intstr.IntOrString {
	return intstr.FromInt32(val)
}
