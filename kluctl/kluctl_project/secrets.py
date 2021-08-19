import logging
import os

from kluctl.kluctl_project.passwordstate import Passwordstate
from kluctl.utils.exceptions import InvalidKluctlProjectConfig
from kluctl.utils.yaml_utils import yaml_load_file, yaml_load

logger = logging.getLogger(__name__)


class SecretsLoader:
    def __init__(self, kluctl_project, secrets_dir):
        from kluctl.kluctl_project.kluctl_project import KluctlProject
        self.kluctl_project: KluctlProject = kluctl_project
        self.secrets_dir = secrets_dir
        self.passwordstate = Passwordstate()

    def load_secrets(self, secrets_source):
        if "path" in secrets_source:
            return self.load_secrets_file(secrets_source)
        if "passwordstate" in secrets_source:
            return self.load_secrets_passwordstate(secrets_source)
        raise InvalidKluctlProjectConfig("Invalid secrets entry")

    def load_secrets_file(self, secrets_source):
        secrets_path = secrets_source["path"]
        path = None
        if os.path.exists(os.path.join(self.kluctl_project.deployment_dir, secrets_path)):
            path = os.path.join(self.kluctl_project.deployment_dir, secrets_path)
        elif os.path.exists(os.path.join(self.secrets_dir, secrets_path)):
            path = os.path.join(self.secrets_dir, secrets_path)
        if not path or not os.path.exists(path):
            logger.error(f"Secrets file {secrets_path} does not exist")
            raise InvalidKluctlProjectConfig(f"Secrets file {secrets_path} does not exist")

        secrets = yaml_load_file(path)
        return secrets.get('secrets', {})

    def load_secrets_passwordstate(self, secrets_source):
        ps = secrets_source["passwordstate"]
        host = ps["host"]
        if "documentId" in ps:
            document_id = ps["documentId"]
            doc = self.passwordstate.get_document(host, document_id)
        else:
            path = ps["passwordList"]
            title = ps["passwordTitle"]
            field = ps.get("passwordField", "GenericField1")
            l = self.passwordstate.get_password_list(host, path)
            doc = self.passwordstate.get_password(host, l["PasswordListID"], title, field)
        y = yaml_load(doc)
        return y.get("secrets", {})
