import React from 'react';

import { CommandResult, DeploymentItemConfig, DeploymentProjectConfig } from "../../../models";
import { NodeData } from "./NodeData";
import { GitIcon, IncludeIcon } from "../../../icons/Icons";
import { PropertiesTable } from "../../PropertiesTable";
import { buildDeploymentItemSummaryProps } from "./DeploymentItemNode";
import { CardTab } from "../../card/CardTabs";
import { buildGitRefString, buildOciRefString } from "../../../api";
import { Archive } from "@mui/icons-material";


export class DeploymentItemIncludeNodeData extends NodeData {
    deploymentItem: DeploymentItemConfig
    includedDeployment: DeploymentProjectConfig

    constructor(commandResult: CommandResult, id: string, deploymentItem: DeploymentItemConfig, includedDeployment: DeploymentProjectConfig) {
        super(commandResult, id, true, true);
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
        } else if (this.deploymentItem.oci) {
            const s = this.deploymentItem.oci!.url.split("/")
            const name = s[s.length-1]
            return <>
                {name}
                {this.deploymentItem.oci!.subDir && (<><br/>{this.deploymentItem.oci!.subDir}</>)}
            </>
        } else {
            return "unknown include"
        }
    }

    buildIcon(): [React.ReactNode, string] {
        if (this.deploymentItem.git) {
            return [<GitIcon/>, "git"]
        } else if (this.deploymentItem.oci) {
            return [<Archive/>, "oci"]
        }
        return [<IncludeIcon />, "include"]
    }

    buildSidePanelTabs(): CardTab[] {
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
            props.push({name: "Ref", value: buildGitRefString(this.deploymentItem.git.ref)})
        } else if (this.deploymentItem.oci) {
            props.push({name: "Type", value: "OciInclude"})
            props.push({name: "Url", value: this.deploymentItem.oci.url})
            props.push({name: "SubDir", value: this.deploymentItem.oci.subDir})
            props.push({name: "Ref", value: buildOciRefString(this.deploymentItem.oci.ref)})
        } else {
            props.push({name: "Type", value: "Unknown"})
        }

        buildDeploymentItemSummaryProps(this.deploymentItem, props)

        return <>
            <PropertiesTable properties={props}/>
        </>
    }
}
