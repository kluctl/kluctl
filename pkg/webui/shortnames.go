package webui

import (
	"strings"
)

// output of "kubectl api-resources"
const apiResourcesOutput = `
bindings                                                      v1                                          true         Binding
componentstatuses                         cs                  v1                                          false        ComponentStatus
configmaps                                cm                  v1                                          true         ConfigMap
endpoints                                 ep                  v1                                          true         Endpoints
events                                    ev                  v1                                          true         Event
limitranges                               limits              v1                                          true         LimitRange
namespaces                                ns                  v1                                          false        Namespace
nodes                                     no                  v1                                          false        Node
persistentvolumeclaims                    pvc                 v1                                          true         PersistentVolumeClaim
persistentvolumes                         pv                  v1                                          false        PersistentVolume
pods                                      po                  v1                                          true         Pod
podtemplates                                                  v1                                          true         PodTemplate
replicationcontrollers                    rc                  v1                                          true         ReplicationController
resourcequotas                            quota               v1                                          true         ResourceQuota
secrets                                                       v1                                          true         Secret
serviceaccounts                           sa                  v1                                          true         ServiceAccount
services                                  svc                 v1                                          true         Service
postgresqls                               pg                  acid.zalan.do/v1                            true         postgresql
challenges                                                    acme.cert-manager.io/v1                     true         Challenge
orders                                                        acme.cert-manager.io/v1                     true         Order
mutatingwebhookconfigurations                                 admissionregistration.k8s.io/v1             false        MutatingWebhookConfiguration
validatingwebhookconfigurations                               admissionregistration.k8s.io/v1             false        ValidatingWebhookConfiguration
agents                                    agent               agent.k8s.elastic.co/v1alpha1               true         Agent
customresourcedefinitions                 crd,crds            apiextensions.k8s.io/v1                     false        CustomResourceDefinition
apiservices                                                   apiregistration.k8s.io/v1                   false        APIService
apmservers                                apm                 apm.k8s.elastic.co/v1                       true         ApmServer
controllerrevisions                                           apps/v1                                     true         ControllerRevision
daemonsets                                ds                  apps/v1                                     true         DaemonSet
deployments                               deploy              apps/v1                                     true         Deployment
replicasets                               rs                  apps/v1                                     true         ReplicaSet
statefulsets                              sts                 apps/v1                                     true         StatefulSet
tokenreviews                                                  authentication.k8s.io/v1                    false        TokenReview
localsubjectaccessreviews                                     authorization.k8s.io/v1                     true         LocalSubjectAccessReview
selfsubjectaccessreviews                                      authorization.k8s.io/v1                     false        SelfSubjectAccessReview
selfsubjectrulesreviews                                       authorization.k8s.io/v1                     false        SelfSubjectRulesReview
subjectaccessreviews                                          authorization.k8s.io/v1                     false        SubjectAccessReview
horizontalpodautoscalers                  hpa                 autoscaling/v2                              true         HorizontalPodAutoscaler
elasticsearchautoscalers                  esa                 autoscaling.k8s.elastic.co/v1alpha1         true         ElasticsearchAutoscaler
cronjobs                                  cj                  batch/v1                                    true         CronJob
jobs                                                          batch/v1                                    true         Job
beats                                     beat                beat.k8s.elastic.co/v1beta1                 true         Beat
sealedsecrets                                                 bitnami.com/v1alpha1                        true         SealedSecret
certificaterequests                       cr,crs              cert-manager.io/v1                          true         CertificateRequest
certificates                              cert,certs          cert-manager.io/v1                          true         Certificate
clusterissuers                                                cert-manager.io/v1                          false        ClusterIssuer
issuers                                                       cert-manager.io/v1                          true         Issuer
certificatesigningrequests                csr                 certificates.k8s.io/v1                      false        CertificateSigningRequest
ciliumclusterwidenetworkpolicies          ccnp                cilium.io/v2                                false        CiliumClusterwideNetworkPolicy
ciliumendpoints                           cep,ciliumep        cilium.io/v2                                true         CiliumEndpoint
ciliumexternalworkloads                   cew                 cilium.io/v2                                false        CiliumExternalWorkload
ciliumidentities                          ciliumid            cilium.io/v2                                false        CiliumIdentity
ciliumnetworkpolicies                     cnp,ciliumnp        cilium.io/v2                                true         CiliumNetworkPolicy
ciliumnodes                               cn,ciliumn          cilium.io/v2                                false        CiliumNode
configs                                                       config.gatekeeper.sh/v1alpha1               true         Config
k8sallowedingressclasses                                      constraints.gatekeeper.sh/v1beta1           false        K8sAllowedIngressClasses
k8spspallowedusers                                            constraints.gatekeeper.sh/v1beta1           false        K8sPSPAllowedUsers
k8spspallowprivilegeescalationcontainer                       constraints.gatekeeper.sh/v1beta1           false        K8sPSPAllowPrivilegeEscalationContainer
k8spspapparmor                                                constraints.gatekeeper.sh/v1beta1           false        K8sPSPAppArmor
k8spspcapabilities                                            constraints.gatekeeper.sh/v1beta1           false        K8sPSPCapabilities
k8spspflexvolumes                                             constraints.gatekeeper.sh/v1beta1           false        K8sPSPFlexVolumes
k8spspforbiddensysctls                                        constraints.gatekeeper.sh/v1beta1           false        K8sPSPForbiddenSysctls
k8spspfsgroup                                                 constraints.gatekeeper.sh/v1beta1           false        K8sPSPFSGroup
k8spsphostfilesystem                                          constraints.gatekeeper.sh/v1beta1           false        K8sPSPHostFilesystem
k8spsphostnamespace                                           constraints.gatekeeper.sh/v1beta1           false        K8sPSPHostNamespace
k8spsphostnetworkingports                                     constraints.gatekeeper.sh/v1beta1           false        K8sPSPHostNetworkingPorts
k8spspprivilegedcontainer                                     constraints.gatekeeper.sh/v1beta1           false        K8sPSPPrivilegedContainer
k8spspprocmount                                               constraints.gatekeeper.sh/v1beta1           false        K8sPSPProcMount
k8spspreadonlyrootfilesystem                                  constraints.gatekeeper.sh/v1beta1           false        K8sPSPReadOnlyRootFilesystem
k8spspseccomp                                                 constraints.gatekeeper.sh/v1beta1           false        K8sPSPSeccomp
k8spspselinuxv2                                               constraints.gatekeeper.sh/v1beta1           false        K8sPSPSELinuxV2
k8spspvolumetypes                                             constraints.gatekeeper.sh/v1beta1           false        K8sPSPVolumeTypes
leases                                                        coordination.k8s.io/v1                      true         Lease
strimzipodsets                            sps                 core.strimzi.io/v1beta2                     true         StrimziPodSet
eniconfigs                                                    crd.k8s.amazonaws.com/v1alpha1              false        ENIConfig
endpointslices                                                discovery.k8s.io/v1                         true         EndpointSlice
elasticsearches                           es                  elasticsearch.k8s.elastic.co/v1             true         Elasticsearch
ingressclassparams                                            elbv2.k8s.aws/v1beta1                       false        IngressClassParams
targetgroupbindings                                           elbv2.k8s.aws/v1beta1                       true         TargetGroupBinding
enterprisesearches                        ent                 enterprisesearch.k8s.elastic.co/v1          true         EnterpriseSearch
events                                    ev                  events.k8s.io/v1                            true         Event
expansiontemplate                                             expansion.gatekeeper.sh/v1alpha1            false        ExpansionTemplate
providers                                                     externaldata.gatekeeper.sh/v1beta1          false        Provider
flowschemas                                                   flowcontrol.apiserver.k8s.io/v1beta2        false        FlowSchema
prioritylevelconfigurations                                   flowcontrol.apiserver.k8s.io/v1beta2        false        PriorityLevelConfiguration
kluctldeployments                                             flux.kluctl.io/v1alpha1                     true         KluctlDeployment
jaegers                                                       jaegertracing.io/v1                         true         Jaeger
kafkabridges                              kb                  kafka.strimzi.io/v1beta2                    true         KafkaBridge
kafkaconnectors                           kctr                kafka.strimzi.io/v1beta2                    true         KafkaConnector
kafkaconnects                             kc                  kafka.strimzi.io/v1beta2                    true         KafkaConnect
kafkamirrormaker2s                        kmm2                kafka.strimzi.io/v1beta2                    true         KafkaMirrorMaker2
kafkamirrormakers                         kmm                 kafka.strimzi.io/v1beta2                    true         KafkaMirrorMaker
kafkarebalances                           kr                  kafka.strimzi.io/v1beta2                    true         KafkaRebalance
kafkas                                    k                   kafka.strimzi.io/v1beta2                    true         Kafka
kafkatopics                               kt                  kafka.strimzi.io/v1beta2                    true         KafkaTopic
kafkausers                                ku                  kafka.strimzi.io/v1beta2                    true         KafkaUser
kibanas                                   kb                  kibana.k8s.elastic.co/v1                    true         Kibana
elasticmapsservers                        ems                 maps.k8s.elastic.co/v1alpha1                true         ElasticMapsServer
nodes                                                         metrics.k8s.io/v1beta1                      false        NodeMetrics
pods                                                          metrics.k8s.io/v1beta1                      true         PodMetrics
tenants                                   tenant              minio.min.io/v2                             true         Tenant
alertmanagerconfigs                       amcfg               monitoring.coreos.com/v1alpha1              true         AlertmanagerConfig
alertmanagers                             am                  monitoring.coreos.com/v1                    true         Alertmanager
podmonitors                               pmon                monitoring.coreos.com/v1                    true         PodMonitor
probes                                    prb                 monitoring.coreos.com/v1                    true         Probe
prometheuses                              prom                monitoring.coreos.com/v1                    true         Prometheus
prometheusrules                           promrule            monitoring.coreos.com/v1                    true         PrometheusRule
servicemonitors                           smon                monitoring.coreos.com/v1                    true         ServiceMonitor
thanosrulers                              ruler               monitoring.coreos.com/v1                    true         ThanosRuler
assign                                                        mutations.gatekeeper.sh/v1                  false        Assign
assignmetadata                                                mutations.gatekeeper.sh/v1                  false        AssignMetadata
modifyset                                                     mutations.gatekeeper.sh/v1                  false        ModifySet
ingressclasses                                                networking.k8s.io/v1                        false        IngressClass
ingresses                                 ing                 networking.k8s.io/v1                        true         Ingress
networkpolicies                           netpol              networking.k8s.io/v1                        true         NetworkPolicy
runtimeclasses                                                node.k8s.io/v1                              false        RuntimeClass
poddisruptionbudgets                      pdb                 policy/v1                                   true         PodDisruptionBudget
podsecuritypolicies                       psp                 policy/v1beta1                              false        PodSecurityPolicy
clusterrolebindings                                           rbac.authorization.k8s.io/v1                false        ClusterRoleBinding
clusterroles                                                  rbac.authorization.k8s.io/v1                false        ClusterRole
rolebindings                                                  rbac.authorization.k8s.io/v1                true         RoleBinding
roles                                                         rbac.authorization.k8s.io/v1                true         Role
dbclusterparametergroups                                      rds.services.k8s.aws/v1alpha1               true         DBClusterParameterGroup
dbclusters                                                    rds.services.k8s.aws/v1alpha1               true         DBCluster
dbinstances                                                   rds.services.k8s.aws/v1alpha1               true         DBInstance
dbparametergroups                                             rds.services.k8s.aws/v1alpha1               true         DBParameterGroup
dbproxies                                                     rds.services.k8s.aws/v1alpha1               true         DBProxy
dbsecuritygroups                                              rds.services.k8s.aws/v1alpha1               true         DBSecurityGroup
dbsubnetgroups                                                rds.services.k8s.aws/v1alpha1               true         DBSubnetGroup
globalclusters                                                rds.services.k8s.aws/v1alpha1               true         GlobalCluster
buckets                                                       s3.services.k8s.aws/v1alpha1                true         Bucket
priorityclasses                           pc                  scheduling.k8s.io/v1                        false        PriorityClass
basicauths                                                    secretgenerator.mittwald.de/v1alpha1        true         BasicAuth
sshkeypairs                                                   secretgenerator.mittwald.de/v1alpha1        true         SSHKeyPair
stringsecrets                                                 secretgenerator.mittwald.de/v1alpha1        true         StringSecret
adoptedresources                                              services.k8s.aws/v1alpha1                   true         AdoptedResource
fieldexports                                                  services.k8s.aws/v1alpha1                   true         FieldExport
volumesnapshotclasses                     vsclass,vsclasses   snapshot.storage.k8s.io/v1                  false        VolumeSnapshotClass
volumesnapshotcontents                    vsc,vscs            snapshot.storage.k8s.io/v1                  false        VolumeSnapshotContent
volumesnapshots                           vs                  snapshot.storage.k8s.io/v1                  true         VolumeSnapshot
stackconfigpolicies                       scp                 stackconfigpolicy.k8s.elastic.co/v1alpha1   true         StackConfigPolicy
constraintpodstatuses                                         status.gatekeeper.sh/v1beta1                true         ConstraintPodStatus
constrainttemplatepodstatuses                                 status.gatekeeper.sh/v1beta1                true         ConstraintTemplatePodStatus
mutatorpodstatuses                                            status.gatekeeper.sh/v1beta1                true         MutatorPodStatus
csidrivers                                                    storage.k8s.io/v1                           false        CSIDriver
csinodes                                                      storage.k8s.io/v1                           false        CSINode
csistoragecapacities                                          storage.k8s.io/v1beta1                      true         CSIStorageCapacity
storageclasses                            sc                  storage.k8s.io/v1                           false        StorageClass
volumeattachments                                             storage.k8s.io/v1                           false        VolumeAttachment
constrainttemplates                                           templates.gatekeeper.sh/v1                  false        ConstraintTemplate
githubcomments                                                templates.kluctl.io/v1alpha1                true         GithubComment
gitlabcomments                                                templates.kluctl.io/v1alpha1                true         GitlabComment
gitprojectors                                                 templates.kluctl.io/v1alpha1                true         GitProjector
listgithubpullrequests                                        templates.kluctl.io/v1alpha1                true         ListGithubPullRequests
listgitlabmergerequests                                       templates.kluctl.io/v1alpha1                true         ListGitlabMergeRequests
objecthandlers                                                templates.kluctl.io/v1alpha1                true         ObjectHandler
objecttemplates                                               templates.kluctl.io/v1alpha1                true         ObjectTemplate
texttemplates                                                 templates.kluctl.io/v1alpha1                true         TextTemplate
backuprepositories                                            velero.io/v1                                true         BackupRepository
backups                                                       velero.io/v1                                true         Backup
backupstoragelocations                    bsl                 velero.io/v1                                true         BackupStorageLocation
deletebackuprequests                                          velero.io/v1                                true         DeleteBackupRequest
downloadrequests                                              velero.io/v1                                true         DownloadRequest
podvolumebackups                                              velero.io/v1                                true         PodVolumeBackup
podvolumerestores                                             velero.io/v1                                true         PodVolumeRestore
resticrepositories                                            velero.io/v1                                true         ResticRepository
restores                                                      velero.io/v1                                true         Restore
schedules                                                     velero.io/v1                                true         Schedule
serverstatusrequests                      ssr                 velero.io/v1                                true         ServerStatusRequest
volumesnapshotlocations                   vsl                 velero.io/v1                                true         VolumeSnapshotLocation
securitygrouppolicies                     sgp                 vpcresources.k8s.aws/v1beta1                true         SecurityGroupPolicy
`

type ShortName struct {
	Group     string `json:"group,omitempty"`
	Kind      string `json:"kind"`
	ShortName string `json:"shortName"`
}

var shortNames []ShortName

func init() {
	for _, l := range strings.Split(apiResourcesOutput, "\n") {
		if l == "" {
			continue
		}
		s := strings.Fields(l)
		if len(s) == 5 {
			sns := strings.Split(s[1], ",")
			apiVersion := s[2]
			kind := s[4]

			group := ""
			s2 := strings.Split(apiVersion, "/")
			if len(s2) == 2 {
				group = s2[0]
			}

			shortNames = append(shortNames, ShortName{
				Group:     group,
				Kind:      kind,
				ShortName: sns[0],
			})
		}
	}
}

func GetShortNames() []ShortName {
	return shortNames
}
