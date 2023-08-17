import React, { useContext, useEffect, useState } from "react";
import { CommandResultSummary } from "../../models";
import { CardBody } from "./Card";
import { ApiContext, AppContext } from "../App";
import { Loading } from "../Loading";
import { ErrorMessage } from "../ErrorMessage";
import { CommandResultNodeData } from "../result-view/nodes/CommandResultNode";
import { doGetRootNode } from "./CommandResultCard";


export const CommandResultSummaryBody = React.memo((props: {
    rs: CommandResultSummary,
    loadData?: boolean
}) => {
    const api = useContext(ApiContext);
    const appContext = useContext(AppContext);

    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<any>();
    const [node, setNode] = useState<CommandResultNodeData>();
    const [prevRsId, setPrevRsId] = useState<string>();

    useEffect(() => {
        let cancelled = false;
        if (!props.loadData) {
            return;
        }

        if (prevRsId === props.rs.id) {
            return;
        }

        const doStartLoading = async () => {
            try {
                setLoading(true);
                const n = await doGetRootNode(api, props.rs, appContext.shortNames);
                if (cancelled) {
                    return;
                }
                setNode(n);
                setPrevRsId(props.rs.id);
            } catch (error) {
                setError(error);
            }
            setLoading(false);
        };

        doStartLoading();

        return () => {
            cancelled = true;
        }
    }, [api, appContext.shortNames, prevRsId, props.loadData, props.rs])

    if (loading) {
        return <Loading />;
    }

    if (error) {
        return <ErrorMessage>
            {error.message}
        </ErrorMessage>;
    }

    if (!node) {
        return null;
    }

    return <CardBody provider={node} />
});
