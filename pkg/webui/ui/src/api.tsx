import {
    AuthInfo,
    CommandResult,
    CommandResultSummary,
    ObjectRef,
    ResultObject,
    ShortName,
    ValidateResult
} from "./models";
import _ from "lodash";
import React from "react";
import { Box, Typography } from "@mui/material";
import Tooltip from "@mui/material/Tooltip";
import "./staticbuild.d.ts"
import { loadScript } from "./loadscript";
import { GitRef } from "./models-static";
import { sleep } from "./utils/misc";
import { KluctlDeploymentWithClusterId } from "./components/App";

console.log(window.location)

export let rootPath = window.location.pathname
if (rootPath.endsWith("/")) {
    rootPath = rootPath.substring(0, rootPath.length-1)
}
if (rootPath.endsWith("index.html")) {
    rootPath = rootPath.substring(0, rootPath.length-"index.html".length-1)
}
const staticPath = rootPath + "/staticdata"

console.log("rootPath=" + rootPath)
console.log("staticPath=" + staticPath)

export enum ObjectType {
    Rendered = "rendered",
    Remote = "remote",
    Applied = "applied",
}

export interface User {
    username: string
    isAdmin: boolean
}

export interface Api {
    getAuthInfo(): Promise<AuthInfo>
    getUser(): Promise<User>
    getShortNames(): Promise<ShortName[]>
    listenEvents(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void>
    getCommandResult(resultId: string): Promise<CommandResult>
    getCommandResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any>
    getValidateResult(resultId: string): Promise<ValidateResult>
    validateNow(cluster: string, name: string, namespace: string): Promise<Response>
    reconcileNow(cluster: string, name: string, namespace: string): Promise<Response>
    deployNow(cluster: string, name: string, namespace: string): Promise<Response>
    pruneNow(cluster: string, name: string, namespace: string): Promise<Response>
    setSuspended(cluster: string, name: string, namespace: string, suspend: boolean): Promise<Response>
    setManualObjectsHash(cluster: string, name: string, namespace: string, objectsHash: string): Promise<Response>
    watchLogs(cluster: string | undefined, name: string | undefined, namespace: string | undefined, reconcileId: string | undefined, handle: (lines: any[]) => void): () => void
}

export async function checkStaticBuild() {
    const p = loadScript(staticPath + "/summaries.js")
    try {
        await p
        return true
    } catch (error) {
        return false
    }
}

export class RealApi implements Api {
    onUnauthorized?: () => void;

    constructor(onUnauthorized?: () => void) {
        this.onUnauthorized = onUnauthorized
    }

    handleErrors(response: Response) {
        if (!response.ok) {
            if (response.status === 401) {
                if (this.onUnauthorized) {
                    this.onUnauthorized()
                }
            }
            throw Error(response.statusText)
        }
    }

    async doGet(path: string, params?: URLSearchParams, abort?: AbortSignal) {
        let url = rootPath + path
        if (params) {
            url += "?" + params.toString()
        }
        const resp = await fetch(url, {
            method: "GET",
            signal: abort ? abort : null,
        })
        this.handleErrors(resp)
        return resp.json()
    }

    async doPost(path: string, body: any) {
        let url = rootPath + path
        const headers = {
            'Accept': 'application/json',
            'Content-Type': 'application/json'
        }
        const resp = await fetch(url, {
            method: "POST",
            body: JSON.stringify(body),
            headers: headers,
        })
        this.handleErrors(resp)
        return resp
    }

    doWebsocket(path: string, params: URLSearchParams) {
        let host = window.location.host
        let proto = "wss"
        if (process.env.NODE_ENV === 'development') {
            host = "localhost:9090"
        }
        if (window.location.protocol !== "https:") {
            proto = "ws"
        }
        let url = `${proto}://${host}${rootPath}${path}`
        url += "?" + params.toString()

        console.log("ws connect: " + url)
        const ws = new WebSocket(url)
        return ws
    }

    async getAuthInfo(): Promise<AuthInfo> {
        return this.doGet("/auth/info")
    }

    async getUser(): Promise<User> {
        return this.doGet("/auth/user")
    }

    async getShortNames(): Promise<ShortName[]> {
        return this.doGet("/api/getShortNames")
    }

    async listenEvents(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        const params = new URLSearchParams()
        if (filterProject) {
            params.set("filterProject", filterProject)
        }
        if (filterSubDir) {
            params.set("filterSubDir", filterSubDir)
        }

        let ws: WebSocket | undefined;
        let cancelled = false

        const connect = async () => {
            if (cancelled) {
                return
            }

            ws = this.doWebsocket("/api/events", params)
            ws.onopen = function () {
                console.log("ws connected")
            }
            ws.onclose = function (event) {
                console.log("ws close")
                if (!cancelled) {
                    sleep(5000).then(connect)
                }
            }
            ws.onerror = function (event) {
                console.log("ws error", event)
            }
            ws.onmessage = function (event: MessageEvent) {
                if (cancelled) {
                    return
                }
                const msg: any[] = JSON.parse(event.data)
                msg.forEach(handle)
            }
        }

        await connect()

        return () => {
            console.log("ws cancel")
            cancelled = true
            if (ws) {
                ws.close()
            }
        }
    }

    async getCommandResult(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getCommandResult", params)
        return new CommandResult(json)
    }

    async getCommandResultObject(resultId: string, ref: ObjectRef, objectType: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        params.set("objectType", objectType)
        buildRefParams(ref, params)
        return await this.doGet("/api/getCommandResultObject", params)
    }

    async getValidateResult(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getValidateResult", params)
        return new ValidateResult(json)
    }

    async validateNow(cluster: string, name: string, namespace: string) {
        return this.doPost("/api/validateNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }

    async deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("/api/deployNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }

    async pruneNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("/api/pruneNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }

    async reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("/api/reconcileNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }

    async setSuspended(cluster: string, name: string, namespace: string, suspend: boolean): Promise<Response> {
        return this.doPost("/api/setSuspended", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
            "suspend": suspend,
        })
    }

    async setManualObjectsHash(cluster: string, name: string, namespace: string, objectsHash: string): Promise<Response> {
        return this.doPost("/api/setManualObjectsHash", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
            "objectsHash": objectsHash,
        })
    }

    watchLogs(cluster: string | undefined, name: string | undefined, namespace: string | undefined, reconcileId: string | undefined, handle: (lines: any[]) => void): () => void {
        const params = new URLSearchParams()
        if (cluster) params.set("cluster", cluster)
        if (name) params.set("name", name)
        if (namespace) params.set("namespace", namespace)
        if (reconcileId) params.set("reconcileId", reconcileId)

        const ws = this.doWebsocket("/api/logs", params)
        let cancelled = false

        ws.onmessage = function (event: MessageEvent) {
            if (cancelled) {
                return
            }
            try {
                const line: any = JSON.parse(event.data)
                handle([line])
            } catch (e) {
            }
        }

        return () => {
            console.log("cancel logs", params.toString())
            cancelled = true
            ws.close()
        }
    }
}

export class StaticApi implements Api {
    async getAuthInfo(): Promise<AuthInfo> {
        const info = new AuthInfo()
        info.authEnabled = false
        info.staticLoginEnabled = false
        return info
    }

    async getUser(): Promise<User> {
        return {
            "username": "no-user",
            "isAdmin": true,
        }
    }

    async getShortNames(): Promise<ShortName[]> {
        await loadScript(staticPath + "/shortnames.js")
        return staticShortNames
    }

    async listenEvents(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        await loadScript(staticPath + "/summaries.js")
        await loadScript(staticPath + "/kluctldeployments.js")

        staticKluctlDeployments.forEach(kd_ => {
            const kd = kd_ as KluctlDeploymentWithClusterId
            if (filterProject && filterProject !== kd.deployment.status?.projectKey?.gitRepoKey) {
                return
            }
            if (filterSubDir && filterSubDir !== kd.deployment.status?.projectKey?.subDir) {
                return
            }
            handle({
                "type": "update_kluctl_deployment",
                "deployment": kd.deployment,
                "clusterId": kd.clusterId,
            })
        })

        staticSummaries.forEach(rs_ => {
            const rs = rs_ as CommandResultSummary
            if (filterProject && filterProject !== rs.projectKey.gitRepoKey) {
                return
            }
            if (filterSubDir && filterSubDir !== rs.projectKey.subDir) {
                return
            }
            handle({
                "type": "update_command_result_summary",
                "summary": rs,
            })
        })
        return () => {
        }
    }

    async getCommandResult(resultId: string): Promise<CommandResult> {
        await loadScript(staticPath + `/result-${resultId}.js`)
        return staticResults.get(resultId)
    }

    async getCommandResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any> {
        const result = await this.getCommandResult(resultId)
        const object = result.objects?.find(x => _.isEqual(x.ref, ref))
        if (!object) {
            throw new Error("object not found")
        }
        switch (objectType) {
            case ObjectType.Rendered:
                return object.rendered
            case ObjectType.Remote:
                return object.remote
            case ObjectType.Applied:
                return object.applied
            default:
                throw new Error("unknown object type " + objectType)
        }
    }

    async getValidateResult(resultId: string): Promise<ValidateResult> {
        throw new Error("not implemented")
    }

    validateNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }

    reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }

    deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }

    pruneNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }

    setSuspended(cluster: string, name: string, namespace: string, suspend: boolean): Promise<Response> {
        throw new Error("not implemented")
    }

    setManualObjectsHash(cluster: string, name: string, namespace: string, objectsHash: string): Promise<Response> {
        throw new Error("not implemented")
    }

    watchLogs(cluster: string, name: string, namespace: string, reconcileId: string, handle: (lines: any[]) => void): () => void {
        return () => {}
    }
}

function buildRefParams(ref: ObjectRef, params: URLSearchParams) {
    params.set("kind", ref.kind)
    params.set("name", ref.name)
    if (ref.group) {
        params.set("group", ref.group)
    }
    if (ref.version) {
        params.set("version", ref.version)
    }
    if (ref.namespace) {
        params.set("namespace", ref.namespace)
    }
}

export function buildRefString(ref: ObjectRef): string {
    if (ref.namespace) {
        return `${ref.namespace}/${ref.kind}/${ref.name}`
    } else {
        if (ref.name) {
            return `${ref.kind}/${ref.name}`
        } else {
            return ref.kind
        }
    }
}

export function buildRefKindElement(ref: ObjectRef, element?: React.ReactElement): React.ReactElement {
    const tooltip = <Box zIndex={1000}>
        <Typography>ApiVersion: {[ref.group, ref.version].filter(x => x).join("/")}</Typography><br/>
        <Typography>Kind: {ref.kind}</Typography>
    </Box>
    return <Tooltip title={tooltip}>
        {element ? element : <Typography>{ref.kind}</Typography>}
    </Tooltip>
}

export function buildObjectRefFromObject(obj: any): ObjectRef {
    const apiVersion: string = obj.apiVersion
    const s = apiVersion.split("/", 2)
    let ref = new ObjectRef()
    if (s.length === 1) {
        ref.version = s[0]
    } else {
        ref.group = s[0]
        ref.version = s[1]
    }
    ref.kind = obj.kind
    ref.namespace = obj.metadata.namespace
    ref.name = obj.metadata.name
    return ref
}

export function buildGitRefString(ref?: GitRef) {
    if (!ref) {
        return "HEAD"
    }
    if (ref.branch) {
        return ref.branch
    } else if (ref.tag) {
        return ref.tag
    } else {
        return "<unknown>"
    }
}

export function findObjectByRef(l: ResultObject[] | undefined, ref: ObjectRef, filter?: (o: ResultObject) => boolean): ResultObject | undefined {
    return l?.find(x => {
        if (filter && !filter(x)) {
            return false
        }
        return _.isEqual(x.ref, ref)
    })
}
