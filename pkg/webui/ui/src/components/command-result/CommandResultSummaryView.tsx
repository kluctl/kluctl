import React, { useMemo } from "react";
import { CommandResult } from "../../models";
import { CardBody } from "../target-cards-view/Card";
import { NodeBuilder } from "./nodes/NodeBuilder";


export const CommandResultSummaryBody = React.memo((props: {
    cr: CommandResult,
}) => {
    const node = useMemo(() => {
        const builder = new NodeBuilder(props.cr)
        const [node] =  builder.buildRoot()
        return node
    }, [props.cr])

    if (!node) {
        return null;
    }

    return <CardBody provider={node} />
});
