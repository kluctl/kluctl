import { useYaml, YamlViewer } from "./YamlViewer";
import React, { useMemo, useState } from "react";
import * as yaml from 'js-yaml';
import { Box, Checkbox, FormControlLabel } from "@mui/material";

export const K8sManifestViewer = (props: {obj: any, initialShowManagedFields?: boolean, initialShowStatus?: boolean}) => {
    const y = useYaml(props.obj)

    const initialShowManagedFields = props.initialShowManagedFields !== undefined ? props.initialShowManagedFields : false
    const initialShowStatus = props.initialShowStatus !== undefined ? props.initialShowStatus : true

    const [showManagedFields, setShowManagedFields] = useState(initialShowManagedFields)
    const [showStatus, setShowStatus] = useState(initialShowStatus)

    const [filtered, hasManagedFields, hasStatus] = useMemo(() => {
        const o = yaml.load(y) as any
        let hasManagedFields = false
        let hasStatus = false
        if (o.metadata.managedFields) {
            hasManagedFields = true
            if (!showManagedFields) {
                delete (o.metadata["managedFields"])
            }
        }
        if (o.status) {
            hasStatus = true
            if (!showStatus) {
                delete (o["status"])
            }
        }
        return [yaml.dump(o), hasManagedFields, hasStatus]
    }, [y, showManagedFields, showStatus])

    return <Box width={"100%"}>
        {hasManagedFields && <FormControlLabel control={<Checkbox defaultChecked={initialShowManagedFields} onChange={e => setShowManagedFields(e.target.checked)} />} label="Show Managed Fields" />}
        {hasStatus && <FormControlLabel control={<Checkbox defaultChecked={initialShowStatus} onChange={e => setShowStatus(e.target.checked)} />} label="Show Status" />}
        <YamlViewer obj={filtered}/>
    </Box>
}
