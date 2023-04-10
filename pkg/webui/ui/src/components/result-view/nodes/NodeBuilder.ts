import {
    DeploymentError,
    DeploymentItemConfig,
    DeploymentProjectConfig,
    ObjectRef,
    ResultObject,
    VarsSource
} from "../../../models";
import { VarsSourceNodeData } from "./VarsSourceNode";
import { DeploymentItemNodeData } from "./DeploymentItemNode";
import { ObjectNodeData } from "./ObjectNode";
import { NodeData } from "./NodeData";
import { CommandResultNodeData } from "./CommandResultNode";
import { VarsSourceCollectionNodeData } from "./VarsSourceCollectionNode";
import { DeploymentItemIncludeNodeData } from "./DeploymentItemIncludeNode";
import { CommandResultProps } from "../CommandResultView";
import { DeletedOrOrphanObjectsCollectionNode } from "./DeletedOrOrphanObjectsCollectionNode";

export class NodeBuilder {
    private props: CommandResultProps
    private changedObjectsMap: Map<string, ResultObject> = new Map()
    private newObjectsMap: Map<string, ObjectRef> = new Map()
    private orphanObjectsMap: Map<string, ObjectRef> = new Map()
    private deletedObjectsMap: Map<string, ObjectRef> = new Map()
    private errorsMap: Map<string, DeploymentError[]> = new Map()
    private warningsMap: Map<string, DeploymentError[]> = new Map()

    constructor(props: CommandResultProps) {
        this.props = props

        props.commandResult.objects?.forEach(o => {
            const key = buildObjectRefKey(o.ref)
            if (o.changes?.length) {
                this.changedObjectsMap.set(key, o)
            }
            if (o.new) {
                this.newObjectsMap.set(key, o.ref)
            }
            if (o.orphan) {
                this.orphanObjectsMap.set(key, o.ref)
            }
            if (o.deleted) {
                this.deletedObjectsMap.set(key, o.ref)
            }
        })
        props.commandResult.errors?.forEach(e => {
            const key = buildObjectRefKey(e.ref)
            let l = this.errorsMap.get(key)
            if (!l) {
                l = [e]
                this.errorsMap.set(key, l)
            } else {
                l.push(e)
            }
        })
        props.commandResult.warnings?.forEach(e => {
            const key = buildObjectRefKey(e.ref)
            let l = this.warningsMap.get(key)
            if (!l) {
                l = [e]
                this.warningsMap.set(key, l)
            } else {
                l.push(e)
            }
        })
    }

    buildRoot(): [CommandResultNodeData, Map<string, NodeData>] {
        const rootNode = new CommandResultNodeData(this.props, "root")

        this.buildDeploymentProjectChildren(rootNode, this.props.commandResult.deployment!)

        if (this.deletedObjectsMap.size) {
            this.buildDeletedOrOrphanNode(rootNode, true, Array.from(this.deletedObjectsMap.values()))
        }
        if (this.orphanObjectsMap.size) {
            this.buildDeletedOrOrphanNode(rootNode, false, Array.from(this.orphanObjectsMap.values()))
        }

        const nodeMap: Map<string, NodeData> = new Map()
        function collect(n: NodeData) {
            nodeMap.set(n.id, n)
            n.children.forEach(c => {
                collect(c)
            })
        }
        collect(rootNode)

        return [rootNode, nodeMap]
    }

    buildDeploymentProjectChildren(node: NodeData, deploymentProject: DeploymentProjectConfig) {
        if (deploymentProject.vars) {
            this.buildVarsSourceCollectionNode(node, deploymentProject.vars)
        }
        deploymentProject.deployments?.forEach((deploymentItem, i) => {
            this.buildDeploymentItemNode(node, deploymentItem, i)
        })
    }

    buildVarsSourceCollectionNode(parentNode: NodeData, varsSources?: VarsSource[]) {
        if (varsSources === undefined) {
            return
        }

        const newId = `${parentNode.id}/(vars)`
        const node = new VarsSourceCollectionNodeData(this.props, newId)
        node.varsSources.push(...varsSources)

        varsSources.forEach((vs, i) => {
            this.buildVarsSourceNode(node, vs, i)
        })

        parentNode.pushChild(node)

        return node
    }

    buildVarsSourceNode(parentNode: NodeData, varsSource: VarsSource, index: number): VarsSourceNodeData {
        const newId = `${parentNode.id}/${index}}`
        const node = new VarsSourceNodeData(this.props, newId, varsSource)
        parentNode.pushChild(node)
        return node
    }

    buildDeploymentItemNode(parentNode: NodeData, deploymentItem: DeploymentItemConfig, index: number): NodeData | undefined{
        let node: NodeData | undefined
        const newId = `${parentNode.id}/(dis)/${index}`
        if (deploymentItem.path) {
            node = new DeploymentItemNodeData(this.props, newId, deploymentItem)
            this.buildVarsSourceCollectionNode(node, deploymentItem.vars)
            deploymentItem.renderedObjects?.forEach(renderedObject => {
                this.buildObjectNode(node!, renderedObject)
            })
        } else if (deploymentItem.include || deploymentItem.git) {
            node = new DeploymentItemIncludeNodeData(this.props, newId, deploymentItem, deploymentItem.renderedInclude!)
            this.buildVarsSourceCollectionNode(node, deploymentItem.vars)
            if (deploymentItem.renderedInclude) {
                this.buildDeploymentProjectChildren(node, deploymentItem.renderedInclude)
            }
        } else {
            return node
        }

        parentNode.pushChild(node)

        return node
    }

    buildObjectNode(parentNode: NodeData, objectRef: ObjectRef): ObjectNodeData {
        const newId = `${parentNode.id}/(obj)/${buildObjectRefKey(objectRef)}`
        const node = new ObjectNodeData(this.props, newId, objectRef)

        const key = buildObjectRefKey(objectRef)
        if (this.changedObjectsMap.has(key)) {
            node.diffStatus?.addChangedObject(this.changedObjectsMap.get(key)!)
        }
        if (this.newObjectsMap.has(key)) {
            node.diffStatus?.newObjects.push(objectRef)
        }
        if (this.deletedObjectsMap.has(key)) {
            node.diffStatus?.deletedObjects.push(objectRef)
        }
        if (this.orphanObjectsMap.has(key)) {
            node.diffStatus?.orphanObjects.push(objectRef)
        }

        this.errorsMap.get(key)?.forEach(e => {
            node.healthStatus?.errors.push(e)
        })
        this.warningsMap.get(key)?.forEach(e => {
            node.healthStatus?.warnings.push(e)
        })

        parentNode.pushChild(node)

        return node
    }

    buildDeletedOrOrphanNode(parentNode: NodeData, deleted: boolean, refs: ObjectRef[]): DeletedOrOrphanObjectsCollectionNode {
        const idType = deleted ? "deleted" : "orphaned"
        const newId = `${parentNode.id}/(${idType})`
        const node = new DeletedOrOrphanObjectsCollectionNode(this.props, newId, deleted)
        refs.forEach(ref => {
            this.buildObjectNode(node, ref)
        })
        parentNode.pushChild(node, true)
        return node
    }
}

function buildObjectRefKey(ref: ObjectRef): string {
    return [ref.group, ref.version, ref.kind, ref.namespace, ref.name].join("+")
}
