package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Metrics subsystem and all keys used by the kluctldeployment controller.
const (
	KluctlDeploymentControllerSubsystem = "kluctldeployments"

	DeploymentIntervalKey  = "deployment_interval_seconds"
	DryRunEnabledKey       = "dry_run_enabled"
	LastObjectStatusKey    = "last_object_status"
	LastDeployStartTimeKey = "last_deploy_start_timestamp_seconds"
	PruneEnabledKey        = "prune_enabled"
	DeleteEnabledKey       = "delete_enabled"
	SourceSpecKey          = "source_spec"
	GitSourceSpecKey       = "git_source_spec"
	OciSourceSpecKey       = "oci_source_spec"
)

var (
	deploymentInterval = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      DeploymentIntervalKey,
		Help:      "The configured deployment interval of a single deployment.",
	}, []string{"namespace", "name"})

	dryRunEnabled = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      DryRunEnabledKey,
		Help:      "Is dry-run enabled for a single deployment.",
	}, []string{"namespace", "name"})

	lastObjectStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      LastObjectStatusKey,
		Help:      "Last object status of a single deployment.",
	}, []string{"namespace", "name"})

	lastDeployStartTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      LastDeployStartTimeKey,
		Help:      "Last start time of a single deployment.",
	}, []string{"namespace", "name"})

	pruneEnabled = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      PruneEnabledKey,
		Help:      "Is pruning enabled for a single deployment.",
	}, []string{"namespace", "name"})

	deleteEnabled = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      DeleteEnabledKey,
		Help:      "Is deletion enabled for a single deployment.",
	}, []string{"namespace", "name"})

	sourceSpec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      SourceSpecKey,
		Help:      "The configured source spec of a single deployment.",
	}, []string{"namespace", "name", "url", "path", "ref"})

	gitSourceSpec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      GitSourceSpecKey,
		Help:      "The configured git source spec of a single deployment.",
	}, []string{"namespace", "name", "url", "path", "ref"})

	ociSourceSpec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      OciSourceSpecKey,
		Help:      "The configured git source spec of a single deployment.",
	}, []string{"namespace", "name", "url", "path", "ref"})
)

func init() {
	metrics.Registry.MustRegister(deploymentInterval)
	metrics.Registry.MustRegister(dryRunEnabled)
	metrics.Registry.MustRegister(lastObjectStatus)
	metrics.Registry.MustRegister(lastDeployStartTime)
	metrics.Registry.MustRegister(pruneEnabled)
	metrics.Registry.MustRegister(deleteEnabled)
	metrics.Registry.MustRegister(sourceSpec)
}

func NewKluctlDeploymentInterval(namespace string, name string) prometheus.Gauge {
	return dryRunEnabled.WithLabelValues(namespace, name)
}

func NewKluctlDryRunEnabled(namespace string, name string) prometheus.Gauge {
	return dryRunEnabled.WithLabelValues(namespace, name)
}

func NewKluctlLastObjectStatus(namespace string, name string) prometheus.Gauge {
	return lastObjectStatus.WithLabelValues(namespace, name)
}

func NewKluctlLastDeployStartTime(namespace string, name string) prometheus.Gauge {
	return lastDeployStartTime.WithLabelValues(namespace, name)
}

func NewKluctlPruneEnabled(namespace string, name string) prometheus.Gauge {
	return pruneEnabled.WithLabelValues(namespace, name)
}

func NewKluctlDeleteEnabled(namespace string, name string) prometheus.Gauge {
	return deleteEnabled.WithLabelValues(namespace, name)
}

func NewKluctlSourceSpec(namespace string, name string, url string, path string, ref string) prometheus.Gauge {
	return sourceSpec.WithLabelValues(namespace, name, url, path, ref)
}

func NewKluctlGitSourceSpec(namespace string, name string, url string, path string, ref string) prometheus.Gauge {
	return gitSourceSpec.WithLabelValues(namespace, name, url, path, ref)
}

func NewKluctlOciSourceSpec(namespace string, name string, url string, path string, ref string) prometheus.Gauge {
	return ociSourceSpec.WithLabelValues(namespace, name, url, path, ref)
}
