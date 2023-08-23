import React from 'react';

import { CommandResult, ObjectRef } from "../../../models";
import { NodeData } from "./NodeData";
import { PublishedWithChanges, Settings, SettingsEthernet, SmartToy, SvgIconComponent } from "@mui/icons-material";
import { PropertiesTable } from "../../PropertiesTable";
import { findObjectByRef, ObjectType } from "../../../api";
import { SidePanelTab } from "../SidePanel";
import { BracketsCurlyIcon } from '../../../icons/Icons';
import { AppContextProps } from "../../App";

const kindMapping: { [key: string]: { icon: SvgIconComponent } } = {
    "/ConfigMap": { icon: Settings },
    "apps/Deployment": { icon: PublishedWithChanges },
    "Service": { icon: SettingsEthernet },
    "ServiceAccount": { icon: SmartToy }
}

export class ObjectNodeData extends NodeData {
    objectRef: ObjectRef

    constructor(commandResult: CommandResult, id: string, objectRef: ObjectRef) {
        super(commandResult, id, true, true);
        this.objectRef = objectRef
    }

    buildSidePanelTitle(): React.ReactNode {
        return this.objectRef.name
    }

    buildIcon(appContext: AppContextProps): [React.ReactNode, string] {
        const sn = appContext.shortNames.find(sn => sn.group === this.objectRef.group && sn.kind === this.objectRef.kind)
        const snStr = sn?.shortName || ""

        const m = kindMapping[this.objectRef.group + "/" + this.objectRef.kind]
        if (m !== undefined) {
            return [React.createElement(m.icon, { fontSize: "large" }), snStr]
        }

        return [<BracketsCurlyIcon />, snStr]
    }

    buildSidePanelTabs(appContext: AppContextProps): SidePanelTab[] {
        const tabs = [
            { label: "Summary", content: this.buildSummaryPage() }
        ]

        this.buildDiffAndHealthPages(tabs)

        if (findObjectByRef(this.commandResult.objects, this.objectRef)?.rendered) {
            tabs.push({ label: "Rendered", content: this.buildObjectPage(this.objectRef, ObjectType.Rendered) })
        }

        if (appContext.user.isAdmin) {
            if (findObjectByRef(this.commandResult.objects, this.objectRef)?.remote) {
                tabs.push({ label: "Remote", content: this.buildObjectPage(this.objectRef, ObjectType.Remote) })
            }
            if (findObjectByRef(this.commandResult.objects, this.objectRef)?.applied) {
                tabs.push({ label: "Applied", content: this.buildObjectPage(this.objectRef, ObjectType.Applied) })
            }
        }

        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props = []

        let apiVersion = this.objectRef.version
        if (this.objectRef.group) {
            apiVersion = this.objectRef.group + "/" + this.objectRef.version
        }
        props.push({ name: "ApiVersion", value: apiVersion })
        props.push({ name: "Kind", value: this.objectRef.kind })

        props.push({ name: "Name", value: this.objectRef.name })
        if (this.objectRef.namespace) {
            props.push({ name: "Namespace", value: this.objectRef.namespace })
        }

        const o = findObjectByRef(this.commandResult.objects, this.objectRef)
        const annotations: { [key: string]: string } = o?.rendered?.metadata.annotations
        if (annotations) {
            Object.keys(annotations).forEach(k => {
                if (k.indexOf("kluctl.io/") !== -1) {
                    props.push({ name: k, value: annotations[k] })
                }
            }
            )
        }

        return <>
            <PropertiesTable properties={props} />
        </>
    }
}

