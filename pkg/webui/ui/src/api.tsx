import {
    CommandResult,
    CommandResultSummary,
    ObjectRef,
    ProjectKey,
    ResultObject,
    ShortName,
    TargetKey
} from "./models";
import _ from "lodash";
import React from "react";
import { Box, Typography } from "@mui/material";
import Tooltip from "@mui/material/Tooltip";
import "./staticbuild.d.ts"
import { loadScript } from "./loadscript";
import { sleep } from "./utils/misc";

const apiUrl = "/api"
const staticPath = "./staticdata"

export enum ObjectType {
    Rendered = "rendered",
    Remote = "remote",
    Applied = "applied",
}

export interface Api {
    getShortNames(): Promise<ShortName[]>
    listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void>
    getResult(resultId: string): Promise<CommandResult>
    getResultSummary(resultId: string): Promise<CommandResultSummary>
    getResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any>
    validateNow(project: ProjectKey, target: TargetKey): Promise<Response>
    reconcileNow(cluster: string, name: string, namespace: string): Promise<Response>
    deployNow(cluster: string, name: string, namespace: string): Promise<Response>
}

class RealOrStaticApi implements Api {
    api: Promise<Api>

    constructor() {
        this.api = this.buildApi()
    }

    async buildApi(): Promise<Api> {
        const p = loadScript(staticPath + "/summaries.js")
        try {
            await p
            return new StaticApi()
        } catch (error) {
            return new RealApi()
        }
    }

    async deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return (await this.api).deployNow(cluster, name, namespace)
    }

    async getResult(resultId: string): Promise<CommandResult> {
        return (await this.api).getResult(resultId)
    }

    async getResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any> {
        return (await this.api).getResultObject(resultId, ref, objectType)
    }

    async getResultSummary(resultId: string): Promise<CommandResultSummary> {
        return (await this.api).getResultSummary(resultId)
    }

    async getShortNames(): Promise<ShortName[]> {
        return (await this.api).getShortNames()
    }

    async listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        return (await this.api).listenUpdates(filterProject, filterSubDir, handle)
    }

    async reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return (await this.api).reconcileNow(cluster, name, namespace)
    }

    async validateNow(project: ProjectKey, target: TargetKey): Promise<Response> {
        return (await this.api).validateNow(project, target)
    }
}

export const api = new RealOrStaticApi()

class RealApi implements Api {
    async getShortNames(): Promise<ShortName[]> {
        let url = `${apiUrl}/getShortNames`
        return fetch(url)
            .then(handleErrors)
            .then((response) => response.json());
    }

    async listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        let host = window.location.host
        if (process.env.NODE_ENV === 'development') {
            host = "localhost:9090"
        }
        let url = `ws://${host}${apiUrl}/ws`
        const params = new URLSearchParams()
        if (filterProject) {
            params.set("filterProject", filterProject)
        }
        if (filterSubDir) {
            params.set("filterSubDir", filterSubDir)
        }
        url += "?" + params.toString()

        let ws: WebSocket | undefined;
        let cancelled = false

        const connect = () => {
            if (cancelled) {
                return
            }

            console.log("ws connect: " + url)
            ws = new WebSocket(url);
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
                const msg = JSON.parse(event.data)
                handle(msg)
            }
        }

        connect()

        return () => {
            console.log("ws cancel")
            cancelled = true
            if (ws) {
                ws.close()
            }
        }
    }

    async getResult(resultId: string) {
        let url = `${apiUrl}/getResult?resultId=${resultId}`
        return fetch(url)
            .then(handleErrors)
            .then(response => response.text())
            .then(json => {
                return new CommandResult(json)
            });
    }

    async getResultSummary(resultId: string) {
        let url = `${apiUrl}/getResultSummary?resultId=${resultId}`
        return fetch(url)
            .then(handleErrors)
            .then(response => response.text())
            .then(json => {
                return new CommandResultSummary(json)
            });
    }

    async getResultObject(resultId: string, ref: ObjectRef, objectType: string) {
        let url = `${apiUrl}/getResultObject?resultId=${resultId}&${buildRefParams(ref)}&objectType=${objectType}`
        return fetch(url)
            .then(handleErrors)
            .then(response => response.json());
    }

    async doPost(f: string, body: any) {
        let url = `${apiUrl}/${f}`
        return fetch(url, {
            method: "POST",
            body: JSON.stringify(body),
            headers: {
                'Accept': 'application/json',
                'Content-Type': 'application/json'
            },
        }).then(handleErrors);
    }

    async validateNow(project: ProjectKey, target: TargetKey) {
        return this.doPost("validateNow", {
            "project": project,
            "target": target,
        })
    }

    async deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("deployNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }

    async reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("reconcileNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }
}

class StaticApi implements Api {
    async getShortNames(): Promise<ShortName[]> {
        await loadScript(staticPath + "/shortnames.js")
        return staticShortNames
    }

    async listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        await loadScript(staticPath + "/summaries.js")

        staticSummaries.forEach(rs => {
            if (filterProject && filterProject !== rs.project.normalizedGitUrl) {
                return
            }
            if (filterSubDir && filterSubDir !== rs.project.subDir) {
                return
            }
            handle({
                "type": "update_summary",
                "summary": rs,
            })
        })
        return () => {
        }
    }

    async getResult(resultId: string): Promise<CommandResult> {
        await loadScript(staticPath + `/result-${resultId}.js`)
        return staticResults.get(resultId)
    }
    async getResultSummary(resultId: string): Promise<CommandResultSummary> {
        await loadScript(staticPath + "/summaries.js")
        return staticSummaries.filter(s => {
            return s.id === resultId
        }).at(0)
    }
    async getResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any> {
        const result = await this.getResult(resultId)
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
    validateNow(project: ProjectKey, target: TargetKey): Promise<Response> {
        throw new Error("not implemented")
    }
    reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }
    deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }
}

function handleErrors(response: Response) {
    if (!response.ok) {
        throw Error(response.statusText)
    }
    return response
}

function buildRefParams(ref: ObjectRef): string {
    const params = new URLSearchParams()
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
    return params.toString()
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

export function findObjectByRef(l: ResultObject[] | undefined, ref: ObjectRef, filter?: (o: ResultObject) => boolean): ResultObject | undefined {
    return l?.find(x => {
        if (filter && !filter(x)) {
            return false
        }
        return _.isEqual(x.ref, ref)
    })
}

export function usePromise<T>(p?: Promise<T>): T  {
    if (p === undefined) {
        throw new Promise<T>(() => undefined)
    }

    const promise = p as any
    if (promise.status === 'fulfilled') {
        return promise.value;
    } else if (promise.status === 'rejected') {
        throw promise.reason;
    } else if (promise.status === 'pending') {
        throw promise;
    } else {
        promise.status = 'pending';
        p.then(
            result => {
                promise.status = 'fulfilled';
                promise.value = result;
            },
            reason => {
                promise.status = 'rejected';
                promise.reason = reason;
            },
        );
        throw promise;
    }
}