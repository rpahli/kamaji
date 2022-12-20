// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"flag"
	"fmt"
	"io"
	"os"
	goRuntime "runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	cmdutils "github.com/clastix/kamaji/cmd/utils"
	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/controllers/soot"
	"github.com/clastix/kamaji/internal"
	datastoreutils "github.com/clastix/kamaji/internal/datastore/utils"
	"github.com/clastix/kamaji/internal/webhook"
)

func NewCmd(scheme *runtime.Scheme) *cobra.Command {
	// CLI flags
	var (
		metricsBindAddress        string
		healthProbeBindAddress    string
		leaderElect               bool
		tmpDirectory              string
		kineImage                 string
		datastore                 string
		managerNamespace          string
		managerServiceAccountName string
		managerServiceName        string
		webhookCABundle           []byte
		migrateJobImage           string

		webhookCAPath string
	)

	ctx := ctrl.SetupSignalHandler()

	cmd := &cobra.Command{
		Use:           "manager",
		Short:         "Start the Kamaji Kubernetes Operator",
		SilenceErrors: false,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) (err error) {
			// Avoid to pollute Kamaji stdout with useless details by the underlying klog implementations
			klog.SetOutput(io.Discard)
			klog.LogToStderr(false)

			if err = cmdutils.CheckFlags(cmd.Flags(), []string{"kine-image", "datastore", "migrate-image", "tmp-directory", "pod-namespace", "webhook-service-name", "serviceaccount-name", "webhook-ca-path"}...); err != nil {
				return err
			}

			if webhookCABundle, err = os.ReadFile(webhookCAPath); err != nil {
				return fmt.Errorf("unable to read webhook CA: %w", err)
			}

			if err = datastoreutils.CheckExists(ctx, scheme, datastore); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			setupLog := ctrl.Log.WithName("setup")

			setupLog.Info(fmt.Sprintf("Kamaji version %s %s%s", internal.GitTag, internal.GitCommit, internal.GitDirty))
			setupLog.Info(fmt.Sprintf("Build from: %s", internal.GitRepo))
			setupLog.Info(fmt.Sprintf("Build date: %s", internal.BuildTime))
			setupLog.Info(fmt.Sprintf("Go Version: %s", goRuntime.Version()))
			setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goRuntime.GOOS, goRuntime.GOARCH))

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme:                  scheme,
				MetricsBindAddress:      metricsBindAddress,
				Port:                    9443,
				HealthProbeBindAddress:  healthProbeBindAddress,
				LeaderElection:          leaderElect,
				LeaderElectionNamespace: managerNamespace,
				LeaderElectionID:        "799b98bc.clastix.io",
			})
			if err != nil {
				setupLog.Error(err, "unable to start manager")

				return err
			}

			tcpChannel := make(controllers.TenantControlPlaneChannel)

			if err = (&controllers.DataStore{TenantControlPlaneTrigger: tcpChannel}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "DataStore")

				return err
			}

			reconciler := &controllers.TenantControlPlaneReconciler{
				Client:    mgr.GetClient(),
				APIReader: mgr.GetAPIReader(),
				Config: controllers.TenantControlPlaneReconcilerConfig{
					DefaultDataStoreName: datastore,
					KineContainerImage:   kineImage,
					TmpBaseDirectory:     tmpDirectory,
				},
				TriggerChan:          tcpChannel,
				KamajiNamespace:      managerNamespace,
				KamajiServiceAccount: managerServiceAccountName,
				KamajiService:        managerServiceName,
				KamajiMigrateImage:   migrateJobImage,
			}

			if err = reconciler.SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "Namespace")

				return err
			}

			if err = (&webhook.Freeze{}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to register webhook", "webhook", "Freeze")

				return err
			}

			if err = (&kamajiv1alpha1.DatastoreUsedSecret{}).SetupWithManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create indexer", "indexer", "DatastoreUsedSecret")

				return err
			}

			if err = (&kamajiv1alpha1.TenantControlPlaneStatusDataStore{}).SetupWithManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create indexer", "indexer", "TenantControlPlaneStatusDataStore")

				return err
			}

			if err = (&kamajiv1alpha1.TenantControlPlane{}).SetupWebhookWithManager(mgr, datastore); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "TenantControlPlane")

				return err
			}
			if err = (&kamajiv1alpha1.DataStore{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "DataStore")

				return err
			}

			if err = (&soot.Manager{
				MigrateCABundle:         webhookCABundle,
				MigrateServiceName:      managerServiceName,
				MigrateServiceNamespace: managerNamespace,
				AdminClient:             mgr.GetClient(),
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to set up soot manager")

				return err
			}

			if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up health check")

				return err
			}
			if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up ready check")

				return err
			}

			setupLog.Info("starting manager")
			if err = mgr.Start(ctx); err != nil {
				setupLog.Error(err, "problem running manager")

				return err
			}

			return nil
		},
	}
	// Setting zap logger
	zapfs := flag.NewFlagSet("zap", flag.ExitOnError)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(zapfs)
	cmd.Flags().AddGoFlagSet(zapfs)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	// Setting CLI flags
	cmd.Flags().StringVar(&metricsBindAddress, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&leaderElect, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().StringVar(&tmpDirectory, "tmp-directory", "/tmp/kamaji", "Directory which will be used to work with temporary files.")
	cmd.Flags().StringVar(&kineImage, "kine-image", "rancher/kine:v0.9.2-amd64", "Container image along with tag to use for the Kine sidecar container (used only if etcd-storage-type is set to one of kine strategies).")
	cmd.Flags().StringVar(&datastore, "datastore", "etcd", "The default DataStore that should be used by Kamaji to setup the required storage.")
	cmd.Flags().StringVar(&migrateJobImage, "migrate-image", fmt.Sprintf("clastix/kamaji:v%s", internal.GitTag), "Specify the container image to launch when a TenantControlPlane is migrated to a new datastore.")
	cmd.Flags().StringVar(&managerNamespace, "pod-namespace", os.Getenv("POD_NAMESPACE"), "The Kubernetes Namespace on which the Operator is running in, required for the TenantControlPlane migration jobs.")
	cmd.Flags().StringVar(&managerServiceName, "webhook-service-name", "kamaji-webhook-service", "The Kamaji webhook server Service name which is used to get validation webhooks, required for the TenantControlPlane migration jobs.")
	cmd.Flags().StringVar(&managerServiceAccountName, "serviceaccount-name", os.Getenv("SERVICE_ACCOUNT"), "The Kubernetes Namespace on which the Operator is running in, required for the TenantControlPlane migration jobs.")
	cmd.Flags().StringVar(&webhookCAPath, "webhook-ca-path", "/tmp/k8s-webhook-server/serving-certs/ca.crt", "Path to the Manager webhook server CA, required for the TenantControlPlane migration jobs.")

	cobra.OnInitialize(func() {
		viper.AutomaticEnv()
	})

	return cmd
}
