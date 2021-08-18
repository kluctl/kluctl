import logging
import os
import tempfile

import yaml

from kluctl.utils.exceptions import CommandError
from kluctl.seal.passwordstate import Passwordstate
from kluctl.seal.sealer import Sealer
from kluctl.utils.external_args import check_required_args
from kluctl.utils.jinja2_utils import render_file, render_str, render_dict_strs
from kluctl.utils.k8s_cluster_base import get_cluster, get_k8s_cluster
from kluctl.utils.utils import merge_dict, get_tmp_base_dir
from kluctl.utils.yaml_utils import yaml_load_file, yaml_load

logger = logging.getLogger(__name__)

SEALME_EXT = ".sealme"

class SealFailedException(Exception):
    pass

passwordstate = Passwordstate()

class SealCluster:
    def __init__(self, deployment_dir, sealme_conf_dir, sealed_secrets_dir, secrets_dir, seal_args, force_reseal):
        self.cluster = get_cluster()
        self.k8s_cluster = get_k8s_cluster()
        self.deployment_dir = os.path.abspath(deployment_dir)
        self.sealme_conf_dir = os.path.abspath(sealme_conf_dir)
        self.sealed_secrets_dir = os.path.abspath(sealed_secrets_dir)
        self.secrets_dir = os.path.abspath(secrets_dir)
        self.conf = yaml_load_file(os.path.join(self.sealme_conf_dir, "sealme-conf.yml"))
        self.guess_sealed_secrets_controller()
        self.sealer = Sealer(self.cluster, self.sealed_secrets_namespace, self.sealed_secrets_controller_name, force_reseal)

        self.seal_args = check_required_args(self.conf.get("args", []), seal_args)

    def guess_sealed_secrets_controller(self):
        self.sealed_secrets_namespace = self.conf.get('sealedSecretsNamespace', 'kube-system')
        self.sealed_secrets_controller_name = self.conf.get('sealedSecretsController', 'sealed-secrets')

    def build_seal_matrix_vars(self, seal_args, seal_secrets):
        cluster = get_cluster()
        jinja_vars = {
            'cluster': cluster,
            'args': merge_dict(self.seal_args, seal_args),
            'secrets': seal_secrets,
        }

        vars = self.conf.get('vars')
        if vars is None:
            vars = []
        if not isinstance(vars, list):
            raise SealFailedException("vars must be a list")

        vars = render_dict_strs(vars, jinja_vars)

        for v in vars:
            if "values" in v:
                merge_dict(jinja_vars, v['values'], False)
            elif "file" in v:
                rel_file = os.path.relpath(os.path.join(self.sealme_conf_dir, v["file"]), self.deployment_dir)
                j = render_file(self.deployment_dir, rel_file, jinja_vars)
                j = yaml_load(j)
                merge_dict(jinja_vars, j, False)

        return jinja_vars

    def load_secrets(self, secrets_entry):
        if "path" in secrets_entry:
            return self.load_secrets_file(secrets_entry)
        if "passwordstate" in secrets_entry:
            return self.load_secrets_passwordstate(secrets_entry)
        raise SealFailedException("Invalid secrets entry")

    def load_secrets_file(self, secrets_entry):
        secrets_path = secrets_entry["path"]
        if os.path.exists(os.path.join(self.sealme_conf_dir, secrets_path)):
            path = os.path.join(self.sealme_conf_dir, secrets_path)
        elif os.path.exists(os.path.join(self.secrets_dir, secrets_path)):
            path = os.path.join(self.secrets_dir, secrets_path)
        else:
            path = os.path.join(self.deployment_dir, secrets_path)
        if not os.path.exists(path):
            logger.error(f"Secrets file {secrets_path} does not exist")
            raise SealFailedException()

        secrets = yaml_load_file(path)
        return secrets.get('secrets', {})

    def load_secrets_passwordstate(self, secrets_entry):
        ps = secrets_entry["passwordstate"]
        host = ps["host"]
        if "documentId" in ps:
            document_id = ps["documentId"]
            doc = passwordstate.get_document(host, document_id)
        else:
            path = ps["passwordList"]
            title = ps["passwordTitle"]
            field = ps.get("passwordField", "GenericField1")
            l = passwordstate.get_password_list(host, path)
            doc = passwordstate.get_password(host, l["PasswordListID"], title, field)
        y = yaml.safe_load(doc)
        return y.get("secrets", {})


    def seal_matrix_entry(self, jinja_vars):
        abs_deployment_dir = os.path.abspath(self.deployment_dir)
        deployment_name = os.path.basename(abs_deployment_dir)
        output_suffix = render_str(self.conf['outputPattern'], jinja_vars)

        for dirpath, dirnames, filenames in os.walk(self.sealme_conf_dir):
            rel_dir = os.path.relpath(dirpath, self.deployment_dir)
            target_dir = os.path.join(self.sealed_secrets_dir, deployment_name, rel_dir)
            for f in filenames:
                if f.endswith(SEALME_EXT):
                    target_file = os.path.join(target_dir, output_suffix, f)
                    target_file = target_file[:-len(SEALME_EXT)]
                    self.seal_file(os.path.join(rel_dir, f), target_file, jinja_vars)

    def seal_file(self, source_file, target_file, jinja_vars):
        with tempfile.NamedTemporaryFile(dir=get_tmp_base_dir(), delete=False) as tmp:
            try:
                s = render_file(self.deployment_dir, source_file, jinja_vars)
                tmp.write(s.encode('utf-8'))
                tmp.close()

                self.sealer.seal_file(tmp.name, target_file)
            except Exception as e:
                err_txt = ''
                if not isinstance(e, SealFailedException):
                    err_txt = ' ' + str(e)
                logger.error(f"Failed sealing {os.path.basename(source_file)}.{err_txt}")
            finally:
                os.unlink(tmp.name)

    def check_rules(self, matrix_entry, jinja_vars):
        all_match = True
        for rule in matrix_entry.get('rules', []):
            rendered_rule = render_str(rule, jinja_vars)
            if rendered_rule != 'True':
                all_match = False
                break
        return all_match

    def seal_matrix(self):
        for i, a in enumerate(self.conf.get('secretsMatrix', [])):
            seal_args_matrix = a.get("argsMatrix", [])
            if "args" in a:
                seal_args_matrix.append(a["args"])
            if not seal_args_matrix:
                seal_args_matrix.append({})
            for seal_args in seal_args_matrix:
                secrets = {}

                try:
                    logger.info(f"Evaluating rules with arguments: {seal_args}")
                    jinja_vars = self.build_seal_matrix_vars(seal_args, {})
                    if not self.check_rules(a, jinja_vars):
                        continue

                    if 'secrets' in a:
                        if type(a['secrets']) is not list:
                            raise CommandError("secrets must be a list")
                        for s in a['secrets']:
                            jinja_vars = self.build_seal_matrix_vars(seal_args, {})
                            s2 = render_dict_strs(s, jinja_vars)
                            secrets2 = self.load_secrets(s2)
                            merge_dict(secrets, secrets2, False)

                    logger.info(f"Sealing with arguments: {seal_args}")

                    jinja_vars = self.build_seal_matrix_vars(seal_args, secrets)
                    self.seal_matrix_entry(jinja_vars)
                except Exception as e:
                    logger.error(f"Failed sealing matrix entry with index {i}. {type(e).__name__}(\"{str(e)}\")")
