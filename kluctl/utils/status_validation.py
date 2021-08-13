import dataclasses

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
    if o["kind"] in ["Deployment", "StatefulSet", "ZookeeperCluster"]:
        if "readyReplicas" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="readyReplicas not in status yet"))
        elif "replicas" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="replicas not in status yet"))
        elif status["readyReplicas"] < status["replicas"]:
            result.errors.append(ValidateResultItem(ref, reason="not-ready", message="readyReplicas (%d) is less then replicas (%d)" % (status["readyReplicas"], status["replicas"])))
    elif o["kind"] == "DaemonSet":
        if "numberReady" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="numberReady not in status yet"))
        elif "desiredNumberScheduled" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="desiredNumberScheduled not in status yet"))
        elif status["numberReady"] < status["desiredNumberScheduled"]:
            result.errors.append(ValidateResultItem(ref, reason="not-ready", message="numberReady (%d) is less then desiredNumberScheduled (%d)" % (status["numberReady"], status["desiredNumberScheduled"])))
    elif o["kind"] == "Job":
        for c in status.get("conditions", []):
            if c["type"] == "Failed" and c["status"] == "True":
                result.errors.append(ValidateResultItem(ref, reason=c.get("reason", "failed"), message=c.get("message", "N/A")))
        if status.get("active", 0) != 0:
            result.errors.append(ValidateResultItem(ref, reason=status.get("reason", "not-completed"), message="Job not finished yet. active=%d" % status["active"]))
    elif o["kind"] == "Kafka":
        ready_found = False
        for c in status.get("conditions", []):
            if c["type"] == "Ready":
                ready_found = True
                if c["status"] != "True":
                    result.errors.append(ValidateResultItem(ref, reason=c.get("reason", "not-ready"), message=c.get("message", "N/A")))
            elif c["type"] == "Warning":
                result.warnings.append(ValidateResultItem(ref, reason=c.get("reason", "not-ready"), message=c.get("message", "N/A")))
        if not ready_found:
            result.errors.append(ValidateResultItem(ref, reason="not-readdy", message="Ready condition not found"))
    elif o["kind"] == "Elasticsearch":
        if "phase" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="phase not in status yet"))
        elif status["phase"] != "Ready":
            result.errors.append(ValidateResultItem(ref, reason="not-ready", message="phase is %s" % status["phase"]))
        elif "health" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="health not in status yet"))
        elif status["health"] == "yellow":
            result.warnings.append(ValidateResultItem(ref, reason="health-yellow", message="health is yellow"))
        elif status["health"] != "green":
            result.errors.append(ValidateResultItem(ref, reason="not-ready", message="health is %s" % status["health"]))
    elif o["kind"] == "Kibana":
        if "health" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="health not in status yet"))
        elif status["health"] == "yellow":
            result.warnings.append(ValidateResultItem(ref, reason="health-yellow", message="health is yellow"))
        elif status["health"] != "green":
            result.errors.append(ValidateResultItem(ref, reason="not-ready", message="health is %s" % status["health"]))
    elif o["kind"] == "postgresql":
        if "PostgresClusterStatus" not in status:
            result.errors.append(ValidateResultItem(ref, reason="field-not-found", message="PostgresClusterStatus not in status yet"))
        elif status["PostgresClusterStatus"] != "Running":
            result.errors.append(ValidateResultItem(ref, reason="not-ready", message="PostgresClusterStatus is %s" % status["PostgresClusterStatus"]))

    for k, v in o["metadata"].get("annotations", {}).items():
        if not k.startswith(RESULT_ANNOTATION):
            continue
        result.results.append(ValidateResultItem(ref, reason=k, message=v))

    return result
