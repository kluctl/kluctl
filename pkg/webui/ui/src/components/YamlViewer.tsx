import React, { useMemo } from "react";
import * as yaml from 'js-yaml';
import { CodeViewer } from "./CodeViewer";

export const useYaml = (obj: any) => {
    const y = useMemo(() => {
        if (typeof obj === "string") {
            return obj
        }
        return yaml.dump(obj)
    }, [obj])
    return y
}

export const YamlViewer = (props: {obj: any}) => {
    const y = useYaml(props.obj)

    return <CodeViewer code={y} language={"yaml"}/>
}
