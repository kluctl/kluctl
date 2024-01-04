package uo

func MergeMap(a, b map[string]any) {
	for key := range b {
		if _, ok := a[key]; ok {
			adict, adictOk := getDict(a[key])
			bdict, bdictOk := getDict(b[key])
			if adictOk && bdictOk {
				MergeMap(adict, bdict)
			} else {
				a[key] = b[key]
			}
		} else {
			a[key] = b[key]
		}
	}
}

func getDict(o any) (map[string]any, bool) {
	if d, ok := o.(map[string]any); ok {
		return d, true
	}
	if x, ok := o.(*UnstructuredObject); ok {
		return x.Object, true
	}
	return nil, false
}
