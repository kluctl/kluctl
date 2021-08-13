

class Inclusion:
    def __init__(self):
        self.includes = set()
        self.excludes = set()

    def add_include(self, type, value):
        self.includes.add((type, value))

    def add_exclude(self, type, value):
        self.excludes.add((type, value))

    def has_type(self, type):
        for t, v in self.includes:
            if t == type:
                return True
        for t, v in self.excludes:
            if t == type:
                return True
        return False

    def _check_list(self, type_and_values, l):
        for t, v in type_and_values:
            if (t, v) in l:
                return True
        return False

    def check_included(self, type_and_values, exclude_if_not_included=False):
        if not self.includes and not self.excludes:
            return True

        is_included = self._check_list(type_and_values, self.includes)
        is_excluded = self._check_list(type_and_values, self.excludes)

        if exclude_if_not_included:
            if not is_included:
                return False

        if is_excluded:
            return False

        return not self.includes or is_included
