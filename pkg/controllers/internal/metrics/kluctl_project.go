package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	DeploymentDurationKey     = "deployment_duration_seconds"
	NumberOfChangedObjectsKey = "number_of_changed_objects"
	NumberOfDeletedObjectsKey = "number_of_deleted_objects"
	NumberOfErrorsKey         = "number_of_errors"
	NumberOfOrphanObjectsKey  = "number_of_orphan_objects"
	NumberOfWarningsKey       = "number_of_warnings"
	PruneDurationKey          = "prune_duration_seconds"
	DeleteDurationKey         = "delete_duration_seconds"
	ValidateDurationKey       = "validate_duration_seconds"
)

var (
	deploymentDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      DeploymentDurationKey,
		Help:      "How long a single deployment takes in seconds.",
	}, []string{"namespace", "name", "mode"})

	numberOfChangedObjects = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      NumberOfChangedObjectsKey,
		Help:      "How many objects have been changed by a single project.",
	}, []string{"namespace", "name"})

	numberOfDeletedObjects = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      NumberOfDeletedObjectsKey,
		Help:      "How many things has been deleted by a single project.",
	}, []string{"namespace", "name"})

	numberOfErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      NumberOfErrorsKey,
		Help:      "How many errors are related to a single project.",
	}, []string{"namespace", "name", "action"})

	numberOfOrphanObjects = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      NumberOfOrphanObjectsKey,
		Help:      "How many orphans are related to a single project.",
	}, []string{"namespace", "name"})

	numberOfWarnings = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      NumberOfWarningsKey,
		Help:      "How many warnings are related to a single project.",
	}, []string{"namespace", "name", "action"})

	pruneDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      PruneDurationKey,
		Help:      "How long a single prune takes in seconds.",
	}, []string{"namespace", "name"})

	deleteDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      DeleteDurationKey,
		Help:      "How long a single delete takes in seconds.",
	}, []string{"namespace", "name"})

	validateDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: KluctlDeploymentControllerSubsystem,
		Name:      ValidateDurationKey,
		Help:      "How long a single validate takes in seconds.",
	}, []string{"namespace", "name"})
)

func init() {
	metrics.Registry.MustRegister(deploymentDuration)
	metrics.Registry.MustRegister(numberOfChangedObjects)
	metrics.Registry.MustRegister(numberOfDeletedObjects)
	metrics.Registry.MustRegister(numberOfErrors)
	metrics.Registry.MustRegister(numberOfOrphanObjects)
	metrics.Registry.MustRegister(numberOfWarnings)
	metrics.Registry.MustRegister(pruneDuration)
	metrics.Registry.MustRegister(deleteDuration)
	metrics.Registry.MustRegister(validateDuration)
}

func NewKluctlDeploymentDuration(namespace string, name string, mode string) prometheus.Observer {
	return deploymentDuration.WithLabelValues(namespace, name, mode)
}

func NewKluctlNumberOfChangedObjects(namespace string, name string) prometheus.Gauge {
	return numberOfChangedObjects.WithLabelValues(namespace, name)
}

func NewKluctlNumberOfDeletedObjects(namespace string, name string) prometheus.Gauge {
	return numberOfDeletedObjects.WithLabelValues(namespace, name)
}

func NewKluctlNumberOfErrors(namespace string, name string, action string) prometheus.Gauge {
	return numberOfErrors.WithLabelValues(namespace, name, action)
}

func NewKluctlNumberOfOrphanObjects(namespace string, name string) prometheus.Gauge {
	return numberOfOrphanObjects.WithLabelValues(namespace, name)
}

func NewKluctlNumberOfWarnings(namespace string, name string, action string) prometheus.Gauge {
	return numberOfWarnings.WithLabelValues(namespace, name, action)
}

func NewKluctlPruneDuration(namespace string, name string) prometheus.Observer {
	return pruneDuration.WithLabelValues(namespace, name)
}

func NewKluctlDeleteDuration(namespace string, name string) prometheus.Observer {
	return deleteDuration.WithLabelValues(namespace, name)
}

func NewKluctlValidateDuration(namespace string, name string) prometheus.Observer {
	return validateDuration.WithLabelValues(namespace, name)
}
