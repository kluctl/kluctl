from kluctl.utils.dict_utils import get_dict_value, copy_dict, set_dict_value, del_dict_value

o = {
    "a": "v1",
    "b": "v2",
    "c": {
        "d": "v3",
        "e": {
            "f": "v4",
            "xa": "v5",
            "g": "v6",
            "xb": "v7",
        }
    },
    "array": [
        {"a": "v1"},
        {"b": "v2"},
    ]
}


def test_get_dict_value():
    assert get_dict_value(o, "a") == "v1"
    assert get_dict_value(o, "b") == "v2"
    assert get_dict_value(o, "c.d") == "v3"
    assert get_dict_value(o, "c.e.f") == "v4"
    assert get_dict_value(o, "c.e.g") == "v6"

def test_get_dict_value_arrays():
    assert isinstance(get_dict_value(o, "array"), list)
    assert isinstance(get_dict_value(o, "array[0]"), dict)
    assert get_dict_value(o, "array[0].a") == "v1"
    assert get_dict_value(o, "array[1].b") == "v2"

def test_set_dict_value():
    o2 = copy_dict(o)
    set_dict_value(o2, "a", "v1a")
    assert get_dict_value(o2, "a") == "v1a"
    set_dict_value(o2, "c.e.f", "xyz")
    assert get_dict_value(o2, "c.e.f") == "xyz"
    set_dict_value(o2, "array[0].a", "a1")
    assert get_dict_value(o2, "array[0].a") == "a1"

def test_set_dict_value_add():
    o2 = copy_dict(o)
    set_dict_value(o2, "x", "x")
    assert get_dict_value(o2, "x") == "x"
    set_dict_value(o2, "y.x", "yx")
    assert get_dict_value(o2, "y.x") == "yx"
    set_dict_value(o2, "array[2].a", "a1")
    assert get_dict_value(o2, "array[2].a") == "a1"

def test_del_dict_value():
    o2 = copy_dict(o)
    del_dict_value(o2, "a")
    assert "a" not in o2
    assert "b" in o2
    del_dict_value(o2, "c.d")
    assert "d" not in o2["c"]
    del_dict_value(o2, "array[1]")
    assert len(o2["array"]) == 1

def test_del_dict_value_wildcard():
    o2 = copy_dict(o)
    del_dict_value(o2, "*")
    assert o2 == {}
    o2 = copy_dict(o)
    del_dict_value(o2, "c.*")
    assert o2["c"] == {}

def test_del_dict_value_wildcard_extended():
    o2 = copy_dict(o)
    del_dict_value(o2, 'c.e."x*"')
    assert o2["c"]["e"] == {"f": "v4", "g": "v6"}
