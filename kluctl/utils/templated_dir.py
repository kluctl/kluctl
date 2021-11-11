import fnmatch
import logging
import os
import shutil

logger = logging.getLogger(__name__)


class TemplatedDir(object):
    def __init__(self, root_dir, rel_source_dir, jinja_env, executor, excluded_patterns):
        self.root_dir = root_dir
        self.rel_source_dir = rel_source_dir
        self.jinja_env = jinja_env
        self.excluded_patterns = excluded_patterns
        self.executor = executor

    def needs_render(self, path):
        for p in self.excluded_patterns:
            if fnmatch.fnmatch(path, p):
                return False
        return True

    def async_render_subdir(self, subdir, target_dir):
        jobs = []

        walk_dir = os.path.join(self.root_dir, self.rel_source_dir, subdir)
        for dirpath, dirnames, filenames in os.walk(walk_dir):
            rel_dirpath = os.path.relpath(dirpath, walk_dir)
            for n in dirnames:
                target_path = os.path.join(target_dir, rel_dirpath, n)
                os.mkdir(target_path)
            for n in filenames:
                source_path = os.path.join(subdir, rel_dirpath, n)
                target_path = os.path.join(target_dir, rel_dirpath, n)
                if self.needs_render(source_path):
                    f = self.executor.submit(self.do_render_file, os.path.join(self.rel_source_dir, source_path), target_path)
                else:
                    f = self.executor.submit(shutil.copy, os.path.join(self.root_dir, self.rel_source_dir, source_path), target_path)
                jobs.append(f)

        return jobs

    def do_render_file(self, source_path, target_path):
        template = self.jinja_env.get_template(source_path.replace("\\", "/"))
        rendered = template.render()
        with open(target_path, "wt") as f:
            f.write(rendered)

    @staticmethod
    def finish_jobs(jobs):
        error = None
        for f in jobs:
            try:
                f.result()
            except Exception as e:
                if error is None:
                    error = e
        if error is not None:
            raise error
