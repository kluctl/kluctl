import fnmatch
import logging
import os
import shutil

logger = logging.getLogger(__name__)


class TemplatedDir(object):
    def __init__(self, dir, jinja_env, executor, excluded_patterns):
        self.dir = dir
        self.jinja_env = jinja_env
        self.excluded_patterns = excluded_patterns
        self.executor = executor

    def needs_render(self, path):
        for p in self.excluded_patterns:
            if fnmatch.fnmatch(path, p):
                return False
        return True

    def async_render_subdir(self, rel_dir, target_dir):
        jobs = self.do_render_dir(rel_dir, target_dir)
        return jobs

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

    def do_render_dir(self, rel_source_dir, target_dir):
        jobs = []
        abs_source_dir = os.path.join(self.dir, rel_source_dir)
        for name in os.listdir(abs_source_dir):
            rel_source_path = os.path.join(rel_source_dir, name)
            abs_source_path = os.path.join(abs_source_dir, name)
            abs_target_path = os.path.join(target_dir, name)
            if os.path.isdir(abs_source_path):
                os.mkdir(abs_target_path)
                jobs += self.do_render_dir(rel_source_path, abs_target_path)
            elif self.needs_render(rel_source_path):
                f = self.executor.submit(self.do_render_file, rel_source_path, abs_target_path)
                jobs.append(f)
            else:
                shutil.copy(abs_source_path, abs_target_path)

        return jobs

    def do_render_file(self, rel_source_path, abs_target_path):
        template = self.jinja_env.get_template(rel_source_path.replace('\\', '/'))
        rendered = template.render()
        with open(abs_target_path, "wt") as f:
            f.write(rendered)
