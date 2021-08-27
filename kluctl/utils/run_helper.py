import io
import os
import subprocess
import sys
import threading
import time
import traceback


def stdin_write_thread(s, input):
    pos = 0
    try:
        while pos < len(input):
            n = s.write(input[pos:])
            pos += n
        s.close()
    except:
        pass

class StdReader(threading.Thread):
    def __init__(self, stream, do_print, line_mode, cb, capture):
        super().__init__()
        self.stream = stream
        self.do_print = do_print
        self.line_mode = line_mode
        self.cb = cb
        self.capture = capture
        self.line_buf = io.BytesIO()
        self.capture_buf = io.BytesIO()

    def split_std_lines(self, buf):
        self.line_buf.write(buf)
        if len(buf) == 0 or buf.find(b"\n") != -1:
            line_buf_value = self.line_buf.getvalue()
            self.line_buf.seek(0)
            self.line_buf.truncate()

            if line_buf_value:
                lines = line_buf_value.split(b"\n")
                if lines and not lines[-1].endswith(b"\n") and len(buf) != 0:
                    self.line_buf.write(lines[-1])
                    lines = lines[:-1]

                lines = [l.decode("utf-8") for l in lines]
                return lines
        return []

    def do_write(self, buf):
        if self.do_print is not None:
            self.do_print.write(buf.decode("utf-8"))
            self.do_print.flush()
        if self.capture:
            self.capture_buf.write(buf)
        if self.cb is not None:
            if self.line_mode:
                lines = self.split_std_lines(buf)
                if len(lines) > 0:
                    self.cb(lines)
            else:
                if len(buf) > 0:
                    self.cb(buf)

    def run(self) -> None:
        try:
            while True:
                buf = self.stream.read()
                if buf is None:
                    time.sleep(0.1)
                    continue
                self.do_write(buf)
                if len(buf) == 0:
                    break
        except Exception:
            traceback.print_exc()

def set_non_blocking(f):
    if sys.platform == "linux" or sys.platform == "darwin":
        import fcntl
        fd = f.fileno()
        fl = fcntl.fcntl(fd, fcntl.F_GETFL)
        fcntl.fcntl(fd, fcntl.F_SETFL, fl | os.O_NONBLOCK)

def run_helper(args, cwd=None, env=None, input=None,
               stdout_func=None, stderr_func=None,
               print_stdout=False, print_stderr=False,
               line_mode=False, return_std=False):
    stdin = None
    if input is not None:
        stdin = subprocess.PIPE
        if isinstance(input, str):
            input = input.encode('utf-8')
    process = subprocess.Popen(args=args, stdin=stdin, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=cwd, env=env)
    with process:
        stdin_thread = None
        if input is not None:
            stdin_thread = threading.Thread(target=stdin_write_thread, args=(process.stdin, input))
            stdin_thread.start()

        set_non_blocking(process.stdout)
        set_non_blocking(process.stderr)

        stdout_reader = StdReader(process.stdout, sys.stdout if print_stdout else None, line_mode, stdout_func, return_std)
        stderr_reader = StdReader(process.stderr, sys.stderr if print_stderr else None, line_mode, stderr_func, return_std)
        stdout_reader.start()
        stderr_reader.start()

        process.wait()
        stdout_reader.join()
        stderr_reader.join()
        if stdin_thread is not None:
            stdin_thread.join()

        if return_std:
            return process.returncode, stdout_reader.capture_buf.getvalue(), stderr_reader.capture_buf.getvalue()
        return process.returncode
