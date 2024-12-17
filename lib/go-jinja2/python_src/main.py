import json
import sys
import warnings

from go_jinja2.jinja2_renderer import Jinja2Renderer

with warnings.catch_warnings():
    # jsonpath_ng contains regexes that cause these warnings when importing:
    # SyntaxWarning: invalid escape sequence '\('
    import jsonpath_ng


def main():
    while True:
        cmd = sys.stdin.readline()
        if not cmd:
            break
        cmd = json.loads(cmd)
        opts = cmd["opts"]

        r = Jinja2Renderer(opts)

        result = {}
        if cmd["cmd"] == "init":
            pass
        elif cmd["cmd"] == "render-strings":
            result = {
                "templateResults": r.RenderStrings(cmd["templates"])
            }
        elif cmd["cmd"] == "render-files":
            result = {
                "templateResults": r.RenderFiles(cmd["templates"])
            }
        elif cmd["cmd"] == "exit":
            break
        else:
            raise Exception("invalid cmd")

        result = json.dumps(result)

        sys.stdout.write(result + "\n")
        sys.stdout.flush()

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt as e:
        pass
