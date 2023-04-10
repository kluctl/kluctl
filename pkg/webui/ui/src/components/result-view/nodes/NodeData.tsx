import { ChangedObject, DeploymentError, ObjectRef, ResultObject } from "../../../models";
import React from "react";
import { Box, Typography } from "@mui/material";
import { CommandResultProps } from "../CommandResultView";
import { ChangesTable } from "../ChangesTable";
import { ErrorsTable } from "../../ErrorsTable";
import { ObjectType } from "../../../api";
import { ObjectYaml } from "../../ObjectYaml";
import { StatusLine } from "../CommandResultStatusLine";
import { SidePanelProvider, SidePanelTab } from "../SidePanel";

export class DiffStatus {
    newObjects: ObjectRef[] = [];
    deletedObjects: ObjectRef[] = [];
    orphanObjects: ObjectRef[] = [];
    changedObjects: ChangedObject[] = [];

    totalInsertions: number = 0;
    totalDeletions: number = 0;
    totalUpdates: number = 0;

    addChangedObject(co: ResultObject) {
        this.changedObjects.push(co)
        co.changes?.forEach(x => {
            switch (x.type) {
                case "insert":
                    this.totalInsertions++
                    break
                case "delete":
                    this.totalDeletions++
                    break
                case "update":
                    this.totalUpdates++
                    break
            }
        })
    }

    merge(other: DiffStatus) {
        this.newObjects = this.newObjects.concat(other.newObjects)
        this.deletedObjects = this.deletedObjects.concat(other.deletedObjects)
        this.orphanObjects = this.orphanObjects.concat(other.orphanObjects)
        this.changedObjects = this.changedObjects.concat(other.changedObjects)

        this.totalInsertions += other.totalInsertions
        this.totalDeletions += other.totalDeletions
        this.totalUpdates += other.totalUpdates
    }

    hasDiffs() {
        if (this.newObjects.length || this.deletedObjects.length || this.orphanObjects.length) {
            return true
        }
        if (this.changedObjects.find(co => co.changes?.length !== 0)) {
            return true
        }
        return false
    }
}

export class HealthStatus {
    errors: DeploymentError[] = []
    warnings: DeploymentError[] = []

    merge(other: HealthStatus) {
        this.errors = this.errors.concat(other.errors)
        this.warnings = this.warnings.concat(other.warnings)
    }
}

export abstract class NodeData implements SidePanelProvider {
    props: CommandResultProps

    id: string
    children: NodeData[] = []

    healthStatus?: HealthStatus;
    diffStatus?: DiffStatus;

    protected constructor(props: CommandResultProps, id: string, hasHealthStatus: boolean, hasDiffStatus: boolean) {
        this.props = props
        this.id = id
        if (hasHealthStatus) {
            this.healthStatus = new HealthStatus()
        }
        if (hasDiffStatus) {
            this.diffStatus = new DiffStatus()
        }
    }

    pushChild(child: NodeData, front?: boolean) {
        if (front) {
            this.children.unshift(child)
        } else {
            this.children.push(child)
        }
        this.merge(child)
    }

    merge(other: NodeData) {
        if (this.diffStatus && other.diffStatus) {
            this.diffStatus.merge(other.diffStatus)
        }
        if (this.healthStatus && other.healthStatus) {
            this.healthStatus.merge(other.healthStatus)
        }
    }

    abstract buildSidePanelTitle(): React.ReactNode

    abstract buildIcon(): [React.ReactNode, string]

    abstract buildSidePanelTabs(): SidePanelTab[]

    buildStatusLine(): React.ReactNode {
        return <StatusLine
            errors={this.healthStatus?.errors.length}
            warnings={this.healthStatus?.warnings.length}
            changedObjects={this.diffStatus?.changedObjects.length}
            newObjects={this.diffStatus?.newObjects.length}
            deletedObjects={this.diffStatus?.deletedObjects.length}
            orphanObjects={this.diffStatus?.orphanObjects.length}
        />
    }

    buildTreeItem(): React.ReactNode {
        const [icon, iconText] = this.buildIcon()

        return <Box>
            <Box display={"flex"}>
                <Box display="flex" justifyContent="center" alignItems="center">
                    <Box display="flex" flexDirection={"column"} alignItems={"center"}>
                        {icon}
                        <Typography fontSize={"10px"} color={"gray"}>{iconText}</Typography>
                    </Box>
                </Box>
                <Box display={"flex"} flexDirection={"column"}>
                    <Box>
                        {this.buildSidePanelTitle()}<br/>
                    </Box>
                    <Box>
                        {this.buildStatusLine()}
                    </Box>
                </Box>
            </Box>
        </Box>
    }

    buildDiffAndHealthPages(tabs: { label: string, content: React.ReactNode }[]) {
        this.buildChangesPage(tabs)
        this.buildErrorsPage(tabs)
        this.buildWarningsPage(tabs)
    }

    buildObjectPage(ref: ObjectRef, objectType: ObjectType): React.ReactNode {
        return <ObjectYaml treeProps={this.props} objectRef={ref} objectType={objectType}/>
    }

    buildChangesPage(tabs: { label: string, content: React.ReactNode }[]) {
        if (!this.diffStatus?.hasDiffs()) {
            return undefined
        }
        tabs.push({
            label: "Changes",
            content: <ChangesTable diffStatus={this.diffStatus}/>
        })
    }

    buildErrorsPage(tabs: { label: string, content: React.ReactNode }[]) {
        if (!this.healthStatus?.errors.length) {
            return undefined
        }
        tabs.push({
            label: "Errors",
            content: <ErrorsTable errors={this.healthStatus.errors}/>
        })
    }

    buildWarningsPage(tabs: { label: string, content: React.ReactNode }[]) {
        if (!this.healthStatus?.warnings.length) {
            return undefined
        }
        tabs.push({
            label: "Warnings",
            content: <ErrorsTable errors={this.healthStatus.warnings}/>
        })
    }
}
