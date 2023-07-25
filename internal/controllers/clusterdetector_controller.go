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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	plumberv1 "github.com/jnytnai0613/plumber/api/v1"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/healthcheck"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

// ClusterDetectorReconciler reconciles a ClusterDetector object
type ClusterDetectorReconciler struct {
	client.Client
}

// NewClusterDetectorReconciler returns a new ClusterDetectorReconciler
func RemoveClusterDetector(localClient client.Client, log logr.Logger) error {
	config, err := kubeconfig.ReadKubeconfigFromClient(localClient)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	ctx := context.Background()

	var currentClusterDetectorList plumberv1.ClusterDetectorList
	if err := localClient.List(
		ctx,
		&currentClusterDetectorList,
		client.InNamespace(constants.Namespace),
	); err != nil {
		return fmt.Errorf("failed to get ClusterDetectorList: %w", err)
	}

	for _, clusterDetector := range currentClusterDetectorList.Items {
		var flag bool
		for name := range config.Contexts {
			if clusterDetector.Spec.Context == name {
				flag = true
				break
			}
		}
		if !flag {
			if err := localClient.Delete(ctx, &clusterDetector); err != nil {
				return fmt.Errorf("failed to delete ClusterDetector: %w", err)
			}
			log.Info(fmt.Sprintf("[ClusterDetector: %s] Deleted.", clusterDetector.GetName()))
		}
	}

	return nil
}

// Create a Custom Resource ClusterDetector and register the remote cluster status.
func SetupClusterDetector(localClient client.Client, log logr.Logger) error {
	config, err := kubeconfig.ReadKubeconfigFromClient(localClient)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	ctx := context.Background()

	for ctxName, detectCtx := range config.Contexts {
		clusterDetector := &plumberv1.ClusterDetector{}
		clusterDetector.SetNamespace(constants.Namespace)
		// .Metadata.Name must be a lowercase RFC 1123 subdomain must consist of lower case alphanumeric
		// characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com',
		// regex used for validation is [a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*').
		//
		// If ContextName is put in .Metadata.Name, it will be trapped by the above restriction,
		// so the format is "ClusterName.UserName".
		clusterDetector.SetName(fmt.Sprintf("%s.%s", detectCtx.Cluster, detectCtx.AuthInfo))

		/////////////////////////////
		// CreateOrUpdate ClusterDetector
		/////////////////////////////

		// Determine the role of the cluster.
		// The local cluster is the primary and the others are the workers.
		role := make(map[string]string)
		if clusterDetector.GetName() == fmt.Sprintf("%s.%s", constants.ClusterName, constants.AuthInfo) {
			role["app.kubernetes.io/role"] = "primary"
		} else {
			role["app.kubernetes.io/role"] = "secondary"
		}

		if op, err := ctrl.CreateOrUpdate(ctx, localClient, clusterDetector, func() error {
			clusterDetector.Labels = role
			clusterDetector.Spec.Context = ctxName
			clusterDetector.Spec.Cluster = detectCtx.Cluster
			clusterDetector.Spec.User = detectCtx.AuthInfo
			return nil
		}); op != controllerutil.OperationResultNone {
			log.Info(fmt.Sprintf("[ClusterDetector: %s] %s", clusterDetector.GetName(), op))
		} else if err != nil {
			return fmt.Errorf("failed to create or update ClusterDetector: %w", err)
		}

		/////////////////////////////
		// Update Status
		/////////////////////////////
		var (
			currentClusterStatus string
			nextClusterStatus    string
		)
		if err := localClient.Get(
			ctx,
			client.ObjectKey{
				Namespace: constants.Namespace,
				Name:      clusterDetector.GetName(),
			},
			clusterDetector); err != nil {
			// If the resource does not exist, create it.
			// Therefore, Not Found errors are ignored.
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get ClusterDetector: %w", err)
			}
		}
		currentClusterStatus = clusterDetector.Status.ClusterStatus

		// Check if the remote cluster is alive.
		nextClusterStatus = "RUNNING"
		if err := healthcheck.HealthChecks(*config.Clusters[detectCtx.Cluster]); err != nil {
			clusterDetector.Status.Reason = fmt.Sprintf("%s", err)
			if currentClusterStatus != "UNKNOWN" {
				log.Error(err, fmt.Sprintf("[Cluster: %s] Health Check failed.", detectCtx.Cluster))
			}
			nextClusterStatus = "UNKNOWN"
		}
		clusterDetector.Status.ClusterStatus = nextClusterStatus
		if err := localClient.Status().Update(ctx, clusterDetector); err != nil {
			return fmt.Errorf("failed to update ClusterDetector status: %w", err)
		}

		if currentClusterStatus != nextClusterStatus {
			log.Info(fmt.Sprintf("[ClusterDetector: %s] Status update completed.", clusterDetector.GetName()))
		}
	}

	return nil
}

//+kubebuilder:rbac:groups=plumber.jnytnai0613.github.io,resources=clusterdetectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=plumber.jnytnai0613.github.io,resources=clusterdetectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=plumber.jnytnai0613.github.io,resources=clusterdetectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=endpoints,verbs=get
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.6/pkg/reconcile
func (r *ClusterDetectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Remove ClusterDetector that is no longer needed.
	if err := RemoveClusterDetector(r.Client, logger); err != nil {
		logger.Error(err, "Failed to remove ClusterDetector.")
		return ctrl.Result{}, err
	}

	// Create or Update ClusterDetector.
	if err := SetupClusterDetector(r.Client, logger); err != nil {
		logger.Error(err, "Failed to initialize ClusterDetector.")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterDetectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []ctrl.Request {
			return []ctrl.Request{
				{NamespacedName: client.ObjectKey{
					Name:      constants.KubeconfigSecretName,
					Namespace: constants.KubeconfigSecretNamespace,
				}},
			}
		})

	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			old := e.ObjectOld.(*corev1.Secret)
			new := e.ObjectNew.(*corev1.Secret)
			return old.ResourceVersion != new.ResourceVersion
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&plumberv1.ClusterDetector{}).
		Watches(
			&corev1.Secret{},
			mapFn,
			builder.WithPredicates(p),
		).
		Complete(r)
}
