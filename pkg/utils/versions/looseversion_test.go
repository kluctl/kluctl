package versions

import (
	"testing"
)

func checkEqual(t *testing.T, a_ string, b_ string) {
	a := LooseVersion(a_)
	b := LooseVersion(b_)
	if a.Less(b, true) {
		t.Errorf("%s < %s should be false", a, b)
	}
	if b.Less(a, true) {
		t.Errorf("%s > %s should be false", b, a)
	}
	if a.Compare(b) != 0 {
		t.Errorf("%s == %s should be true", a, b)
	}
}

func checkLess(t *testing.T, a_ string, b_ string) {
	a := LooseVersion(a_)
	b := LooseVersion(b_)
	if !a.Less(b, true) {
		t.Errorf("%s < %s should be true", a, b)
	}
	if b.Less(a, true) {
		t.Errorf("%s < %s should be false", b, a)
	}
	if a.Compare(b) != -1 {
		t.Errorf("%s.Compare(%s) should be -1", a, b)
	}
	if b.Compare(a) != 1 {
		t.Errorf("%s.Compare(%s) should be 1", b, a)
	}
}

func TestEquality(t *testing.T) {
	checkEqual(t, "1", "1")
	checkEqual(t, "1.1", "1.1")
	checkEqual(t, "1.1a", "1.1a")
	checkEqual(t, "1.1-a", "1.1-a")
	checkEqual(t, "1.a-1", "1.a-1")
}

func TestLess(t *testing.T) {
	checkLess(t, "1", "2")
	checkLess(t, "1", "1.1")
	checkLess(t, "1.1a", "1.1")
	checkLess(t, "1.1-a", "1.1")
	checkLess(t, "1.1a", "1.1b")
	checkLess(t, "1.1-a", "1.1-b")
	checkLess(t, "1.a-1", "1.a-2")
}

func TestMavenVersions(t *testing.T) {
	checkLess(t, "1.1-SNAPSHOT", "1.1")
	checkLess(t, "1", "1.1")
	checkLess(t, "1-SNAPSHOT", "1.1")
	checkLess(t, "1.1", "1.1.1-SNAPSHOT")
	checkLess(t, "1.1-SNAPSHOT", "1.1.1-SNAPSHOT")
}

func TestSuffixes(t *testing.T) {
	checkLess(t, "1.1-1", "1.1-2")
	checkLess(t, "1.1-suffix-1", "1.1-suffix-2")
	checkLess(t, "1.1-suffix-2", "1.1-suffix-10")
	checkLess(t, "1.1-suffix-2", "1.1-suffiy-1")
	checkLess(t, "1.1-1-1", "1.1-2-1")
	checkLess(t, "1.1-2-1", "1.1-2-2")
	checkLess(t, "1.1-2-1", "1.1-100-2")
	checkLess(t, "1.1-2-1", "1.1")
	checkLess(t, "1.1-2", "1.1-2-1")
	checkLess(t, "1.1-a-1", "1.1-1-1")
}

func TestLooseVersionNoNums(t *testing.T) {
	checkLess(t, "-snapshot1", "-snapshot2")
	checkLess(t, "-1", "-2")
	checkLess(t, "-1.1", "-1.2")
}

func TestLooseVersionVPrefix(t *testing.T) {
	checkLess(t, "v1.0", "v1.1")
	checkLess(t, "v2.0", "v2.1")
	checkLess(t, "v1.0", "v2.0")
	checkLess(t, "1.0", "v2.0")
	checkLess(t, "v1.0suffix", "v1.0")
	checkLess(t, "v1.0-suffix", "v1.0")
	checkLess(t, "v1.0suffix1", "v1.0suffix2")
	checkLess(t, "v1.0-suffix1", "v1.0-suffix2")
}
