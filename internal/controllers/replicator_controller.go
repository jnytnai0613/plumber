/*
MIT License
Copyright (c) 2023 Junya Taniai

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	networkv1apply "k8s.io/client-go/applyconfigurations/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	plumberv1 "github.com/jnytnai0613/plumber/api/v1"
	cli "github.com/jnytnai0613/plumber/pkg/client"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/pki"
)

// ReplicatorReconciler reconciles a Replicator object
type ReplicatorReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

type ReplicateRuntime struct {
	ClientSet  *kubernetes.Clientset
	IsPrimary  bool
	Context    context.Context
	Log        logr.Logger
	Cluster    string
	Replicator plumberv1.Replicator
	Request    reconcile.Request
}

var (
	syncStatus []plumberv1.PerResourceApplyStatus
	owner      *metav1apply.OwnerReferenceApplyConfiguration
	replicator plumberv1.Replicator
)

// Create OwnerReference with CR as Owner
func createOwnerReferences(
	log logr.Logger,
	scheme *runtime.Scheme,
) error {
	gvk, err := apiutil.GVKForObject(&replicator, scheme)
	if err != nil {
		log.Error(err, "Unable get GVK")
		return fmt.Errorf("unable to get GVK: %w", err)
	}

	owner = metav1apply.OwnerReference().
		WithAPIVersion(gvk.GroupVersion().String()).
		WithKind(gvk.Kind).
		WithName(replicator.GetName()).
		WithUID(replicator.GetUID()).
		WithBlockOwnerDeletion(true).
		WithController(true)

	return nil
}

func (r *ReplicatorReconciler) applyConfigMap(
	applyRuntime ReplicateRuntime,
	fieldMgr string,
) error {
	var (
		configMapClient = applyRuntime.ClientSet.CoreV1().ConfigMaps(applyRuntime.Replicator.Spec.ReplicationNamespace)
		log             = applyRuntime.Log
	)

	nextConfigMapApplyConfig := corev1apply.ConfigMap(
		applyRuntime.Replicator.Spec.ConfigMapName,
		applyRuntime.Replicator.Spec.ReplicationNamespace).
		WithData(applyRuntime.Replicator.Spec.ConfigMapData)

	if applyRuntime.IsPrimary {
		nextConfigMapApplyConfig.WithOwnerReferences(owner)
	}

	configMap, err := configMapClient.Get(
		applyRuntime.Context,
		applyRuntime.Replicator.Spec.ConfigMapName,
		metav1.GetOptions{},
	)
	if err != nil {
		// If the resource does not exist, create it.
		// Therefore, Not Found errors are ignored.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get ConfigMap: %w", err)
		}
	}
	currConfigMapApplyConfig, err := corev1apply.ExtractConfigMap(configMap, fieldMgr)
	if err != nil {
		return fmt.Errorf("failed to extract ConfigMap: %w", err)
	}

	kind := *nextConfigMapApplyConfig.Kind
	name := *nextConfigMapApplyConfig.Name
	applyStatus := "applied"
	s := plumberv1.PerResourceApplyStatus{
		Cluster:     applyRuntime.Cluster,
		Kind:        kind,
		Name:        name,
		ApplyStatus: applyStatus,
	}
	if equality.Semantic.DeepEqual(currConfigMapApplyConfig, nextConfigMapApplyConfig) {
		syncStatus = append(syncStatus, s)
		return nil
	}

	applied, err := configMapClient.Apply(
		applyRuntime.Context,
		nextConfigMapApplyConfig,
		metav1.ApplyOptions{
			FieldManager: fieldMgr,
			Force:        true,
		},
	)
	if err != nil {
		applyStatus = "not applied"
		s.ApplyStatus = applyStatus
		syncStatus = append(syncStatus, s)

		log.Error(err, "unable to apply")
		return fmt.Errorf("failed to apply ConfigMap: %w", err)
	}

	syncStatus = append(syncStatus, s)

	log.Info(fmt.Sprintf("Nginx ConfigMap Applied: [cluster] %s, [resource] %s", applyRuntime.Cluster, applied.GetName()))

	return nil
}

func (r *ReplicatorReconciler) applyDeployment(
	applyRuntime ReplicateRuntime,
	fieldMgr string,
) error {
	var (
		deploymentClient = applyRuntime.ClientSet.AppsV1().Deployments(applyRuntime.Replicator.Spec.ReplicationNamespace)
		labels           = map[string]string{"apps": "nginx"}
		log              = applyRuntime.Log
	)

	nextDeploymentApplyConfig := appsv1apply.Deployment(
		applyRuntime.Replicator.Spec.DeploymentName,
		applyRuntime.Replicator.Spec.ReplicationNamespace).
		WithSpec(appsv1apply.DeploymentSpec().
			WithSelector(metav1apply.LabelSelector().
				WithMatchLabels(labels)))

	if applyRuntime.Replicator.Spec.DeploymentSpec.Replicas != nil {
		replicas := *applyRuntime.Replicator.Spec.DeploymentSpec.Replicas
		nextDeploymentApplyConfig.Spec.WithReplicas(replicas)
	}

	if applyRuntime.Replicator.Spec.DeploymentSpec.Strategy != nil {
		types := *applyRuntime.Replicator.Spec.DeploymentSpec.Strategy.Type
		rollingUpdate := applyRuntime.Replicator.Spec.DeploymentSpec.Strategy.RollingUpdate
		nextDeploymentApplyConfig.Spec.WithStrategy(appsv1apply.DeploymentStrategy().
			WithType(types).
			WithRollingUpdate(rollingUpdate))
	}

	podTemplate := applyRuntime.Replicator.Spec.DeploymentSpec.Template
	podTemplate.WithLabels(labels)

	nextDeploymentApplyConfig.Spec.WithTemplate(podTemplate)

	if applyRuntime.IsPrimary {
		nextDeploymentApplyConfig.WithOwnerReferences(owner)
	}

	// If EmptyDir is not set to Medium or SizeLimit, applyconfiguration
	// returns an empty pointer address. Therefore, a comparison between
	// applyconfiguration in subsequent steps will always detect a difference.
	// In the following process, if neither Medium nor SizeLimit is set,
	// explicitly set nil to prevent the above problem.
	for i, v := range nextDeploymentApplyConfig.Spec.Template.Spec.Volumes {
		e := v.EmptyDir
		if e != nil {
			if v.EmptyDir.Medium != nil || v.EmptyDir.SizeLimit != nil {
				break
			}
			nextDeploymentApplyConfig.Spec.Template.Spec.Volumes[i].
				WithEmptyDir(nil)
		}
	}

	deployment, err := deploymentClient.Get(
		applyRuntime.Context,
		applyRuntime.Replicator.Spec.DeploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		// If the resource does not exist, create it.
		// Therefore, Not Found errors are ignored.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Deployment: %w", err)
		}
	}
	currDeploymentMapApplyConfig, err := appsv1apply.ExtractDeployment(deployment, fieldMgr)
	if err != nil {
		return fmt.Errorf("failed to extract Deployment: %w", err)
	}

	kind := *nextDeploymentApplyConfig.Kind
	name := *nextDeploymentApplyConfig.Name
	applyStatus := "applied"
	s := plumberv1.PerResourceApplyStatus{
		Cluster:     applyRuntime.Cluster,
		Kind:        kind,
		Name:        name,
		ApplyStatus: applyStatus,
	}
	if equality.Semantic.DeepEqual(currDeploymentMapApplyConfig, nextDeploymentApplyConfig) {
		syncStatus = append(syncStatus, s)
		return nil
	}

	applied, err := deploymentClient.Apply(
		applyRuntime.Context,
		nextDeploymentApplyConfig,
		metav1.ApplyOptions{
			FieldManager: fieldMgr,
			Force:        true,
		},
	)
	if err != nil {
		applyStatus = "not applied"
		s.ApplyStatus = applyStatus
		syncStatus = append(syncStatus, s)

		log.Error(err, "unable to apply")
		return fmt.Errorf("failed to apply Deployment: %w", err)
	}

	syncStatus = append(syncStatus, s)

	log.Info(fmt.Sprintf("Nginx Deployment Applied: [cluster] %s, [resource] %s", applyRuntime.Cluster, applied.GetName()))

	return nil
}

func (r *ReplicatorReconciler) applyService(
	applyRuntime ReplicateRuntime,
	fieldMgr string,
) error {
	var (
		serviceClient = applyRuntime.ClientSet.CoreV1().Services(applyRuntime.Replicator.Spec.ReplicationNamespace)
		labels        = map[string]string{"apps": "nginx"}
		log           = applyRuntime.Log
	)

	nextServiceApplyConfig := corev1apply.Service(
		applyRuntime.Replicator.Spec.ServiceName,
		applyRuntime.Replicator.Spec.ReplicationNamespace).
		WithSpec((*corev1apply.ServiceSpecApplyConfiguration)(applyRuntime.Replicator.Spec.ServiceSpec).
			WithSelector(labels))

	if applyRuntime.IsPrimary {
		nextServiceApplyConfig.WithOwnerReferences(owner)
	}

	service, err := serviceClient.Get(
		applyRuntime.Context,
		applyRuntime.Replicator.Spec.ServiceName,
		metav1.GetOptions{},
	)
	if err != nil {
		// If the resource does not exist, create it.
		// Therefore, Not Found errors are ignored.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Service: %w", err)
		}
	}
	currServiceApplyConfig, err := corev1apply.ExtractService(service, fieldMgr)
	if err != nil {
		return fmt.Errorf("failed to extract Service: %w", err)
	}

	kind := *nextServiceApplyConfig.Kind
	name := *nextServiceApplyConfig.Name
	applyStatus := "applied"
	s := plumberv1.PerResourceApplyStatus{
		Cluster:     applyRuntime.Cluster,
		Kind:        kind,
		Name:        name,
		ApplyStatus: applyStatus,
	}
	if equality.Semantic.DeepEqual(currServiceApplyConfig, nextServiceApplyConfig) {
		syncStatus = append(syncStatus, s)
		return nil
	}

	applied, err := serviceClient.Apply(
		applyRuntime.Context,
		nextServiceApplyConfig,
		metav1.ApplyOptions{
			FieldManager: fieldMgr,
			Force:        true,
		},
	)
	if err != nil {
		applyStatus = "not applied"
		s.ApplyStatus = applyStatus
		syncStatus = append(syncStatus, s)

		log.Error(err, "unable to apply")
		return fmt.Errorf("failed to apply Service: %w", err)
	}

	syncStatus = append(syncStatus, s)

	log.Info(fmt.Sprintf("Nginx Service Applied: [cluster] %s, [resource] %s", applyRuntime.Cluster, applied.GetName()))

	return nil
}

func (r *ReplicatorReconciler) applyIngress(
	applyRuntime ReplicateRuntime,
	fieldMgr string,
) error {
	var (
		annotateRewriteTarget = map[string]string{"nginx.ingress.kubernetes.io/rewrite-target": "/"}
		annotateVerifyClient  = map[string]string{"nginx.ingress.kubernetes.io/auth-tls-verify-client": "on"}
		annotateTlsSecret     = map[string]string{"nginx.ingress.kubernetes.io/auth-tls-secret": fmt.Sprintf("%s/%s", applyRuntime.Replicator.Spec.ReplicationNamespace, constants.IngressSecretName)}
		ingressClient         = applyRuntime.ClientSet.NetworkingV1().Ingresses(applyRuntime.Replicator.Spec.ReplicationNamespace)
		log                   = applyRuntime.Log
		secretClient          = applyRuntime.ClientSet.CoreV1().Secrets(applyRuntime.Replicator.Spec.ReplicationNamespace)
	)

	nextIngressApplyConfig := networkv1apply.Ingress(
		applyRuntime.Replicator.Spec.IngressName,
		applyRuntime.Replicator.Spec.ReplicationNamespace).
		WithAnnotations(annotateRewriteTarget).
		WithSpec((*networkv1apply.IngressSpecApplyConfiguration)(applyRuntime.Replicator.Spec.IngressSpec).
			WithIngressClassName(constants.IngressClassName))

	ingress, err := ingressClient.Get(
		applyRuntime.Context,
		applyRuntime.Replicator.Spec.IngressName,
		metav1.GetOptions{},
	)
	if err != nil {
		// If the resource does not exist, create it.
		// Therefore, Not Found errors are ignored.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Ingress: %w", err)
		}
	}

	if applyRuntime.Replicator.Spec.IngressSecureEnabled {
		// Re-create Secret if 'spec.tls[].hosts[]' has changed
		if len(ingress.Spec.TLS) > 0 {
			secrets, err := secretClient.List(
				applyRuntime.Context,
				metav1.ListOptions{},
			)
			if err != nil {
				return fmt.Errorf("failed to list Secret: %w", err)
			}

			ih := ingress.Spec.TLS[0].Hosts[0]
			sh := *applyRuntime.Replicator.Spec.IngressSpec.Rules[0].Host
			if ih != sh {
				log.Info("Host is not different")
				for _, secret := range secrets.Items {
					if err := secretClient.Delete(
						applyRuntime.Context,
						secret.GetName(),
						metav1.DeleteOptions{},
					); err != nil {
						return fmt.Errorf("failed to delete Secret: %w", err)
					}
					log.Info(fmt.Sprintf("delete Secret resource: %s", secret.GetName()))
				}
			}
		}

		if err := r.applyIngressSecret(
			applyRuntime,
			constants.FieldManager,
		); err != nil {
			log.Error(err, "Unable create Ingress Secret")
			return fmt.Errorf("unable to create Ingress Secret: %w", err)
		}

		if err := r.applyClientSecret(
			applyRuntime,
			constants.FieldManager,
		); err != nil {
			log.Error(err, "Unable create Client Secret")
			return fmt.Errorf("unable to create Client Secret: %w", err)
		}

		nextIngressApplyConfig.
			WithAnnotations(annotateVerifyClient).
			WithAnnotations(annotateTlsSecret).
			Spec.
			WithTLS(networkv1apply.IngressTLS().
				WithHosts(*applyRuntime.Replicator.Spec.IngressSpec.Rules[0].Host).
				WithSecretName(constants.IngressSecretName))
	}

	// TODO:
	// Need to consider duplicate checks.
	if len(nextIngressApplyConfig.Spec.TLS) > 0 {
		// When replicating to multiple targets, the TLS field may contain multiple identical values.
		// Therefore, only one value is allowed.
		nextIngressApplyConfig.Spec.TLS = nextIngressApplyConfig.Spec.TLS[:1]
	}

	if applyRuntime.IsPrimary {
		nextIngressApplyConfig.WithOwnerReferences(owner)
	}

	currIngressApplyConfig, err := networkv1apply.ExtractIngress(ingress, fieldMgr)
	if err != nil {
		return fmt.Errorf("failed to extract Ingress: %w", err)
	}

	kind := *nextIngressApplyConfig.Kind
	name := *nextIngressApplyConfig.Name
	applyStatus := "applied"
	s := plumberv1.PerResourceApplyStatus{
		Cluster:     applyRuntime.Cluster,
		Kind:        kind,
		Name:        name,
		ApplyStatus: applyStatus,
	}
	if equality.Semantic.DeepEqual(currIngressApplyConfig, nextIngressApplyConfig) {
		syncStatus = append(syncStatus, s)
		return nil
	}

	applied, err := ingressClient.Apply(
		applyRuntime.Context,
		nextIngressApplyConfig,
		metav1.ApplyOptions{
			FieldManager: fieldMgr,
			Force:        true,
		},
	)
	if err != nil {
		applyStatus = "not applied"
		s.ApplyStatus = applyStatus
		syncStatus = append(syncStatus, s)

		log.Error(err, "unable to apply")
		return fmt.Errorf("failed to apply Ingress: %w", err)
	}

	syncStatus = append(syncStatus, s)

	log.Info(fmt.Sprintf("Nginx Ingress Applied: [cluster] %s, [resource] %s", applyRuntime.Cluster, applied.GetName()))

	return nil
}

func (r *ReplicatorReconciler) applyIngressSecret(
	applyRuntime ReplicateRuntime,
	fieldMgr string,
) error {
	var (
		log          = applyRuntime.Log
		secretClient = applyRuntime.ClientSet.CoreV1().Secrets(applyRuntime.Replicator.Spec.ReplicationNamespace)
	)

	secret, err := secretClient.Get(
		applyRuntime.Context,
		constants.ClientSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		// If the resource does not exist, create it.
		// Therefore, Not Found errors are ignored.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Secret: %w", err)
		}
	}

	if len(secret.GetName()) > 0 {
		return nil
	}

	caCrt, _, err := pki.CreateCaCrt()
	if err != nil {
		log.Error(err, "Unable create CA Certificates")
		return fmt.Errorf("unable to create CA Certificates: %w", err)
	}

	svrCrt, svrKey, err := pki.CreateSvrCrt(applyRuntime.Replicator)
	if err != nil {
		log.Error(err, "Unable create Server Certificates")
		return fmt.Errorf("unable to create Server Certificates: %w", err)
	}

	secData := map[string][]byte{
		"tls.crt": svrCrt,
		"tls.key": svrKey,
		"ca.crt":  caCrt,
	}

	nextIngressSecretApplyConfig := corev1apply.Secret(
		constants.IngressSecretName,
		applyRuntime.Replicator.Spec.ReplicationNamespace).
		WithData(secData)

	if applyRuntime.IsPrimary {
		nextIngressSecretApplyConfig.WithOwnerReferences(owner)
	}

	kind := *nextIngressSecretApplyConfig.Kind
	name := *nextIngressSecretApplyConfig.Name
	applyStatus := "applied"
	s := plumberv1.PerResourceApplyStatus{
		Cluster:     applyRuntime.Cluster,
		Kind:        kind,
		Name:        name,
		ApplyStatus: applyStatus,
	}

	applied, err := secretClient.Apply(
		applyRuntime.Context,
		nextIngressSecretApplyConfig,
		metav1.ApplyOptions{
			FieldManager: fieldMgr,
			Force:        true,
		},
	)
	if err != nil {
		applyStatus = "not applied"
		s.ApplyStatus = applyStatus
		syncStatus = append(syncStatus, s)

		log.Error(err, "unable to apply")
		return fmt.Errorf("failed to apply Secret: %w", err)
	}

	syncStatus = append(syncStatus, s)

	log.Info(fmt.Sprintf("Nginx Server Certificates Secret Applied: [cluster] %s, [resource] %s", applyRuntime.Cluster, applied.GetName()))

	return nil
}

func (r *ReplicatorReconciler) applyClientSecret(
	applyRuntime ReplicateRuntime,
	fieldMgr string,
) error {
	var (
		log          = applyRuntime.Log
		secretClient = applyRuntime.ClientSet.CoreV1().Secrets(applyRuntime.Replicator.Spec.ReplicationNamespace)
	)

	secret, err := secretClient.Get(
		applyRuntime.Context,
		constants.ClientSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		// If the resource does not exist, create it.
		// Therefore, Not Found errors are ignored.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Secret: %w", err)
		}
	}

	if len(secret.GetName()) > 0 {
		return nil
	}

	cliCrt, cliKey, err := pki.CreateClientCrt()
	if err != nil {
		log.Error(err, "Unable create Client Certificates")
		return fmt.Errorf("unable to create Client Certificates: %w", err)
	}

	secData := map[string][]byte{
		"client.crt": cliCrt,
		"client.key": cliKey,
	}

	nextClientSecretApplyConfig := corev1apply.Secret(
		constants.ClientSecretName,
		applyRuntime.Replicator.Spec.ReplicationNamespace).
		WithData(secData)

	if applyRuntime.IsPrimary {
		nextClientSecretApplyConfig.WithOwnerReferences(owner)
	}

	applied, err := secretClient.Apply(
		applyRuntime.Context,
		nextClientSecretApplyConfig,
		metav1.ApplyOptions{
			FieldManager: fieldMgr,
			Force:        true,
		},
	)
	if err != nil {
		log.Error(err, "unable to apply")
		return fmt.Errorf("failed to apply Secret: %w", err)
	}

	log.Info(fmt.Sprintf("Nginx Client Certificates Secret Applied: [cluster] %s, [resource] %s", applyRuntime.Cluster, applied.GetName()))

	return nil
}

func (r *ReplicatorReconciler) applyResources(
	applyFuncArgs ReplicateRuntime,
) error {
	var applyErr error

	if applyFuncArgs.Replicator.Spec.ConfigMapData != nil {
		// Create Configmap
		// Generate default.conf and index.html
		if err := r.applyConfigMap(
			applyFuncArgs,
			constants.FieldManager,
		); err != nil {
			applyErr = multierr.Append(applyErr, err)
		}
	}

	// Create Deployment
	// Deployment resources are required.
	if err := r.applyDeployment(
		applyFuncArgs,
		constants.FieldManager,
	); err != nil {
		applyErr = multierr.Append(applyErr, err)
	}

	// Create Service
	if applyFuncArgs.Replicator.Spec.ServiceSpec != nil {
		if err := r.applyService(
			applyFuncArgs,
			constants.FieldManager,
		); err != nil {
			applyErr = multierr.Append(applyErr, err)
		}
	}

	// Create Ingress
	if applyFuncArgs.Replicator.Spec.IngressSpec != nil {
		if err := r.applyIngress(
			applyFuncArgs,
			constants.FieldManager,
		); err != nil {
			applyErr = multierr.Append(applyErr, err)
		}
	}

	return applyErr
}

func (r *ReplicatorReconciler) Replicate(
	ctx context.Context,
	log logr.Logger,
	req ctrl.Request,
	primaryClientSet map[string]*kubernetes.Clientset,
	secondaryClientsets map[string]*kubernetes.Clientset,
) error {
	var (
		applyFailed      bool
		err              error
		replicateRuntime ReplicateRuntime
	)

	replicateRuntime = ReplicateRuntime{
		Context:    ctx,
		Log:        log,
		Replicator: replicator,
		Request:    req,
	}

	for primaryClusterName, clientSet := range primaryClientSet {
		replicateRuntime.ClientSet = clientSet
		replicateRuntime.IsPrimary = true
		replicateRuntime.Cluster = primaryClusterName
		if err := r.applyResources(replicateRuntime); err != nil {
			return fmt.Errorf("failed to apply resources: %w", err)
		}
	}

	// After successful resource deployment to the local cluster,
	// replicate the resources to the remote cluster.
	for secondaryClusterName, clientSet := range secondaryClientsets {
		replicateRuntime.ClientSet = clientSet
		replicateRuntime.IsPrimary = false
		replicateRuntime.Cluster = secondaryClusterName
		if err = r.applyResources(replicateRuntime); err != nil {
			applyFailed = true
			log.Error(err, fmt.Sprintf("Could not replicate to Secondary Cluster %s", secondaryClusterName))
		}
	}
	// If one of the clusters fails to replicate, it is considered a synchronization failure.
	if applyFailed {
		return fmt.Errorf("Could not sync on all clusters")
	}

	return nil
}

func createNamespace(
	ctx context.Context,
	log logr.Logger,
	replicator plumberv1.Replicator,
	secondaryClientsets map[string]*kubernetes.Clientset,
) error {
	for cluster, clientSet := range secondaryClientsets {
		var namespaceClient = clientSet.CoreV1().Namespaces()

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: replicator.Spec.ReplicationNamespace,
			},
		}

		if _, err := namespaceClient.Get(
			ctx,
			replicator.Spec.ReplicationNamespace,
			metav1.GetOptions{},
		); err != nil {
			// If the resource does not exist, create it.
			// Therefore, Not Found errors are ignored.
			if !errors.IsNotFound(err) {
				return fmt.Errorf("Could not get namespace %w", err)
			}

			created, err := namespaceClient.Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("Could not create namespace %w", err)
			}

			log.Info(fmt.Sprintf("Namespace creation: [cluster] %s, [resource] %s", cluster, created.GetName()))
		}
	}
	return nil
}

func deletePrimaryNamespace(
	ctx context.Context,
	log logr.Logger,
	primaryClientSet map[string]*kubernetes.Clientset,
	primaryNamespaceName string,
) error {
	for _, clientSet := range primaryClientSet {
		var namespaceClient = clientSet.CoreV1().Namespaces()
		if err := namespaceClient.Delete(
			ctx,
			primaryNamespaceName,
			metav1.DeleteOptions{},
		); err != nil {
			log.Error(err, "unable to delete primary namespace")
		}
	}

	return nil
}

func deleteSecondaryClusterResources(
	ctx context.Context,
	log logr.Logger,
	replicator plumberv1.Replicator,
	secondaryClientsets map[string]*kubernetes.Clientset,
) error {
	var deleteErr error
	for cluster, clientSet := range secondaryClientsets {
		var (
			configMapClient  = clientSet.CoreV1().ConfigMaps(replicator.Spec.ReplicationNamespace)
			deploymentClient = clientSet.AppsV1().Deployments(replicator.Spec.ReplicationNamespace)
			serviceClient    = clientSet.CoreV1().Services(replicator.Spec.ReplicationNamespace)
			ingressClient    = clientSet.NetworkingV1().Ingresses(replicator.Spec.ReplicationNamespace)
			secretClient     = clientSet.CoreV1().Secrets(replicator.Spec.ReplicationNamespace)
			namespaceClient  = clientSet.CoreV1().Namespaces()
		)

		if replicator.Spec.IngressSecureEnabled {
			if err := secretClient.Delete(
				ctx,
				constants.ClientSecretName,
				metav1.DeleteOptions{},
			); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete client secret for secondary cluster %s.", cluster))
				deleteErr = multierr.Append(deleteErr, err)
			}

			if err := secretClient.Delete(
				ctx,
				constants.IngressSecretName,
				metav1.DeleteOptions{},
			); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete server secret for secondary cluster %s.", cluster))
				deleteErr = multierr.Append(deleteErr, err)
			}
		}

		if replicator.Spec.IngressSpec != nil {
			if err := ingressClient.Delete(
				ctx,
				replicator.Spec.IngressName,
				metav1.DeleteOptions{},
			); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete ingress for secondary cluster %s.", cluster))
				deleteErr = multierr.Append(deleteErr, err)
			}
		}

		if replicator.Spec.ServiceSpec != nil {
			if err := serviceClient.Delete(
				ctx,
				replicator.Spec.ServiceName,
				metav1.DeleteOptions{},
			); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete service for secondary cluster %s.", cluster))
				deleteErr = multierr.Append(deleteErr, err)
			}
		}

		// Required Resources
		if err := deploymentClient.Delete(
			ctx,
			replicator.Spec.DeploymentName,
			metav1.DeleteOptions{},
		); err != nil {
			log.Error(err, fmt.Sprintf("Unable to delete deployment for secondary cluster %s.", cluster))
			deleteErr = multierr.Append(deleteErr, err)
		}

		if replicator.Spec.ConfigMapData != nil {
			if err := configMapClient.Delete(
				ctx,
				replicator.Spec.ConfigMapName,
				metav1.DeleteOptions{},
			); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete configmap for secondary cluster %s.", cluster))
				deleteErr = multierr.Append(deleteErr, err)
			}
		}

		if err := namespaceClient.Delete(
			ctx,
			replicator.Spec.ReplicationNamespace,
			metav1.DeleteOptions{},
		); err != nil {
			log.Error(err, fmt.Sprintf("Unable to delete namespace for secondary cluster %s.", cluster))
			deleteErr = multierr.Append(deleteErr, err)
		}
	}

	return deleteErr
}

//+kubebuilder:rbac:groups=plumber.jnytnai0613.github.io,resources=replicators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=plumber.jnytnai0613.github.io,resources=replicators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=plumber.jnytnai0613.github.io,resources=replicators/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.6/pkg/reconcile
func (r *ReplicatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		logger               = log.FromContext(ctx)
		clusterDetectors     plumberv1.ClusterDetectorList
		primaryNamespaceName string
	)

	if err := createOwnerReferences(logger, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Client.List(ctx, &clusterDetectors, client.InNamespace(constants.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Client.Get(ctx, req.NamespacedName, &replicator); err != nil {
		logger.Error(err, "unable to fetch CR Replicator")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Generate ClientSet for primary cluster.
	// ClientSet for primary cluster are used for replication.
	primaryClientsets, err := cli.CreatePrimaryClientsets()
	if err != nil {
		logger.Error(err, "Unable to create primary clientset")
		return ctrl.Result{}, err
	}

	// Generate ClientSet for secondary cluster.
	// ClientSet for secondary cluster are used for replication and Finalize.
	secondaryClientsets, err := cli.CreateSecondaryClientsets(ctx, r.Client, replicator)
	if err != nil {
		logger.Error(err, "Unable to create secondary clientset")
		return ctrl.Result{}, err
	}

	// Resources in secondary clusters are considered external resources.
	// Therefore, they are deleted by finalizer.
	finalizerName := "plumber.jnytnai0613.github.io/finalizer"
	if !replicator.ObjectMeta.DeletionTimestamp.IsZero() {
		primaryNamespaceName = replicator.Spec.ReplicationNamespace
		if controllerutil.ContainsFinalizer(&replicator, finalizerName) {
			if err := deleteSecondaryClusterResources(ctx, logger, replicator, secondaryClientsets); err != nil {
				logger.Error(err, "Unable to delete secondary cluster resources")
			}

			controllerutil.RemoveFinalizer(&replicator, finalizerName)
			if err := r.Update(ctx, &replicator); err != nil {
				return ctrl.Result{}, err
			}
		}

		// After controllerutil.RemoveFinalizer processing, the Replicator resource is deleted.
		// Since the child resources are deleted by deleting the Replicator resource,
		// namespace can also be deleted.
		if err := deletePrimaryNamespace(
			ctx,
			logger,
			primaryClientsets,
			primaryNamespaceName,
		); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&replicator, finalizerName) {
		controllerutil.AddFinalizer(&replicator, finalizerName)
		if err := r.Update(ctx, &replicator); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create a namespace for replication in the Primary Cluster.
	if err := createNamespace(ctx, logger, replicator, primaryClientsets); err != nil {
		logger.Error(err, "Unable to create namespace for primary cluster.")
	}

	// Create a namespace for replication in the Secondary Cluster.
	if err := createNamespace(ctx, logger, replicator, secondaryClientsets); err != nil {
		logger.Error(err, "Unable to create namespace for secondary cluster.")
	}

	// Initialize syncStatus slice once to update the status of the replicator.
	// If not initialized, the status held in the previous Reconcile is used.
	syncStatus = nil
	if err := r.Replicate(ctx, logger, req, primaryClientsets, secondaryClientsets); err != nil {
		replicator.Status.Applied = syncStatus
		replicator.Status.Synced = "not synced"
		if err := r.Status().Update(ctx, &replicator); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	replicator.Status.Applied = syncStatus
	replicator.Status.Synced = "synced"
	if err := r.Status().Update(ctx, &replicator); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplicatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&plumberv1.Replicator{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkv1.Ingress{}).
		Complete(r)
}
