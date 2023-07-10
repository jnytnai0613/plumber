/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	plumberv1 "github.com/jnytnai0613/plumber/api/v1"
	"github.com/jnytnai0613/plumber/internal/controllers"
	"github.com/jnytnai0613/plumber/pkg/client"
	cli "github.com/jnytnai0613/plumber/pkg/client"
	"github.com/jnytnai0613/plumber/pkg/constants"
	"github.com/jnytnai0613/plumber/pkg/kubeconfig"
)

var (
	enableLeaderElection bool
	metricsAddr          string
	probeAddr            string
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "plumber",
	Short: "Replicate resources between clusters.",
	Long:  `Replicate resources between clusters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sub(); err != nil {
			return err
		}

		return nil
	},
}

func sub() error {
	var resyncPeriod = time.Second * 30

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		SyncPeriod:             &resyncPeriod,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "1ea4190f.jnytnai0613.github.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	if err = (&controllers.ClusterDetectorReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterDetector")
		return err
	}
	if err = (&controllers.ReplicatorReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor("replicator-controller"),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Replicator")
		return err
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(plumberv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	cmdFlag := rootCmd.Flags()
	cmdFlag.StringVar(
		&metricsAddr,
		"metrics-bind-address",
		":8080",
		"The address the metric endpoint binds to.",
	)

	cmdFlag.StringVar(
		&probeAddr,
		"health-probe-bind-address",
		":8081",
		"The address the probe endpoint binds to.",
	)

	cmdFlag.BoolVar(
		&enableLeaderElection,
		"leader-elect",
		false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.",
	)

	opts := zap.Options{
		Development: true,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Use the credentials in the Controller Pod's ServiceAccount to generate clients.
	// Used for the following purposes
	// - Initialization of ClusterDetector resource
	// - Obtaining the Secret resource object when extracting kubeconfig
	//   from the config Secret resource in the kubeconfig namespace
	localClient, restConfig, err := cli.CreateLocalClient(setupLog, *scheme)
	if err != nil {
		setupLog.Error(err, "Failed to create local client.")
		os.Exit(1)
	}

	// Create a kubeconfig file for the primary cluster.
	// This is used to create a clientset for the primary cluster.
	clientset, err := client.CreateClientSetFromRestConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create clientset.")
		os.Exit(1)
	}

	// Generate a kubeconfig file for the primary cluster.
	primaryConfig, err := kubeconfig.GeneratePrimaryConfig(clientset, restConfig)
	if err != nil {
		setupLog.Error(err, "Failed to generate primary config.")
		os.Exit(1)
	}

	// Create a clientset for the primary cluster.
	// This is used to create a clientset for the primary cluster.
	namespaceClient := clientset.CoreV1().Namespaces()
	secretClient := clientset.CoreV1().Secrets(constants.KubeconfigSecretNamespace)
	if err := kubeconfig.ApplyNamespacedSecret(
		context.Background(),
		namespaceClient,
		secretClient,
		primaryConfig,
	); err != nil {
		setupLog.Error(err, "Failed to apply namespaced secret.")
		os.Exit(1)
	}

	setupLog.Info("Initializing ClusterDetector resources")
	// Initialization of ClusterDetector resource
	if err = controllers.SetupClusterDetector(localClient, setupLog); err != nil {
		if !errors.IsNotFound(err) {
			setupLog.Error(err, "Failed to initialize ClusterDetector.")
			os.Exit(1)
		}
	}
	setupLog.Info("Initialization of all ClusterDetector resources completed")
}
