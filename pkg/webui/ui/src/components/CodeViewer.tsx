import React from 'react';

import { Light as SyntaxHighlighter } from 'react-syntax-highlighter';
import { docco } from 'react-syntax-highlighter/dist/esm/styles/hljs';

import yamlLanguage from 'react-syntax-highlighter/dist/esm/languages/hljs/yaml';
import diffLanguage from 'react-syntax-highlighter/dist/esm/languages/hljs/diff';

SyntaxHighlighter.registerLanguage('yaml', yamlLanguage);
SyntaxHighlighter.registerLanguage('diff', diffLanguage);

export function CodeViewer(props: { code: string, language: string }) {
    return <SyntaxHighlighter language={props.language} style={docco} customStyle={{
        overflowX: undefined, // the parent container is handling scrolling
        flex: "1 1 auto",
        height: "max-content",
    }}>
        {props.code!}
    </SyntaxHighlighter>
}
