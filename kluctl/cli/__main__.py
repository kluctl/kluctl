import logging
import sys
import traceback

from jinja2 import TemplateError
from yaml import YAMLError

from kluctl.cli.main_cli_group import cli_group
from kluctl.command_error import CommandError

logger = logging.getLogger(__name__)

config = {}

def main():
    try:
        cli_group(prog_name="kluctl")
    except (CommandError, YAMLError) as e:
        print(e, file=sys.stderr)
        sys.exit(1)
    except TemplateError as e:
        etype, value, tb = sys.exc_info()
        extracted_tb = traceback.extract_tb(tb)
        found_template = None
        for i, s in reversed(list(enumerate(extracted_tb))):
            if not s.filename.endswith(".py"):
                found_template = i
                break
        if found_template is not None:
            traceback.print_list([extracted_tb[found_template]])
            print("%s: %s" % (type(e).__name__, str(e)), file=sys.stderr)
        else:
            traceback.print_exception(etype, value, tb)
        sys.exit(1)
    except Exception as e:
        from kubernetes.dynamic.exceptions import UnauthorizedError
        if isinstance(e, UnauthorizedError):
            logger.error("Failed to authenticate/authorize for kubernetes cluster")
            sys.exit(1)
        else:
            raise

if __name__ == "__main__":
    main()
