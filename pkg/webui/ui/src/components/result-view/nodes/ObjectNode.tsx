import React from 'react';

import { ObjectRef } from "../../../models";
import { NodeData } from "./NodeData";
import {
    DataObject,
    PublishedWithChanges,
    Settings,
    SettingsEthernet,
    SmartToy,
    SvgIconComponent
} from "@mui/icons-material";
import { PropertiesTable } from "../../PropertiesTable";
import { findObjectByRef, ObjectType } from "../../../api";
import { CommandResultProps } from "../CommandResultView";
import { SidePanelTab } from "../SidePanel";

const kindMapping: {[key: string]: {icon: SvgIconComponent}} = {
    "/ConfigMap": {icon: Settings},
    "apps/Deployment": {icon: PublishedWithChanges},
    "Service": {icon: SettingsEthernet},
    "ServiceAccount": {icon: SmartToy}
}

export class ObjectNodeData extends NodeData {
    objectRef: ObjectRef

    constructor(props: CommandResultProps, id: string, objectRef: ObjectRef) {
        super(props, id, true, true);
        this.objectRef = objectRef
    }

    buildSidePanelTitle(): React.ReactNode {
        return this.objectRef.name
    }

    buildIcon(): [React.ReactNode, string] {
        const sn = this.props.shortNames.find(sn => sn.group === this.objectRef.group && sn.kind === this.objectRef.kind)
        const snStr = sn?.shortName || ""

        const m = kindMapping[this.objectRef.group + "/" + this.objectRef.kind]
        if (m !== undefined) {
            return [React.createElement(m.icon, {fontSize: "large"}), snStr]
        }

        return [<DataObject fontSize={"large"}/>, snStr]
    }

    buildSidePanelTabs(): SidePanelTab[] {
        const tabs = [
            {label: "Summary", content: this.buildSummaryPage()}
        ]

        this.buildDiffAndHealthPages(tabs)

        if (findObjectByRef(this.props.commandResult.objects, this.objectRef)?.rendered) {
            tabs.push({ label: "Rendered", content: this.buildObjectPage(this.objectRef, ObjectType.Rendered) })
        }

        if (findObjectByRef(this.props.commandResult.objects, this.objectRef)?.remote) {
            tabs.push({label: "Remote", content: this.buildObjectPage(this.objectRef, ObjectType.Remote)})
        }
        if (findObjectByRef(this.props.commandResult.objects, this.objectRef)?.applied) {
            tabs.push({label: "Applied", content: this.buildObjectPage(this.objectRef, ObjectType.Applied)})
        }

        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props = []

        let apiVersion = this.objectRef.version
        if (this.objectRef.group) {
            apiVersion = this.objectRef.group + "/" + this.objectRef.version
        }
        props.push({name: "ApiVersion", value: apiVersion})
        props.push({name: "Kind", value: this.objectRef.kind})

        props.push({name: "Name", value: this.objectRef.name})
        if (this.objectRef.namespace) {
            props.push({ name: "Namespace", value: this.objectRef.namespace })
        }

        const o = findObjectByRef(this.props.commandResult.objects, this.objectRef)
        const annotations: {[key: string]: string} = o?.rendered?.metadata.annotations
        if (annotations) {
            Object.keys(annotations).forEach(k => {
                    if (k.indexOf("kluctl.io/") !== -1) {
                        props.push({ name: k, value: annotations[k] })
                    }
                }
            )
        }

        return <>
            <PropertiesTable properties={props}/>
        </>
    }
}

