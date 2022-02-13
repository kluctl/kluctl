package uo

func MergeMap(a, b map[string]interface{}) {
	for key := range b {
		if _, ok := a[key]; ok {
			adict, adictOk := a[key].(map[string]interface{})
			bdict, bdictOk := b[key].(map[string]interface{})
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
