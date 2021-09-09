import ast
import re

LOOSE_SEMVER_REGEX=r"^(([0-9]+)(\.[0-9]+)*)?(.*)$"
SUFFIX_COMPONENT_REGEX = re.compile(r'(\d+ | [a-z]+ | \.)', re.VERBOSE)

"""
Allows to compare ints and strings. Strings are always considered less then ints.
"""
class IntOrString(object):
    def __init__(self, x):
        self.x = x

    @staticmethod
    def less(a, b):
        a_int = isinstance(a, int)
        b_int = isinstance(b, int)
        if a_int == b_int:
            return a < b
        if a_int:
            return False
        else:
            return True

    def __lt__(self, other):
        return IntOrString.less(self.x, other.x)

class LooseVersionComparator(object):
    def __init__(self, v):
        self.version = v

    @staticmethod
    def split_version(v):
        r = re.compile(LOOSE_SEMVER_REGEX)
        m = r.match(v)
        nums = m.group(1)
        suffix = m.group(4)
        nums = [int(x) for x in nums.split('.')] if nums else []
        return nums, suffix

    @staticmethod
    def split_suffix(v):
        components = [x for x in SUFFIX_COMPONENT_REGEX.split(v) if x and x != '.']
        for i, obj in enumerate(components):
            try:
                components[i] = int(obj)
            except ValueError:
                pass
        components = [IntOrString(x) for x in components]
        return components

    @staticmethod
    def less(a, b, prefer_long_suffix=True):
        a_nums, a_suffix = LooseVersionComparator.split_version(a)
        b_nums, b_suffix = LooseVersionComparator.split_version(b)

        if a_nums < b_nums:
            return True
        if a_nums > b_nums:
            return False

        if len(a_suffix) == 0 and len(b_suffix) != 0:
            return False
        elif len(a_suffix) != 0 and len(b_suffix) == 0:
            return True

        a_suffix = LooseVersionComparator.split_suffix(a_suffix)
        b_suffix = LooseVersionComparator.split_suffix(b_suffix)

        for i in range(min(len(a_suffix), len(b_suffix))):
            if a_suffix[i] < b_suffix[i]:
                return True
            if b_suffix[i] < a_suffix[i]:
                return False

        if prefer_long_suffix:
            if len(a_suffix) < len(b_suffix):
                return True
        else:
            if len(b_suffix) < len(a_suffix):
                return True
        return False

    def __lt__(self, other):
        return LooseVersionComparator.less(self.version, other.version)

    @staticmethod
    def compare(a, b):
        if LooseVersionComparator.less(a, b):
            return -1
        if LooseVersionComparator.less(b, a):
            return 1
        return 0

class LatestVersion(object):
    def match(self, version):
        raise NotImplementedError()
    def filter(self, versions):
        return [v for v in versions if self.match(v)]
    def latest(self, versions):
        raise NotImplementedError()

class RegexLatestVersion(LatestVersion):
    def __init__(self, pattern):
        self.pattern_str = pattern
        self.pattern = re.compile(pattern)

    def match(self, version):
        return self.pattern.match(version)

    def latest(self, versions):
        versions = sorted(versions, key=LooseVersionComparator)
        return versions[-1]

    def __str__(self):
        return f"regex(pattern=\"{self.pattern_str}\")"

class LooseSemVerLatestVersion(LatestVersion):
    def __init__(self, allow_no_nums=False):
        self.allow_no_nums = allow_no_nums
        self.pattern = re.compile(LOOSE_SEMVER_REGEX)

    def match(self, version):
        m = self.pattern.match(version)
        if not m:
            return False
        if not self.allow_no_nums and m.group(1) is None:
            return False
        return True

    def latest(self, versions):
        versions = sorted(versions, key=LooseVersionComparator)
        return versions[-1]

    def __str__(self):
        return f"semver(allow_no_nums={self.allow_no_nums})"

class PrefixLatestVersion(LatestVersion):
    def __init__(self, prefix, suffix=RegexLatestVersion('.*')):
        self.prefix = prefix
        self.suffix = suffix
        self.pattern = re.compile(r"^%s(.*)$" % prefix)

    def match(self, version):
        m = self.pattern.match(version)
        if not m:
            return False
        return self.suffix.match(m.group(1))

    def latest(self, versions):
        filtered_versions = []
        suffix_versions = []
        for v in versions:
            m = self.pattern.match(v)
            if m:
                filtered_versions.append(v)
                suffix_versions.append(m.group(1))
        latest = self.suffix.latest(suffix_versions)
        i = suffix_versions.index(latest)
        return filtered_versions[i]

    def __str__(self):
        return f"prefix(prefix=\"{self.prefix}\", suffix={str(self.suffix)})"

class NumberLatestVersion(RegexLatestVersion):
    def __init__(self):
        super().__init__("^[0-9]+$")

    def latest(self, versions):
        versions = [int(x) for x in versions]
        versions.sort()
        return versions[-1]

    def __str__(self):
        return f"number()"

def build_latest_version_from_str(s):
    def do_raise():
        raise ValueError("invalid latest_version filter: %s" % s)

    def parse_ast(a):
        if not isinstance(a, ast.Call):
            do_raise()

        args = []
        kwargs = {}

        def parse_arg(arg):
            if isinstance(arg, ast.Constant):
                return arg.value
            elif isinstance(arg, ast.Call):
                return parse_ast(arg)
            else:
                do_raise()

        for arg in a.args:
            args.append(parse_arg(arg))
        for arg in a.keywords:
            kwargs[arg.arg] = parse_arg(arg.value)

        name = a.func.id
        if name == "regex":
            cls = RegexLatestVersion
        elif name == "semver":
            cls = LooseSemVerLatestVersion
        elif name == "prefix":
            cls = PrefixLatestVersion
        elif name == "number":
            cls = NumberLatestVersion
        else:
            do_raise()
        return cls(*args, **kwargs)

    try:
        a = ast.parse(s, mode="eval")
        if not isinstance(a, ast.Expression):
            do_raise()
        return parse_ast(a.body)
    except ValueError:
        raise
    except Exception as e:
        raise do_raise()
