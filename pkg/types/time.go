package types

import (
	"encoding/json"
	"time"
)

type JsonTime string

func FromTime(t time.Time) JsonTime {
	return JsonTime(t.Format(time.RFC3339Nano))
}

func (t JsonTime) ToTime() (time.Time, error) {
	t2, err := time.Parse(time.RFC3339Nano, string(t))
	if err != nil {
		return time.Time{}, err
	}
	return t2, nil
}

func (t *JsonTime) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	t2 := JsonTime(s)
	_, err = t2.ToTime()
	if err != nil {
		return err
	}
	*t = t2
	return nil
}

func (t JsonTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(t))
}
