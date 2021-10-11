import os
import subprocess

import pytest_kind

pytest_kind.cluster.KIND_VERSION = "v0.11.1"
pytest_kind.cluster.KUBECTL_VERSION = "v1.21.5"

# Same as pytest_kind.cluster.KindCluster.kubectl, but with os.environment properly passed to the subprocess
def my_kubectl(self, *args, **kwargs):
    self.ensure_kubectl()
    return subprocess.check_output(
        [str(self.kubectl_path), *args],
        env={
            **os.environ,
            "KUBECONFIG": str(self.kubeconfig_path),
        },
        encoding="utf-8",
        **kwargs,
    )


pytest_kind.cluster.KindCluster.kubectl = my_kubectl