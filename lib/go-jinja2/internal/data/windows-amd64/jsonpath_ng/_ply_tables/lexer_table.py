# lexer_table.py. This file automatically created by PLY (version 3.11). Don't edit!
_tabversion   = '3.10'
_lextokens    = set(('DOUBLEDOT', 'ID', 'NAMED_OPERATOR', 'NUMBER', 'WHERE', 'WHERENOT'))
_lexreflags   = 64
_lexliterals  = '*.[]()$,:|&~'
_lexstateinfo = {'INITIAL': 'inclusive', 'singlequote': 'exclusive', 'doublequote': 'exclusive', 'backquote': 'exclusive'}
_lexstatere   = {'INITIAL': [('(?P<t_ID>([a-zA-Z_@]|[\\u4E00-\\u9FA5]|[\\U0001F600-\\U0001F64F])([a-zA-Z0-9_@\\-]|[\\u4E00-\\u9FA5]|[\\U0001F600-\\U0001F64F])*)|(?P<t_NUMBER>-?\\d+)|(?P<t_singlequote>\')|(?P<t_doublequote>")|(?P<t_backquote>`)|(?P<t_newline>\\n)|(?P<t_DOUBLEDOT>\\.\\.)', [None, ('t_ID', 'ID'), None, None, ('t_NUMBER', 'NUMBER'), ('t_singlequote', 'singlequote'), ('t_doublequote', 'doublequote'), ('t_backquote', 'backquote'), ('t_newline', 'newline'), (None, 'DOUBLEDOT')])], 'singlequote': [("(?P<t_singlequote_content>[^'\\\\]+)|(?P<t_singlequote_escape>\\\\.)|(?P<t_singlequote_end>')", [None, ('t_singlequote_content', 'content'), ('t_singlequote_escape', 'escape'), ('t_singlequote_end', 'end')])], 'doublequote': [('(?P<t_doublequote_content>[^"\\\\]+)|(?P<t_doublequote_escape>\\\\.)|(?P<t_doublequote_end>")', [None, ('t_doublequote_content', 'content'), ('t_doublequote_escape', 'escape'), ('t_doublequote_end', 'end')])], 'backquote': [('(?P<t_backquote_escape>\\\\.)|(?P<t_backquote_content>[^`\\\\]+)|(?P<t_backquote_end>`)', [None, ('t_backquote_escape', 'escape'), ('t_backquote_content', 'content'), ('t_backquote_end', 'end')])]}
_lexstateignore = {'backquote': '', 'doublequote': '', 'INITIAL': ' \t', 'singlequote': ''}
_lexstateerrorf = {'backquote': 't_backquote_error', 'doublequote': 't_doublequote_error', 'INITIAL': 't_error', 'singlequote': 't_singlequote_error'}
_lexstateeoff = {}
