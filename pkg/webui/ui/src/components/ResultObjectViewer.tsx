import { CommandResult, ObjectRef } from "../models";
import { ObjectType } from "../api";
import React from "react";

import { Loading, useLoadingHelper } from "./Loading";
import { useAppContext } from "./App";
import * as yaml from 'js-yaml';
import { ErrorMessage } from "./ErrorMessage";
import { Box } from "@mui/material";
import { K8sManifestViewer } from "./K8sManifestViewer";

export const ResultObjectViewer = (props: { cr: CommandResult, objectRef: ObjectRef, objectType: ObjectType }) => {
    const appCtx = useAppContext()
    const [loading, error, content] = useLoadingHelper<string>(true, async () => {
        const o = await appCtx.api.getCommandResultObject(props.cr.id, props.objectRef, props.objectType)
        return yaml.dump(o)
    }, [props.cr.id, props.objectRef, props.objectType])

    if (loading) {
        return <Box width={"100%"}>
            <Loading />
        </Box>
    } else if (error) {
        return <ErrorMessage>
            {error.message}
        </ErrorMessage>
    } else {
        return <K8sManifestViewer obj={content}/>
    }
}
