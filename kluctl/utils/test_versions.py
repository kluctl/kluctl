import unittest

from kluctl.utils.versions import LooseVersionComparator, LooseSemVerLatestVersion, PrefixLatestVersion, \
    NumberLatestVersion

versions_list = [
    "abc",
    "1",
    "1.1",
    "a-1",
    "1.1.1",
    "1.1.1-a",
    "1.1.1-1",
    "b-1-c",
    "b-100",
    "12",
]

class TestLooseVersionComparator(unittest.TestCase):
    def check_equal(self, a, b):
        self.assertFalse(LooseVersionComparator.less(a, b))
        self.assertFalse(LooseVersionComparator.less(b, a))
        self.assertIs(LooseVersionComparator.compare(a, b), 0)

    def check_less(self, a, b):
        self.assertTrue(LooseVersionComparator.less(a, b))
        self.assertFalse(LooseVersionComparator.less(b, a))
        self.assertIs(LooseVersionComparator.compare(a, b), -1)
        self.assertIs(LooseVersionComparator.compare(b, a), 1)

    def test_equality(self):
        self.check_equal('1', '1')
        self.check_equal('1.1', '1.1')
        self.check_equal('1.1a', '1.1a')
        self.check_equal('1.1-a', '1.1-a')
        self.check_equal('1.a-1', '1.a-1')

    def test_less(self):
        self.check_less('1', '2')
        self.check_less('1', '1.1')
        self.check_less('1.1a', '1.1')
        self.check_less('1.1-a', '1.1')
        self.check_less('1.1a', '1.1b')
        self.check_less('1.1-a', '1.1-b')
        self.check_less('1.a-1', '1.a-2')

    def test_maven_versions(self):
        self.check_less('1.1-SNAPSHOT', '1.1')
        self.check_less('1', '1.1')
        self.check_less('1-SNAPSHOT', '1.1')
        self.check_less('1.1', '1.1.1-SNAPSHOT')
        self.check_less('1.1-SNAPSHOT', '1.1.1-SNAPSHOT')

    def test_suffixes(self):
        self.check_less('1.1-1', '1.1-2')
        self.check_less('1.1-suffix-1', '1.1-suffix-2')
        self.check_less('1.1-suffix-2', '1.1-suffix-10')
        self.check_less('1.1-suffix-2', '1.1-suffiy-1')
        self.check_less('1.1-1-1', '1.1-2-1')
        self.check_less('1.1-2-1', '1.1-2-2')
        self.check_less('1.1-2-1', '1.1-100-2')
        self.check_less('1.1-2-1', '1.1')
        self.check_less('1.1-2', '1.1-2-1')
        self.check_less('1.1-a-1', '1.1-1-1')

    def test_loose_version_no_nums(self):
        self.check_less('-snapshot1', '-snapshot2')
        self.check_less('-1', '-2')
        self.check_less('-1.1', '-1.2')

class TestVersionFilters(unittest.TestCase):
    def test_loose_semver(self):
        filtered = LooseSemVerLatestVersion().filter(versions_list)
        self.assertListEqual(filtered, [
            "1",
            "1.1",
            "1.1.1",
            "1.1.1-a",
            "1.1.1-1",
            "12",
        ])

    def test_prefix(self):
        filtered = PrefixLatestVersion("a-").filter(versions_list)
        self.assertListEqual(filtered, [
            "a-1",
        ])

    def test_number(self):
        filtered = NumberLatestVersion().filter(versions_list)
        self.assertListEqual(filtered, [
            "1",
            "12"
        ])