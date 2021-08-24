import logging
import sys

from git import GitCommandError
from jinja2 import TemplateError
from yaml import YAMLError

from kluctl.cli.main_cli_group import cli_group
from kluctl.utils.exceptions import CommandError, InvalidKluctlProjectConfig
from kluctl.utils.jinja2_utils import print_template_error

logger = logging.getLogger(__name__)

config = {}

def main():
    try:
        cli_group(prog_name="kluctl")
    except (CommandError, YAMLError) as e:
        print(e, file=sys.stderr)
        sys.exit(1)
    except InvalidKluctlProjectConfig as e:
        print(e.message, file=sys.stderr)
        sys.exit(1)
    except TemplateError as e:
        print_template_error(e)
        sys.exit(1)
    except GitCommandError as e:
        print(e.stderr)
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
