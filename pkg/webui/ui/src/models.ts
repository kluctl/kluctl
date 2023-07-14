/* Do not change, this code is generated from Golang structs */

import { GitRef } from './models-static'

export class DeploymentError {
    ref: ObjectRef;
    message: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.ref = this.convertValues(source["ref"], ObjectRef);
        this.message = source["message"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class Change {
    type: string;
    jsonPath: string;
    oldValue?: any;
    newValue?: any;
    unifiedDiff?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.type = source["type"];
        this.jsonPath = source["jsonPath"];
        this.oldValue = source["oldValue"];
        this.newValue = source["newValue"];
        this.unifiedDiff = source["unifiedDiff"];
    }
}
export class ResultObject {
    ref: ObjectRef;
    changes?: Change[];
    new?: boolean;
    orphan?: boolean;
    deleted?: boolean;
    hook?: boolean;
    rendered?: any;
    remote?: any;
    applied?: any;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.ref = this.convertValues(source["ref"], ObjectRef);
        this.changes = this.convertValues(source["changes"], Change);
        this.new = source["new"];
        this.orphan = source["orphan"];
        this.deleted = source["deleted"];
        this.hook = source["hook"];
        this.rendered = source["rendered"];
        this.remote = source["remote"];
        this.applied = source["applied"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class IgnoreForDiffItemConfig {
    fieldPath?: string[];
    fieldPathRegex?: string[];
    group?: string;
    kind?: string;
    name?: string;
    namespace?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.fieldPath = source["fieldPath"];
        this.fieldPathRegex = source["fieldPathRegex"];
        this.group = source["group"];
        this.kind = source["kind"];
        this.name = source["name"];
        this.namespace = source["namespace"];
    }
}
export class HelmChartConfig {
    repo?: string;
    path?: string;
    credentialsId?: string;
    chartName?: string;
    chartVersion?: string;
    updateConstraints?: string;
    releaseName: string;
    namespace?: string;
    output?: string;
    skipCRDs?: boolean;
    skipUpdate?: boolean;
    skipPrePull?: boolean;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.repo = source["repo"];
        this.path = source["path"];
        this.credentialsId = source["credentialsId"];
        this.chartName = source["chartName"];
        this.chartVersion = source["chartVersion"];
        this.updateConstraints = source["updateConstraints"];
        this.releaseName = source["releaseName"];
        this.namespace = source["namespace"];
        this.output = source["output"];
        this.skipCRDs = source["skipCRDs"];
        this.skipUpdate = source["skipUpdate"];
        this.skipPrePull = source["skipPrePull"];
    }
}
export class DeleteObjectItemConfig {
    group?: string;
    kind?: string;
    name: string;
    namespace?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.group = source["group"];
        this.kind = source["kind"];
        this.name = source["name"];
        this.namespace = source["namespace"];
    }
}
export class GitProject {
    url: string;
    ref?: GitRef;
    subDir?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.url = source["url"];
        this.ref = new GitRef(source["ref"]);
        this.subDir = source["subDir"];
    }
}
export class DeploymentItemConfig {
    path?: string;
    include?: string;
    git?: GitProject;
    tags?: string[];
    barrier?: boolean;
    message?: string;
    waitReadiness?: boolean;
    vars?: VarsSource[];
    skipDeleteIfTags?: boolean;
    onlyRender?: boolean;
    alwaysDeploy?: boolean;
    deleteObjects?: DeleteObjectItemConfig[];
    when?: string;
    renderedHelmChartConfig?: HelmChartConfig;
    renderedObjects?: ObjectRef[];
    renderedInclude?: DeploymentProjectConfig;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.path = source["path"];
        this.include = source["include"];
        this.git = this.convertValues(source["git"], GitProject);
        this.tags = source["tags"];
        this.barrier = source["barrier"];
        this.message = source["message"];
        this.waitReadiness = source["waitReadiness"];
        this.vars = this.convertValues(source["vars"], VarsSource);
        this.skipDeleteIfTags = source["skipDeleteIfTags"];
        this.onlyRender = source["onlyRender"];
        this.alwaysDeploy = source["alwaysDeploy"];
        this.deleteObjects = this.convertValues(source["deleteObjects"], DeleteObjectItemConfig);
        this.when = source["when"];
        this.renderedHelmChartConfig = this.convertValues(source["renderedHelmChartConfig"], HelmChartConfig);
        this.renderedObjects = this.convertValues(source["renderedObjects"], ObjectRef);
        this.renderedInclude = this.convertValues(source["renderedInclude"], DeploymentProjectConfig);
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class SealedSecretsConfig {
    outputPattern?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.outputPattern = source["outputPattern"];
    }
}
export class VarsSourceVault {
    address: string;
    path: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.address = source["address"];
        this.path = source["path"];
    }
}
export class VarsSourceAwsSecretsManager {
    secretName: string;
    region?: string;
    profile?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.secretName = source["secretName"];
        this.region = source["region"];
        this.profile = source["profile"];
    }
}
export class VarsSourceHttp {
    url?: string;
    method?: string;
    body?: string;
    headers?: {[key: string]: string};
    jsonPath?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.url = source["url"];
        this.method = source["method"];
        this.body = source["body"];
        this.headers = source["headers"];
        this.jsonPath = source["jsonPath"];
    }
}
export class VarsSourceClusterConfigMapOrSecret {
    name?: string;
    labels?: {[key: string]: string};
    namespace: string;
    key: string;
    targetPath?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.name = source["name"];
        this.labels = source["labels"];
        this.namespace = source["namespace"];
        this.key = source["key"];
        this.targetPath = source["targetPath"];
    }
}
export class VarsSourceGit {
    url: string;
    ref?: GitRef;
    path: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.url = source["url"];
        this.ref = new GitRef(source["ref"]);
        this.path = source["path"];
    }
}
export class VarsSource {
    ignoreMissing?: boolean;
    noOverride?: boolean;
    values?: any;
    file?: string;
    git?: VarsSourceGit;
    clusterConfigMap?: VarsSourceClusterConfigMapOrSecret;
    clusterSecret?: VarsSourceClusterConfigMapOrSecret;
    systemEnvVars?: any;
    http?: VarsSourceHttp;
    awsSecretsManager?: VarsSourceAwsSecretsManager;
    vault?: VarsSourceVault;
    when?: string;
    renderedVars?: any;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.ignoreMissing = source["ignoreMissing"];
        this.noOverride = source["noOverride"];
        this.values = source["values"];
        this.file = source["file"];
        this.git = this.convertValues(source["git"], VarsSourceGit);
        this.clusterConfigMap = this.convertValues(source["clusterConfigMap"], VarsSourceClusterConfigMapOrSecret);
        this.clusterSecret = this.convertValues(source["clusterSecret"], VarsSourceClusterConfigMapOrSecret);
        this.systemEnvVars = source["systemEnvVars"];
        this.http = this.convertValues(source["http"], VarsSourceHttp);
        this.awsSecretsManager = this.convertValues(source["awsSecretsManager"], VarsSourceAwsSecretsManager);
        this.vault = this.convertValues(source["vault"], VarsSourceVault);
        this.when = source["when"];
        this.renderedVars = source["renderedVars"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class DeploymentProjectConfig {
    vars?: VarsSource[];
    sealedSecrets?: SealedSecretsConfig;
    when?: string;
    deployments?: DeploymentItemConfig[];
    commonLabels?: {[key: string]: string};
    commonAnnotations?: {[key: string]: string};
    overrideNamespace?: string;
    tags?: string[];
    ignoreForDiff?: IgnoreForDiffItemConfig[];

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.vars = this.convertValues(source["vars"], VarsSource);
        this.sealedSecrets = this.convertValues(source["sealedSecrets"], SealedSecretsConfig);
        this.when = source["when"];
        this.deployments = this.convertValues(source["deployments"], DeploymentItemConfig);
        this.commonLabels = source["commonLabels"];
        this.commonAnnotations = source["commonAnnotations"];
        this.overrideNamespace = source["overrideNamespace"];
        this.tags = source["tags"];
        this.ignoreForDiff = this.convertValues(source["ignoreForDiff"], IgnoreForDiffItemConfig);
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ClusterInfo {
    clusterId: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.clusterId = source["clusterId"];
    }
}
export class GitInfo {
    url?: string;
    ref?: GitRef;
    subDir: string;
    commit: string;
    dirty: boolean;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.url = source["url"];
        this.ref = new GitRef(source["ref"]);
        this.subDir = source["subDir"];
        this.commit = source["commit"];
        this.dirty = source["dirty"];
    }
}
export class KluctlDeploymentInfo {
    name: string;
    namespace: string;
    clusterId: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.name = source["name"];
        this.namespace = source["namespace"];
        this.clusterId = source["clusterId"];
    }
}
export class CommandInfo {
    initiator: string;
    startTime: string;
    endTime: string;
    kluctlDeployment?: KluctlDeploymentInfo;
    command?: string;
    target?: string;
    targetNameOverride?: string;
    contextOverride?: string;
    args?: any;
    images?: FixedImage[];
    dryRun?: boolean;
    noWait?: boolean;
    forceApply?: boolean;
    replaceOnError?: boolean;
    forceReplaceOnError?: boolean;
    abortOnError?: boolean;
    includeTags?: string[];
    excludeTags?: string[];
    includeDeploymentDirs?: string[];
    excludeDeploymentDirs?: string[];

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.initiator = source["initiator"];
        this.startTime = source["startTime"];
        this.endTime = source["endTime"];
        this.kluctlDeployment = this.convertValues(source["kluctlDeployment"], KluctlDeploymentInfo);
        this.command = source["command"];
        this.target = source["target"];
        this.targetNameOverride = source["targetNameOverride"];
        this.contextOverride = source["contextOverride"];
        this.args = source["args"];
        this.images = this.convertValues(source["images"], FixedImage);
        this.dryRun = source["dryRun"];
        this.noWait = source["noWait"];
        this.forceApply = source["forceApply"];
        this.replaceOnError = source["replaceOnError"];
        this.forceReplaceOnError = source["forceReplaceOnError"];
        this.abortOnError = source["abortOnError"];
        this.includeTags = source["includeTags"];
        this.excludeTags = source["excludeTags"];
        this.includeDeploymentDirs = source["includeDeploymentDirs"];
        this.excludeDeploymentDirs = source["excludeDeploymentDirs"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ObjectRef {
    group?: string;
    version?: string;
    kind: string;
    name: string;
    namespace?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.group = source["group"];
        this.version = source["version"];
        this.kind = source["kind"];
        this.name = source["name"];
        this.namespace = source["namespace"];
    }
}
export class FixedImage {
    image: string;
    resultImage: string;
    deployedImage?: string;
    namespace?: string;
    object?: ObjectRef;
    deployment?: string;
    container?: string;
    deployTags?: string[];
    deploymentDir?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.image = source["image"];
        this.resultImage = source["resultImage"];
        this.deployedImage = source["deployedImage"];
        this.namespace = source["namespace"];
        this.object = this.convertValues(source["object"], ObjectRef);
        this.deployment = source["deployment"];
        this.container = source["container"];
        this.deployTags = source["deployTags"];
        this.deploymentDir = source["deploymentDir"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class SealingConfig {
    args?: any;
    secretSets?: string[];
    certFile?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.args = source["args"];
        this.secretSets = source["secretSets"];
        this.certFile = source["certFile"];
    }
}
export class Target {
    name: string;
    context?: string;
    args?: any;
    sealingConfig?: SealingConfig;
    images?: FixedImage[];
    discriminator?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.name = source["name"];
        this.context = source["context"];
        this.args = source["args"];
        this.sealingConfig = this.convertValues(source["sealingConfig"], SealingConfig);
        this.images = this.convertValues(source["images"], FixedImage);
        this.discriminator = source["discriminator"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class TargetKey {
    targetName?: string;
    clusterId: string;
    discriminator?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.targetName = source["targetName"];
        this.clusterId = source["clusterId"];
        this.discriminator = source["discriminator"];
    }
}
export class ProjectKey {
    gitRepoKey?: string;
    subDir?: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.gitRepoKey = source["gitRepoKey"];
        this.subDir = source["subDir"];
    }
}
export class CommandResult {
    id: string;
    projectKey: ProjectKey;
    targetKey: TargetKey;
    target: Target;
    command?: CommandInfo;
    gitInfo?: GitInfo;
    clusterInfo: ClusterInfo;
    deployment?: DeploymentProjectConfig;
    objects?: ResultObject[];
    errors?: DeploymentError[];
    warnings?: DeploymentError[];
    seenImages?: FixedImage[];

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.id = source["id"];
        this.projectKey = this.convertValues(source["projectKey"], ProjectKey);
        this.targetKey = this.convertValues(source["targetKey"], TargetKey);
        this.target = this.convertValues(source["target"], Target);
        this.command = this.convertValues(source["command"], CommandInfo);
        this.gitInfo = this.convertValues(source["gitInfo"], GitInfo);
        this.clusterInfo = this.convertValues(source["clusterInfo"], ClusterInfo);
        this.deployment = this.convertValues(source["deployment"], DeploymentProjectConfig);
        this.objects = this.convertValues(source["objects"], ResultObject);
        this.errors = this.convertValues(source["errors"], DeploymentError);
        this.warnings = this.convertValues(source["warnings"], DeploymentError);
        this.seenImages = this.convertValues(source["seenImages"], FixedImage);
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class CommandResultSummary {
    id: string;
    projectKey: ProjectKey;
    targetKey: TargetKey;
    target: Target;
    commandInfo: CommandInfo;
    gitInfo?: GitInfo;
    clusterInfo?: ClusterInfo;
    renderedObjects: number;
    remoteObjects: number;
    appliedObjects: number;
    appliedHookObjects: number;
    newObjects: number;
    changedObjects: number;
    orphanObjects: number;
    deletedObjects: number;
    errors: DeploymentError[];
    warnings: DeploymentError[];
    totalChanges: number;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.id = source["id"];
        this.projectKey = this.convertValues(source["projectKey"], ProjectKey);
        this.targetKey = this.convertValues(source["targetKey"], TargetKey);
        this.target = this.convertValues(source["target"], Target);
        this.commandInfo = this.convertValues(source["commandInfo"], CommandInfo);
        this.gitInfo = this.convertValues(source["gitInfo"], GitInfo);
        this.clusterInfo = this.convertValues(source["clusterInfo"], ClusterInfo);
        this.renderedObjects = source["renderedObjects"];
        this.remoteObjects = source["remoteObjects"];
        this.appliedObjects = source["appliedObjects"];
        this.appliedHookObjects = source["appliedHookObjects"];
        this.newObjects = source["newObjects"];
        this.changedObjects = source["changedObjects"];
        this.orphanObjects = source["orphanObjects"];
        this.deletedObjects = source["deletedObjects"];
        this.errors = this.convertValues(source["errors"], DeploymentError);
        this.warnings = this.convertValues(source["warnings"], DeploymentError);
        this.totalChanges = source["totalChanges"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ValidateResultEntry {
    ref: ObjectRef;
    annotation: string;
    message: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.ref = this.convertValues(source["ref"], ObjectRef);
        this.annotation = source["annotation"];
        this.message = source["message"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ValidateResult {
    id: string;
    projectKey: ProjectKey;
    targetKey: TargetKey;
    kluctlDeployment?: KluctlDeploymentInfo;
    startTime: string;
    endTime: string;
    ready: boolean;
    warnings?: DeploymentError[];
    errors?: DeploymentError[];
    results?: ValidateResultEntry[];

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.id = source["id"];
        this.projectKey = this.convertValues(source["projectKey"], ProjectKey);
        this.targetKey = this.convertValues(source["targetKey"], TargetKey);
        this.kluctlDeployment = this.convertValues(source["kluctlDeployment"], KluctlDeploymentInfo);
        this.startTime = source["startTime"];
        this.endTime = source["endTime"];
        this.ready = source["ready"];
        this.warnings = this.convertValues(source["warnings"], DeploymentError);
        this.errors = this.convertValues(source["errors"], DeploymentError);
        this.results = this.convertValues(source["results"], ValidateResultEntry);
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ValidateResultSummary {
    id: string;
    projectKey: ProjectKey;
    targetKey: TargetKey;
    kluctlDeployment?: KluctlDeploymentInfo;
    startTime: string;
    endTime: string;
    ready: boolean;
    warnings: number;
    errors: number;
    results: number;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.id = source["id"];
        this.projectKey = this.convertValues(source["projectKey"], ProjectKey);
        this.targetKey = this.convertValues(source["targetKey"], TargetKey);
        this.kluctlDeployment = this.convertValues(source["kluctlDeployment"], KluctlDeploymentInfo);
        this.startTime = source["startTime"];
        this.endTime = source["endTime"];
        this.ready = source["ready"];
        this.warnings = source["warnings"];
        this.errors = source["errors"];
        this.results = source["results"];
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ChangedObject {
    ref: ObjectRef;
    changes?: Change[];

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.ref = this.convertValues(source["ref"], ObjectRef);
        this.changes = this.convertValues(source["changes"], Change);
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}
export class ShortName {
    group?: string;
    kind: string;
    shortName: string;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.group = source["group"];
        this.kind = source["kind"];
        this.shortName = source["shortName"];
    }
}
export class UnstructuredObject {


    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);

    }
}
export class ProjectTargetKey {
    project: ProjectKey;
    target: TargetKey;

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.project = this.convertValues(source["project"], ProjectKey);
        this.target = this.convertValues(source["target"], TargetKey);
    }

	convertValues(a: any, classs: any, asMap: boolean = false): any {
	    if (!a) {
	        return a;
	    }
	    if (a.slice) {
	        return (a as any[]).map(elem => this.convertValues(elem, classs));
	    } else if ("object" === typeof a) {
	        if (asMap) {
	            for (const key of Object.keys(a)) {
	                a[key] = new classs(a[key]);
	            }
	            return a;
	        }
	        return new classs(a);
	    }
	    return a;
	}
}