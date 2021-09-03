import dataclasses

from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.k8s_object_utils import ObjectRef, get_object_ref

RESULT_ANNOTATION = "validate-result.kluctl.io/"

@dataclasses.dataclass(frozen=True, eq=True)
class ValidateResultItem:
    ref: ObjectRef
    reason: str
    message: str

@dataclasses.dataclass
class ValidateResult:
    warnings: list = dataclasses.field(default_factory=list)
    errors: list = dataclasses.field(default_factory=list)
    results: list = dataclasses.field(default_factory=list)

def validate_object(o):
    result = ValidateResult()
    if "status" not in o:
        return result
    ref = get_object_ref(o)
    status = o["status"]

    class ValidateFailed(Exception):
        pass

    def add_error(reason, message):
        result.errors.append(ValidateResultItem(ref, reason=reason, message=message))

    def add_warning(reason, message):
        result.warnings.append(ValidateResultItem(ref, reason=reason, message=message))

    def find_conditions(type, do_error, do_raise):
        ret = []
        for c in status.get("conditions", []):
            if c["type"] == type:
                ret.append((c.get("status"), c.get("reason"), c.get("message")))
        if not ret and do_error:
            add_error("condition-not-found", "%s condition not in status" % type)
            if do_raise:
                raise ValidateFailed()
        return ret

    def get_condition(type, do_error, do_raise):
        c = find_conditions(type, do_error, do_raise)
        if not c:
            return None, None, None
        if len(c) != 1:
            add_error("condition-not-one", "%s condition found more then once" % type)
            if do_raise:
                raise ValidateFailed()
        return c[0]

    def get_field(field, do_error, do_raise, default=None):
        v = get_dict_value(status, field)
        if v is None and do_error:
            add_error("field-not-found", "%s field not in status or empty" % field)
            if do_raise:
                raise ValidateFailed()
        if v is None:
            return default
        return v

    def parse_int_or_percent(v):
        if isinstance(v, int):
            return v, False
        return int(v.replace("%", "")), True

    def value_from_int_or_percent(v, total):
        v, is_percent = parse_int_or_percent(v)
        if is_percent:
            v = v * total / 100
        return int(v)

    try:
        if o["kind"] == "Pod":
            s, r, m = get_condition("Ready", True, True)
            if s != "True":
                add_error(r or "not-ready", m or "Not ready")
        elif o["kind"] == "Job":
            s, r, m = get_condition("Failed", False, False)
            if s == "True":
                add_error(r or "failed", m or "N/A")
            else:
                s, r, m = get_condition("Complete", True, True)
                if s != "True":
                    add_error(r or "not-completed", m or "Not completed")
        elif o["kind"] in ["Deployment", "ZookeeperCluster"]:
            ready_replicas = get_field("readyReplicas", True, True)
            replicas = get_field("replicas", True, True)
            if ready_replicas < replicas:
                add_error("not-ready", "readyReplicas (%d) is less then replicas (%d)" % (ready_replicas, replicas))
        elif o["kind"] == "PersistentVolumeClaim":
            phase = get_field("phase", True, True)
            if phase != "Bound":
                add_error("not-bound", "Volume is not bound")
        elif o["kind"] == "Service":
            svc_type = get_dict_value(o, "spec.type")
            if svc_type != "ExternalName":
                if get_dict_value(o, "spec.clusterIP", "") == "":
                    add_error("no-cluster-ip", "Service does not have a cluster IP")
                elif svc_type == "LoadBalancer":
                    if len(get_dict_value(o, "spec.externalIPs", [])) == 0:
                        get_field("loadBalancer.ingress", True, True)
        elif o["kind"] == "DaemonSet":
            if get_dict_value(o, "spec.updateStrategy.type") == "RollingUpdate":
                updated_number_scheduled = get_field("updatedNumberScheduled", True, True)
                desired_number_scheduled = get_field("desiredNumberScheduled", True, True)
                if updated_number_scheduled != desired_number_scheduled:
                    add_error("not-ready", "DaemonSet is not ready. %d out of %d expected pods have been scheduled" % (updated_number_scheduled, desired_number_scheduled))
                else:
                    max_unavailable = get_dict_value(o, "spec.updateStrategy.maxUnavailable", 1)
                    try:
                        max_unavailable = value_from_int_or_percent(max_unavailable, desired_number_scheduled)
                    except:
                        max_unavailable = desired_number_scheduled
                    expected_ready = desired_number_scheduled - max_unavailable
                    number_ready = get_field("numberReady", True, True)
                    if number_ready < expected_ready:
                        add_error("not-ready", "DaemonSet is not ready. %d out of %d expected pods are ready" % (number_ready, expected_ready))
        elif o["kind"] == "CustomResourceDefinition":
            # This is based on how Helm check for ready CRDs.
            # See https://github.com/helm/helm/blob/249d1b5fb98541f5fb89ab11019b6060d6b169f1/pkg/kube/ready.go#L342
            s, r, m = get_condition("Established", False, False)
            if s != "True":
                s, r, m = get_condition("NamesAccepted", True, True)
                if s != "False":
                    add_error("not-ready", "CRD is not ready")
        elif o["kind"] == "StatefulSet":
            if get_dict_value(o, "spec.updateStrategy.type") == "RollingUpdate":
                partition = get_dict_value(o, "spec.updateStrategy.rollingUpdate.partition", 0)
                replicas = get_dict_value(o, "spec.replicas", 1)
                updated_replicas = get_field("updatedReplicas", True, True)
                expected_replicas = replicas - partition
                if updated_replicas != expected_replicas:
                    add_error("not-ready", "StatefulSet is not ready. %d out of %d expected pods have been scheduled" % (updated_replicas, expected_replicas))
                else:
                    ready_replicas = get_field("readyReplicas", True, True)
                    if ready_replicas != replicas:
                        add_error("not-ready", "StatefulSet is not ready. %d out of %d expected pods are ready" % (ready_replicas, replicas))
        elif o["kind"] == "Kafka":
            for s, r, m in find_conditions("Warning", False, False):
                add_warning(r or "warning", m or "N/A")
            s, r, m = get_condition("Ready", True, True)
            if s != "True":
                add_error(r or "not-ready", m or "N/A")
        elif o["kind"] == "Elasticsearch":
            phase = get_field("phase", True, True)
            if phase != "Ready":
                add_error("not-ready", "phase is %s" % phase)
            else:
                health = get_field("health", True, True)
                if health == "yellow":
                    add_warning("health-yellow", "health is yellow")
                elif health != "green":
                    add_error("not-ready", "health is %s" % health)
        elif o["kind"] == "Kibana":
            health = get_field("health", True, True)
            if health == "yellow":
                add_warning("health-yellow", "health is yellow")
            elif health != "green":
                add_error("not-ready", "health is %s" % health)
        elif o["kind"] == "postgresql":
            pstatus = get_field("PostgresClusterStatus", True, True)
            if pstatus != "Running":
                add_error("not-ready", "PostgresClusterStatus is %s" % pstatus)
    except ValidateFailed:
        pass

    for k, v in o["metadata"].get("annotations", {}).items():
        if not k.startswith(RESULT_ANNOTATION):
            continue
        result.results.append(ValidateResultItem(ref, reason=k, message=v))

    return result
