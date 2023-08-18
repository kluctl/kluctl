import { ChangedObject, CommandResult, DeploymentError, ObjectRef, ResultObject } from "../../../models";
import React from "react";
import { Box, Divider, Typography } from "@mui/material";
import { ChangesTable } from "../ChangesTable";
import { ErrorsTable } from "../../ErrorsTable";
import { ObjectType, User } from "../../../api";
import { ResultObjectViewer } from "../../ResultObjectViewer";
import { StatusLine } from "../CommandResultStatusLine";
import { SidePanelProvider, SidePanelTab } from "../SidePanel";
import { AppContextProps } from "../../App";

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
    commandResult: CommandResult

    id: string
    children: NodeData[] = []

    healthStatus?: HealthStatus;
    diffStatus?: DiffStatus;

    protected constructor(commandResult: CommandResult, id: string, hasHealthStatus: boolean, hasDiffStatus: boolean) {
        this.commandResult = commandResult
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

    abstract buildIcon(appContext: AppContextProps): [React.ReactNode, string]

    abstract buildSidePanelTabs(user: User): SidePanelTab[]

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

    buildTreeItem(appContext: AppContextProps, hasChildren: boolean): React.ReactNode {
        const [icon, iconText] = this.buildIcon(appContext);

        const hasStatusLine = [
            this.healthStatus?.errors.length,
            this.healthStatus?.warnings.length,
            this.diffStatus?.changedObjects.length,
            this.diffStatus?.newObjects.length,
            this.diffStatus?.deletedObjects.length,
            this.diffStatus?.orphanObjects.length,
        ].some(x => (x || 0) > 0);

        return <Box
            display='flex'
            height='100%'
            flex='1 1 auto'
        >
            <Box
                display='flex'
                flexDirection='column'
                alignItems='center'
                justifyContent='center'
                width='30px'
                height='100%'
                flex='0 0 auto'
                mr='13px'
                sx={{
                    '& svg': {
                        width: '30px',
                        height: '30px'
                    }
                }}
            >
                {icon}
                <Typography
                    variant='subtitle1'
                    component='div'
                    fontSize='12px'
                    fontWeight={400}
                    lineHeight='16px'
                    height='16px'
                >
                    {iconText}
                </Typography>
            </Box>
            <Box
                display='flex'
                height='100%'
                flex='1 1 auto'
                py='15px'
            >
                <Typography
                    variant='h6'
                    component='div'
                    sx={{ wordBreak: 'break-all' }}
                    {...(hasChildren ? {} : { fontSize: '16px', lineHeight: '22px' })}
                >
                    {this.buildSidePanelTitle()}
                </Typography>
            </Box>
            {hasStatusLine && <Box
                height='100%'
                width='172px'
                flex='0 0 auto'
                display='flex'
                alignItems='center'
                px='14px'
                ml='14px'
                position='relative'
            >
                <Divider
                    orientation='vertical'
                    sx={{
                        height: '40px',
                        position: 'absolute',
                        left: 0
                    }}
                />
                {this.buildStatusLine()}
            </Box>}
        </Box>
    }

    buildDiffAndHealthPages(tabs: { label: string, content: React.ReactNode }[]) {
        this.buildChangesPage(tabs)
        this.buildErrorsPage(tabs)
        this.buildWarningsPage(tabs)
    }

    buildObjectPage(ref: ObjectRef, objectType: ObjectType): React.ReactNode {
        return <ResultObjectViewer cr={this.commandResult} objectRef={ref} objectType={objectType} />
    }

    buildChangesPage(tabs: { label: string, content: React.ReactNode }[]) {
        if (!this.diffStatus?.hasDiffs()) {
            return undefined
        }
        tabs.push({
            label: "Changes",
            content: <ChangesTable diffStatus={this.diffStatus} />
        })
    }

    buildErrorsPage(tabs: { label: string, content: React.ReactNode }[]) {
        if (!this.healthStatus?.errors.length) {
            return undefined
        }
        tabs.push({
            label: "Errors",
            content: <ErrorsTable errors={this.healthStatus.errors} />
        })
    }

    buildWarningsPage(tabs: { label: string, content: React.ReactNode }[]) {
        if (!this.healthStatus?.warnings.length) {
            return undefined
        }
        tabs.push({
            label: "Warnings",
            content: <ErrorsTable errors={this.healthStatus.warnings} />
        })
    }
}
