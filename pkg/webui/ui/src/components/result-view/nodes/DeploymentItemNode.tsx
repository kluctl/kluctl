import React from 'react';

import { DeploymentItemConfig } from "../../../models";
import { NodeData } from "./NodeData";
import { Source } from "@mui/icons-material";
import { PropertiesTable } from "../../PropertiesTable";
import { CommandResultProps } from "../CommandResultView";
import { SidePanelTab } from "../SidePanel";


export class DeploymentItemNodeData extends NodeData {
    deploymentItem: DeploymentItemConfig

    constructor(props: CommandResultProps, id: string, deploymentItem: DeploymentItemConfig) {
        super(props, id, true, true);
        this.deploymentItem = deploymentItem
    }

    buildSidePanelTitle(): React.ReactNode {
        return this.deploymentItem.path
    }

    buildIcon(): [React.ReactNode, string] {
        let iconText = "di"
        return [<Source fontSize={"large"}/>, iconText]
    }

    buildSidePanelTabs(): SidePanelTab[] {
        const tabs = [
            {label: "Summary", content: this.buildSummaryPage()},
        ]
        this.buildDiffAndHealthPages(tabs)
        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props = []

        props.push({name: "Path", value: this.deploymentItem.path})

        buildDeploymentItemSummaryProps(this.deploymentItem, props)

        return <>
            <PropertiesTable properties={props}/>
        </>
    }
}

export function buildDeploymentItemSummaryProps(di: DeploymentItemConfig, props: {name: string, value: React.ReactNode}[]) {
    if (di.barrier !== undefined) {
        props.push({name: "Barrier", value: di.barrier + ""})
    }
    if (di.waitReadiness !== undefined) {
        props.push({name: "WaitReadiness", value: di.waitReadiness + ""})
    }
    if (di.skipDeleteIfTags !== undefined) {
        props.push({name: "SkipDeleteIfTags", value: di.skipDeleteIfTags + ""})
    }
    if (di.onlyRender !== undefined) {
        props.push({name: "OnlyRender", value: di.onlyRender + ""})
    }
    if (di.alwaysDeploy !== undefined) {
        props.push({name: "AlwaysDeploy", value: di.alwaysDeploy + ""})
    }
    if (di.deleteObjects) {
        // TODO this is ugly
        props.push({name: "DeleteObjects", value: JSON.stringify(di.deleteObjects)})
    }
    if (di.when) {
        props.push({name: "When", value: di.when})
    }
}
