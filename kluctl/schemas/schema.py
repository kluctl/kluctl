import dataclasses
import os

from kluctl.utils.exceptions import InvalidKluctlProjectConfig
from kluctl.utils.yaml_utils import yaml_load_file


def load_schema(name):
    path = os.path.abspath(__file__)
    path = os.path.dirname(path)
    path = os.path.join(path, name)
    return yaml_load_file(path)


kluctl_project_schema = load_schema("kluctl-project.yml")

fixed_images_schema = load_schema("fixed-images.yml")
target_config_schema = load_schema("target-config.yml")

def validate_kluctl_project_config(config):
    from jsonschema import ValidationError
    from jsonschema import validators
    try:
        validator = validators.Draft7Validator(kluctl_project_schema)
        validator.validate(config)
    except ValidationError as e:
        raise InvalidKluctlProjectConfig(str(e), config)


@dataclasses.dataclass
class GitProject:
    url: str
    ref: str
    subdir: str

def parse_git_project(project, default_git_subdir):
    if isinstance(project, str):
        git_url = project
        git_ref = None
        git_subdir = default_git_subdir
    else:
        git_url = project["url"]
        git_ref = project.get("ref")
        git_subdir = project.get("subdir", default_git_subdir)
    return GitProject(git_url, git_ref, git_subdir)
