import contextlib
import dataclasses
import gzip
import hashlib
import logging
import os
import re
import shutil
import tarfile
from contextlib import contextmanager
from pathlib import Path
from tempfile import TemporaryDirectory, NamedTemporaryFile
from typing import ContextManager

import jsonschema

from kluctl.schemas.schema import validate_kluctl_project_config, parse_git_project, target_config_schema
from kluctl.utils.dict_utils import copy_dict, get_dict_value, merge_dict
from kluctl.utils.exceptions import InvalidKluctlProjectConfig, CommandError
from kluctl.utils.git_utils import parse_git_url, get_git_commit, get_git_ref, MirroredGitRepo, git_ls_remote
from kluctl.utils.jinja2_utils import render_dict_strs
from kluctl.utils.k8s_cluster_base import load_cluster_config
from kluctl.utils.utils import get_tmp_base_dir, MyThreadPoolExecutor
from kluctl.utils.yaml_utils import yaml_load_file, yaml_save_file

logger = logging.getLogger(__name__)


def load_kluctl_project_config(path):
    try:
        config = yaml_load_file(path)
    except Exception as e:
        raise InvalidKluctlProjectConfig(str(e), None)
    validate_kluctl_project_config(config)

    if "clusters" in config and isinstance(config["clusters"], dict):
        config["clusters"] = [config["clusters"]]

    secret_sets = set()
    for s in get_dict_value(config, "secretsConfig.secretSets", []):
        secret_sets.add(s["name"])
    for target in config.get("targets", []):
        for s in get_dict_value(target, "sealingConfig.secretSets", []):
            if s not in secret_sets:
                raise InvalidKluctlProjectConfig("secretSet %s from target %s does not exist" % (s, target["name"]))

    return config

@dataclasses.dataclass
class GitProjectInfo:
    url: str
    ref: str
    commit: str
    dir: str

@dataclasses.dataclass
class DynamicTargetInfo:
    base_target: dict
    dir: str
    extra_target_config: dict = None
    git_project: object = None
    ref: str = None
    ref_pattern: str = None

class KluctlProject:
    def __init__(self, project_url, project_ref, config_file, local_clusters, local_deployment, local_sealed_secrets, tmp_dir):
        self.project_url = project_url
        self.project_ref = project_ref
        self.config_file = config_file
        self.local_clusters = local_clusters
        self.local_deployment = local_deployment
        self.local_sealed_secrets = local_sealed_secrets

        self.tmp_dir = tmp_dir
        self.config = None
        self.targets = None
        self.involved_repos = {}
        self.mirror_repos = {}
        self.refs_for_urls = {}

    def create_tgz(self, path, metadata_path, reproducible):
        with open(path, mode="wb") as f:
            with gzip.GzipFile(filename="reproducible" if reproducible else None, mode="wb", compresslevel=9, fileobj=f, mtime=0 if reproducible else None) as gz:
                with tarfile.TarFile.taropen("", mode="w", fileobj=gz) as tar:
                    def mf_filter(ti: tarfile.TarInfo):
                        if ".git" in Path(ti.name).parts:
                            return None
                        if reproducible:
                            # make the tar reproducible (always same hash)
                            ti.uid = 0
                            ti.gid = 0
                            ti.mtime = 0
                        return ti

                    metadata_yaml = {
                        "involved_repos": self.involved_repos,
                        "targets": self.targets,
                    }
                    if metadata_path is not None:
                        # metadata.yml outside of archive
                        yaml_save_file(metadata_yaml, metadata_path)
                    else:
                        # metadata.yml as part of the archive
                        with NamedTemporaryFile(dir=get_tmp_base_dir()) as tmp:
                            yaml_save_file(metadata_yaml, tmp.name)
                            tar.add(tmp.name, "metadata.yml", filter=mf_filter)
                    tar.add(self.config_file, ".kluctl.yml", filter=mf_filter)
                    tar.add(self.kluctl_project_dir, "kluctl-project", True, filter=mf_filter)
                    tar.add(self.deployment_dir, "deployment", True, filter=mf_filter)
                    tar.add(self.clusters_dir, "clusters", True, filter=mf_filter)
                    tar.add(self.sealed_secrets_dir, "sealed-secrets", True, filter=mf_filter)

    @staticmethod
    def from_archive(path, metadata_path, tmp_dir):
        if os.path.isfile(path):
            with open(path, mode="rb") as f:
                with tarfile.open(mode="r:gz", fileobj=f) as tgz:
                    tgz.extractall(tmp_dir)
            dir = tmp_dir
        else:
            dir = path
        if metadata_path is not None:
            metadata = yaml_load_file(metadata_path)
        else:
            metadata = yaml_load_file(os.path.join(dir, "metadata.yml"))
        deployment_dir = os.path.join(dir, "deployment")
        project = KluctlProject(None, None,
                                os.path.join(dir, ".kluctl.yml"),
                                os.path.join(dir, "clusters"),
                                deployment_dir,
                                os.path.join(dir, "sealed-secrets"),
                                dir)
        project.involved_repos = metadata["involved_repos"]
        project.targets = metadata["targets"]
        return project

    def build_clone_dir(self, url, ref):
        if ref is None:
            ref = "HEAD"
        ref = ref.replace("/", "-").replace("\\", "-")
        url = parse_git_url(url)
        base_name = os.path.basename(url.path)
        url_hash = hashlib.sha256(("%s:%s" % (url.host, url.path)).encode("utf-8")).hexdigest()
        return os.path.join(self.tmp_dir, "%s-%s" % (base_name, url_hash), ref)

    def clone_git_project(self, git_project_config, default_git_subdir, do_add_involved_repo, do_lock):
        os.makedirs(os.path.join(self.tmp_dir, "git"), exist_ok=True)

        git_project = parse_git_project(git_project_config["project"], default_git_subdir)
        target_dir = self.build_clone_dir(git_project.url, git_project.ref)

        mirror_repo = self.mirror_repos.setdefault(git_project.url, MirroredGitRepo(git_project.url))
        with (mirror_repo.locked() if do_lock else contextlib.suppress()):
            if not mirror_repo.has_updated:
                mirror_repo.update()
            mirror_repo.clone_project(git_project.ref, target_dir)

        git_ref = get_git_ref(target_dir)

        dir = target_dir
        if git_project.subdir is not None:
            dir = os.path.join(dir, git_project.subdir)
        commit = get_git_commit(target_dir)
        info = GitProjectInfo(url=git_project.url, ref=git_ref, commit=commit, dir=dir)
        if do_add_involved_repo:
            self.add_involved_repo(info.url, info.ref, {info.ref: info.commit})
        return info

    def local_project(self, dir):
        return GitProjectInfo(url=None, ref=None, dir=dir, commit=None)

    def clone_kluctl_project(self):
        if self.project_url is None:
            return self.local_project(os.getcwd())

        return self.clone_git_project({
            "project": {
                "url": self.project_url,
                "ref": self.project_ref
            }
        }, None, True, True)

    def add_involved_repo(self, url, ref_pattern, refs):
        s = self.involved_repos.setdefault(url, [])
        e = {
            "ref_pattern": ref_pattern,
            "refs": refs,
        }
        if e not in s:
            s.append(e)

    def load(self, allow_git):
        kluctl_project_info = self.clone_kluctl_project()
        if self.config_file is None:
            c = os.path.join(kluctl_project_info.dir, ".kluctl.yml")
            if os.path.exists(c):
                self.config_file = c
        if self.config_file is not None:
            self.config = load_kluctl_project_config(self.config_file)
        else:
            self.config = {}

        if allow_git:
            self.update_git_caches()

        def do_clone(key, default_git_subdir, local_dir):
            if local_dir is not None:
                return [self.local_project(local_dir)]
            if key not in self.config:
                path = kluctl_project_info.dir
                if default_git_subdir is not None:
                    path = os.path.join(path, default_git_subdir)
                return [self.local_project(path)]

            if not allow_git:
                raise InvalidKluctlProjectConfig("Tried to load something from git while it was not allowed")

            if isinstance(self.config[key], list):
                projects = self.config[key]
            else:
                projects = [self.config[key]]
            ret = []
            for project in projects:
                info = self.clone_git_project(project, default_git_subdir, True, True)
                ret.append(info)
            return ret

        deployment_info = do_clone("deployment", None, self.local_deployment)[0]
        clusters_infos = do_clone("clusters", "clusters", self.local_clusters)
        sealed_secrets_info = do_clone("sealedSecrets", ".sealed-secrets", self.local_sealed_secrets)[0]

        merged_clusters_dir = os.path.join(self.tmp_dir, "merged-clusters")
        self.merge_clusters_dirs(merged_clusters_dir, clusters_infos)

        self.kluctl_project_dir = kluctl_project_info.dir
        self.deployment_dir = deployment_info.dir
        self.clusters_dir = merged_clusters_dir
        self.sealed_secrets_dir = sealed_secrets_info.dir

    def update_git_caches(self):
        with MyThreadPoolExecutor() as executor:
            futures = []
            def do_update_repo(repo):
                with repo.locked():
                    if not repo.has_updated:
                        repo.update()

            def do_update_projects(key):
                if key not in self.config:
                    return
                if isinstance(self.config[key], list):
                    projects = self.config[key]
                else:
                    projects = [self.config[key]]
                for project in projects:
                    url = parse_git_project(project["project"], None).url
                    if url in self.mirror_repos:
                        return
                    mirror_repo = MirroredGitRepo(url)
                    self.mirror_repos[url] = mirror_repo
                    f = executor.submit(do_update_repo, mirror_repo)
                    futures.append(f)

            do_update_projects("deployment")
            do_update_projects("clusters")
            do_update_projects("sealedSecrets")

            for target in self.config.get("targets", []):
                target_config = target.get("targetConfig")
                if target_config is None:
                    continue

                if "project" in target_config:
                    url = parse_git_project(target_config["project"], None).url
                    if url not in self.mirror_repos:
                        mirror_repo = MirroredGitRepo(url)
                        self.mirror_repos[url] = mirror_repo
                        f = executor.submit(do_update_repo, mirror_repo)
                        futures.append(f)
                    if url not in self.refs_for_urls:
                        self.refs_for_urls[url] = executor.submit(git_ls_remote, url)

        for f in futures:
            f.result()
        for url in list(self.refs_for_urls.keys()):
            self.refs_for_urls[url] = self.refs_for_urls[url].result()

    def load_targets(self):
        target_names = set()
        self.targets = []

        target_infos = []
        for base_target in self.config.get("targets", []):
            target_infos += self.prepare_dynamic_targets(base_target)

        self.clone_dynamic_targets(target_infos)

        for target_info in target_infos:
            try:
                target = self.build_dynamic_target(target_info.base_target, target_info.dir)
                if target_info.extra_target_config is not None:
                    merge_dict(target, {"targetConfig": target_info.extra_target_config}, clone=False)
            except Exception as e:
                # Only fail if non-dynamic targets fail to load
                if target_info.ref_pattern is None:
                    raise e
                logger.warning("Failed to load dynamic target config for project. Error=%s" % (str(e)))
                continue

            target = self.render_target(target)
            target["baseTarget"] = target_info.base_target

            if target["name"] in target_names:
                logger.warning("Duplicate target %s" % target["name"])
            else:
                target_names.add(target["name"])
                self.targets.append(target)

    def render_target(self, target):
        errors = []
        # Try rendering the target multiple times, until all values can be rendered successfully. This allows the target
        # to reference itself in complex ways. We'll also try loading the cluster vars in each iteration.
        for i in range(10):
            jinja2_vars = {
                "target": target
            }
            try:
                # Try to load cluster vars. This might fail in case jinja templating is used in the cluster name
                # of the target. We assume that this will then succeed in a later iteration
                cluster_vars, _ = load_cluster_config(self.clusters_dir, target["cluster"], offline=True)
                jinja2_vars["cluster"] = cluster_vars
            except:
                pass
            target2, errors = render_dict_strs(target, jinja2_vars, do_raise=False)
            if not errors and target == target2:
                break
            target = target2
        if errors:
            raise errors[0]
        return target

    def prepare_dynamic_targets(self, base_target):
        target_config = base_target.get("targetConfig")
        if target_config and "project" in target_config:
            return self.prepare_dynamic_targets_external(base_target)
        else:
            return self.prepare_dynamic_targets_simple(base_target)

    def prepare_dynamic_targets_simple(self, base_target):
        if "targetConfig" in base_target:
            target_config = base_target["targetConfig"]
            if "ref" in target_config or "refPattern" in target_config:
                raise InvalidKluctlProjectConfig("'ref' and/or 'refPattern' are not allowed for non-external dynamic targets")

        dynamic_targets = [DynamicTargetInfo(
            base_target=base_target,
            dir=self.kluctl_project_dir,
        )]
        return dynamic_targets

    def prepare_dynamic_targets_external(self, base_target):
        target_config = base_target["targetConfig"]
        git_project = parse_git_project(target_config["project"], None)
        refs = self.refs_for_urls[git_project.url]

        target_config_ref = target_config.get("ref")
        ref_pattern = target_config.get("refPattern")

        if target_config_ref is not None and ref_pattern is not None:
            raise InvalidKluctlProjectConfig("'refPattern' and 'ref' can't be specified together")

        default_branch = None
        for ref, commit in refs.items():
            if ref != "HEAD" and commit == refs["HEAD"]:
                default_branch = ref[len("refs/heads/"):]

        if target_config_ref is None and ref_pattern is None:
            # use default branch of repo
            target_config_ref = default_branch
            if target_config_ref is None:
                raise InvalidKluctlProjectConfig("Git project %s seems to have no default branch" % git_project.url)

        matched_refs = []
        if target_config_ref is not None:
            if "refs/heads/%s" % target_config_ref not in refs:
                raise InvalidKluctlProjectConfig("Git project %s has no ref %s" % (git_project.url, target_config_ref))
            ref_pattern = target_config_ref

        ref_pattern_re = re.compile(r'^refs/heads/%s$' % ref_pattern)
        for ref, commit in refs.items():
            if not ref_pattern_re.match(ref):
                continue
            matched_refs.append(ref[len("refs/heads/"):])

        dynamic_targets = []

        for ref in matched_refs:
            dynamic_targets.append(DynamicTargetInfo(
                base_target=base_target,
                dir=self.build_clone_dir(git_project.url, ref),
                git_project=git_project,
                ref=ref,
                ref_pattern=ref_pattern,
                extra_target_config={
                    "ref": ref,
                    "defaultBranch": default_branch,
                }
            ))

        return dynamic_targets

    def clone_dynamic_targets(self, dynamic_targets):
        @contextmanager
        def lock_all_repos():
            for r in self.mirror_repos.values():
                r.lock()
            try:
                yield
            finally:
                for r in self.mirror_repos.values():
                    r.unlock()

        with lock_all_repos(), MyThreadPoolExecutor(max_workers=8) as executor:
            unique_clones = {}
            for target_info in dynamic_targets:
                if target_info.git_project is None:
                    continue

                if target_info.dir in unique_clones:
                    continue

                f = executor.submit(self.clone_git_project, {
                    "project": {
                        "url": target_info.git_project.url,
                        "ref": target_info.ref,
                    }
                }, None, False, False)
                unique_clones[target_info.dir] = f

            refs_by_url_and_pattern = {}

            for target_info in dynamic_targets:
                if target_info.git_project is None:
                    continue
                info = unique_clones[target_info.dir].result()
                refs_by_url_and_pattern.setdefault(info.url, {}).setdefault(target_info.ref_pattern, {})[info.ref] = info.commit

            for url, ref_patterns in refs_by_url_and_pattern.items():
                for ref_pattern, refs in ref_patterns.items():
                    self.add_involved_repo(url, ref_pattern, refs)

    def build_dynamic_target(self, base_target, dir):
        target = copy_dict(base_target)
        target.setdefault("args", {})
        target.setdefault("images", [])

        if "targetConfig" in base_target:
            target_config = base_target["targetConfig"]
            config_file = target_config.get("file", "target-config.yml")
            config_path = os.path.join(dir, config_file)
            if not os.path.exists(config_path):
                raise InvalidKluctlProjectConfig("No target config file with name %s found in target" % config_file)

            target_config_file = self.load_target_config(config_path)

            # merge args
            for arg_name, value in target_config_file.get("args", {}).items():
                self.check_dynamic_arg(target, arg_name, value)
                target["args"][arg_name] = value

            # We prepend the dynamic images to ensure they get higher priority later
            target["images"] = target_config_file.get("images", []) + target["images"]

        return target

    def load_target_config(self, path):
        target_config = yaml_load_file(path)
        if target_config is None:
            return {}
        jsonschema.validate(target_config, target_config_schema)
        return target_config

    def merge_clusters_dirs(self, merged_clusters_dir, clusters_infos):
        os.makedirs(merged_clusters_dir)
        for c in clusters_infos:
            if not os.path.exists(c.dir):
                logger.warning("Cluster dir '%s' does not exist" % c.dir)
                continue
            for f in os.listdir(c.dir):
                af = os.path.join(c.dir, f)
                if os.path.isfile(af):
                    shutil.copy(af, os.path.join(merged_clusters_dir, f))

    def find_target(self, name):
        for target in self.targets:
            if target["name"] == name:
                return target
        raise InvalidKluctlProjectConfig("Target '%s' not existent in kluctl project config" % name)

    def check_dynamic_arg(self, target, arg_name, arg_value):
        dyn_arg = None
        for x in target.get("dynamicArgs", []):
            if x["name"] == arg_name:
                dyn_arg = x
                break
        if not dyn_arg:
            raise InvalidKluctlProjectConfig(f"Dynamic argument {arg_name} is not allowed for target")

        arg_pattern = dyn_arg.get("pattern", ".*")
        if not re.fullmatch(arg_pattern, arg_value):
            raise InvalidKluctlProjectConfig(f"Dynamic argument {arg_name} does not match required pattern '{arg_pattern}'")


@contextmanager
def load_kluctl_project(project_url, project_ref, config_file,
                        local_clusters=None, local_deployment=None, local_sealed_secrets=None) -> ContextManager[KluctlProject]:
    with TemporaryDirectory(dir=get_tmp_base_dir()) as tmp_dir:
        project = KluctlProject(None, project_url, project_ref, config_file, local_clusters, local_deployment, local_sealed_secrets, tmp_dir)
        yield project

@contextmanager
def load_kluctl_project_from_args(kwargs) -> ContextManager[KluctlProject]:
    with TemporaryDirectory(dir=get_tmp_base_dir()) as tmp_dir:
        if kwargs["from_archive"]:
            if any(kwargs[x] for x in ["project_url", "project_ref", "project_config", "local_clusters", "local_deployment", "local_sealed_secrets"]):
                raise CommandError("--from-archive can not be combined with any other project related option")
            project = KluctlProject.from_archive(kwargs["from_archive"], kwargs["from_archive_metadata"], tmp_dir)
            project.load(False)
        else:
            project = KluctlProject(kwargs["project_url"], kwargs["project_ref"], kwargs["project_config"], kwargs["local_clusters"], kwargs["local_deployment"], kwargs["local_sealed_secrets"], tmp_dir)
            project.load(True)
            project.load_targets()
        yield project
