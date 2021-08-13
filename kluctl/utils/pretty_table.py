from kluctl.utils.utils import runHelper


def pretty_table(table, limit_widths):
    cols = len(table[0])

    def max_width(col, max_w):
        w = 0
        for l in table:
            w = max(w, len(l[col]))
        if max_w != -1:
            w = min(w, max_w)
        return w

    widths = [max_width(i, limit_widths[i] if i < len(limit_widths) else -1) for i in range(cols)]

    if len(limit_widths) < cols:
        # last column should use all remaining space
        try:
            # Try to figure out terminal width
            _, stdout, _ = runHelper(['stty', 'size'], stderr_log_level=None)
            _, term_columns = stdout.split()
            term_columns = int(term_columns)
        except:
            # Probably not a terminal, so let's use some large default
            term_columns = 200
        widths[len(limit_widths)] = term_columns - sum(widths[:-1]) - (cols - 1) * 3 - 4

    horizontal_separator = '+-'
    for i in range(cols):
        horizontal_separator += '-' * widths[i]
        if i != cols - 1:
            horizontal_separator += '-+-'
    horizontal_separator += '-+'

    ret = ''
    ret += horizontal_separator + '\n'
    for l in table:
        pos = [0] * cols

        while any([pos[i] < len(l[i]) for i in range(cols)]):
            ret += '| '
            for i in range(cols):
                t = l[i][pos[i]:pos[i] + widths[i]]
                new_line = t.find('\n')
                if new_line != -1:
                    t = t[:new_line]
                    pos[i] += 1
                pos[i] += len(t)
                ret += t
                ret += ' ' * (widths[i] - len(t))
                if i != cols - 1:
                    ret += ' | '
            ret += ' |\n'

        ret += horizontal_separator + '\n'

    return ret
