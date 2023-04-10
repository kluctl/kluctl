import React from 'react';

import { Light as SyntaxHighlighter } from 'react-syntax-highlighter';
import { docco } from 'react-syntax-highlighter/dist/esm/styles/hljs';

import yamlLanguage from 'react-syntax-highlighter/dist/esm/languages/hljs/yaml';
import diffLanguage from 'react-syntax-highlighter/dist/esm/languages/hljs/diff';

SyntaxHighlighter.registerLanguage('yaml', yamlLanguage);
SyntaxHighlighter.registerLanguage('diff', diffLanguage);

export function CodeViewer(props: { code: string, language: string }) {
    return <SyntaxHighlighter language={props.language} style={docco}>
            {props.code!}
        </SyntaxHighlighter>


    /*return <Box height={"100%"}><Box sx={{
        overflowY: 'scroll', // Enable vertical scrolling
        height: "100%",
        //maxHeight: '100%', // Set a fixed height
        border: '1px solid #ccc', // Optional border
        padding: '10px', // Optional padding
    }}>
        <SyntaxHighlighter language={props.language} style={docco}>
            {props.code!}
        </SyntaxHighlighter>
    </Box></Box>*/
}