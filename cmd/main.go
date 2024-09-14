/*
Copyright 2024.

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

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/go-logr/zapr"
	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	amicontroller "github.com/ibeify/opsy-ami-operator/internal/controller/ami"
	"github.com/ibeify/opsy-ami-operator/pkg/cond"
	"github.com/ibeify/opsy-ami-operator/pkg/events"
	"github.com/ibeify/opsy-ami-operator/pkg/opsy"
	"github.com/ibeify/opsy-ami-operator/pkg/status"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	red     = "\033[31m"
	yellow  = "\033[33m"
	green   = "\033[32m"
	cyan    = "\033[36m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	reset   = "\033[0m"
)

func colorPrint(color, text string) {
	fmt.Print(color + text + reset)
}

var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	brandBanner = `
  ______                                   ______  __       __ ______       ______                                        __
 /      \                                 /      \|  \     /  \      \     /      \                                      |  \
|  ▓▓▓▓▓▓\ ______   _______ __    __     |  ▓▓▓▓▓▓\ ▓▓\   /  ▓▓\▓▓▓▓▓▓    |  ▓▓▓▓▓▓\ ______   ______   ______   ______  _| ▓▓_    ______   ______
| ▓▓  | ▓▓/      \ /       \  \  |  \    | ▓▓__| ▓▓ ▓▓▓\ /  ▓▓▓ | ▓▓      | ▓▓  | ▓▓/      \ /      \ /      \ |      \|   ▓▓ \  /      \ /      \
| ▓▓  | ▓▓  ▓▓▓▓▓▓\  ▓▓▓▓▓▓▓ ▓▓  | ▓▓    | ▓▓    ▓▓ ▓▓▓▓\  ▓▓▓▓ | ▓▓      | ▓▓  | ▓▓  ▓▓▓▓▓▓\  ▓▓▓▓▓▓\  ▓▓▓▓▓▓\ \▓▓▓▓▓▓\\▓▓▓▓▓▓ |  ▓▓▓▓▓▓\  ▓▓▓▓▓▓\
| ▓▓  | ▓▓ ▓▓  | ▓▓\▓▓    \| ▓▓  | ▓▓    | ▓▓▓▓▓▓▓▓ ▓▓\▓▓ ▓▓ ▓▓ | ▓▓      | ▓▓  | ▓▓ ▓▓  | ▓▓ ▓▓    ▓▓ ▓▓   \▓▓/      ▓▓ | ▓▓ __| ▓▓  | ▓▓ ▓▓   \▓▓
| ▓▓__/ ▓▓ ▓▓__/ ▓▓_\▓▓▓▓▓▓\ ▓▓__/ ▓▓    | ▓▓  | ▓▓ ▓▓ \▓▓▓| ▓▓_| ▓▓_     | ▓▓__/ ▓▓ ▓▓__/ ▓▓ ▓▓▓▓▓▓▓▓ ▓▓     |  ▓▓▓▓▓▓▓ | ▓▓|  \ ▓▓__/ ▓▓ ▓▓
 \▓▓    ▓▓ ▓▓    ▓▓       ▓▓\▓▓    ▓▓    | ▓▓  | ▓▓ ▓▓  \▓ | ▓▓   ▓▓ \     \▓▓    ▓▓ ▓▓    ▓▓\▓▓     \ ▓▓      \▓▓    ▓▓  \▓▓  ▓▓\▓▓    ▓▓ ▓▓
  \▓▓▓▓▓▓| ▓▓▓▓▓▓▓ \▓▓▓▓▓▓▓ _\▓▓▓▓▓▓▓     \▓▓   \▓▓\▓▓      \▓▓\▓▓▓▓▓▓      \▓▓▓▓▓▓| ▓▓▓▓▓▓▓  \▓▓▓▓▓▓▓\▓▓       \▓▓▓▓▓▓▓   \▓▓▓▓  \▓▓▓▓▓▓ \▓▓
         | ▓▓              |  \__| ▓▓                                              | ▓▓
         | ▓▓               \▓▓    ▓▓                                              | ▓▓
          \▓▓                \▓▓▓▓▓▓                                                \▓▓

			An Operator to automated AMI lifecycle management in Kubernetes.
`
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(amiv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {

	logo := true // hehehe :D

	if logo {
		lines := strings.Split(brandBanner, "\n")
		// colors := []string{red, red, yellow, green, cyan, blue, magenta, magenta, red, yellow, green, cyan, blue}
		colors := []string{cyan, cyan, cyan, cyan, cyan, cyan, cyan, cyan, cyan, cyan, cyan, cyan, cyan}

		for i, line := range lines {
			if i < len(colors) {
				colorPrint(colors[i], line+"\n")
			} else {
				fmt.Println(line)
			}
		}
	}

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	flag.Parse()

	config := zap.NewProductionConfig()
	config.Encoding = "console"
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.LevelEncoder(func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		var color string
		switch level {
		case zapcore.DebugLevel:
			color = magenta
		case zapcore.InfoLevel:
			color = green
		case zapcore.WarnLevel:
			color = yellow
		case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
			color = red // Red for errors
		default:
			color = reset // Default color (reset)
		}
		enc.AppendString(color + level.CapitalString() + "\033[0m")
	})
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	zapLogger, err := config.Build()
	if err != nil {
		panic(err)
	}

	coloredLog := zapr.NewLogger(zapLogger)
	defer zapLogger.Sync()
	ctrl.SetLogger(coloredLog)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "40b3c27f.opsy.dev",
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
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes client")
		os.Exit(1)
	}

	if err = (&amicontroller.PackerBuilderReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Clientset:    clientset,
		EventManager: events.New(mgr.GetEventRecorderFor("packerbuilder-controller")),
		Status:       status.New(mgr.GetClient()),
		Cond:         cond.New(mgr.GetClient()),
		OpsyRunner:   opsy.New(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PackerBuilder")
		os.Exit(1)
	}
	if err = (&amicontroller.AMIRefresherReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		EventManager: events.New(mgr.GetEventRecorderFor("ami-refresh-controller")),
		Status:       status.New(mgr.GetClient()),
		Cond:         cond.New(mgr.GetClient()),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AMIRefresher")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
