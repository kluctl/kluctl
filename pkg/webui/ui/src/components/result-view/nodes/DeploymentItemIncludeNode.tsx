import React from 'react';

import { DeploymentItemConfig, DeploymentProjectConfig } from "../../../models";
import { NodeData } from "./NodeData";
import { FolderZip } from "@mui/icons-material";
import { GitIcon } from "../../../icons/GitIcon";
import { CommandResultProps } from "../CommandResultView";
import { PropertiesTable } from "../../PropertiesTable";
import { buildDeploymentItemSummaryProps } from "./DeploymentItemNode";
import { SidePanelTab } from "../SidePanel";


export class DeploymentItemIncludeNodeData extends NodeData {
    deploymentItem: DeploymentItemConfig
    includedDeployment: DeploymentProjectConfig

    constructor(props: CommandResultProps, id: string, deploymentItem: DeploymentItemConfig, includedDeployment: DeploymentProjectConfig) {
        super(props, id, true, true);
        this.deploymentItem = deploymentItem
        this.includedDeployment = includedDeployment
    }

    buildSidePanelTitle(): React.ReactNode {
        if (this.deploymentItem.include) {
            return this.deploymentItem.include
        } else if (this.deploymentItem.git) {
            const s = this.deploymentItem.git!.url.split("/")
            const name = s[s.length-1]
            return <>
                {name}
                {this.deploymentItem.git!.subDir && (<><br/>{this.deploymentItem.git!.subDir}</>)}
            </>
        } else {
            return "unknown include"
        }
    }

    buildIcon(): [React.ReactNode, string] {
        if (this.deploymentItem.git) {
            return [<GitIcon/>, "git"]
        }
        return [<FolderZip fontSize={"large"}/>, "include"]
    }

    buildSidePanelTabs(): SidePanelTab[] {
        const tabs = [
            {label: "Summary", content: this.buildSummaryPage()},
        ]
        this.buildDiffAndHealthPages(tabs)
        return tabs;
    }

    buildSummaryPage(): React.ReactNode {
        const props = []

        if (this.deploymentItem.include) {
            props.push({ name: "Type", value: "LocalInclude" })
            props.push({ name: "Path", value: this.deploymentItem.include })
        } else if (this.deploymentItem.git) {
            props.push({name: "Type", value: "GitInclude"})
            props.push({name: "Url", value: this.deploymentItem.git.url})
            props.push({name: "SubDir", value: this.deploymentItem.git.subDir})
            let ref = "HEAD"
            if (this.deploymentItem.git.ref) {
                ref = this.deploymentItem.git.ref!
            }
            props.push({name: "Ref", value: ref})
        } else {
            props.push({name: "Type", value: "Unknown"})
        }

        buildDeploymentItemSummaryProps(this.deploymentItem, props)

        return <>
            <PropertiesTable properties={props}/>
        </>
    }
}
