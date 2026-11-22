package iplookup

// labelTable interns match labels so host maps hold ids instead of strings.
// Id 0 is "" and is never freed.
type labelTable struct {
	byLabel map[string]uint32
	text    []string
	refs    []uint32
	free    []uint32
}

// newLabelTable returns a table whose only interned label is "".
func newLabelTable() labelTable {
	return labelTable{
		byLabel: map[string]uint32{"": 0},
		text:    []string{""},
		refs:    []uint32{1},
	}
}

// intern returns the id for label, incrementing its ref. Id 0 is never freed.
func (t *labelTable) intern(label string) uint32 {
	if label == "" {
		return 0
	}
	if id, ok := t.byLabel[label]; ok {
		t.refs[id]++
		return id
	}

	// Reuse a freed slot so the table does not grow without bound on churn.
	if n := len(t.free); n > 0 {
		id := t.free[n-1]
		t.free = t.free[:n-1]
		t.byLabel[label] = id
		t.text[id] = label
		t.refs[id] = 1
		return id
	}

	id := uint32(len(t.text)) //nolint:gosec // G115: distinct labels on one Helper, not a wire length
	t.byLabel[label] = id
	t.text = append(t.text, label)
	t.refs = append(t.refs, 1)
	return id
}

// unref drops one live use of id. Id 0 is never freed.
func (t *labelTable) unref(id uint32) {
	if id == 0 {
		return
	}
	t.refs[id]--
	if t.refs[id] > 0 {
		return
	}
	delete(t.byLabel, t.text[id])
	t.text[id] = ""
	t.free = append(t.free, id)
}
