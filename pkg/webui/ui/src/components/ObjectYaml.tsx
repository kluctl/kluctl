import { CommandResultProps } from "./result-view/CommandResultView";
import { ObjectRef } from "../models";
import { api, ObjectType, usePromise } from "../api";
import React, { Suspense, useEffect, useState } from "react";
import { CodeViewer } from "./CodeViewer";

import * as yaml from 'js-yaml';
import { Loading } from "./Loading";

export const ObjectYaml = (props: {treeProps: CommandResultProps, objectRef: ObjectRef, objectType: ObjectType}) => {
    const [promise, setPromise] = useState<Promise<string>>()

    useEffect(() => {
        const p = api.getResultObject(props.treeProps.summary.id, props.objectRef, props.objectType)
            .then(yaml.dump)
        setPromise(p)
    }, [props.treeProps, props.objectRef, props.objectType])

    const Content = () => {
        return <CodeViewer code={usePromise(promise)} language={"yaml"}/>
    }

    return <Suspense fallback={<Loading/>}>
        <Content/>
    </Suspense>
}