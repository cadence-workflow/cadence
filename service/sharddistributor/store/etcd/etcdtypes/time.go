package etcdtypes

import "time"

// Time is a wrapper around time that implements JSON marshalling/unmarshalling
// in time.RFC3339Nano format to keep precision when storing in etcd.
// Convert to UTC before storing/parsing to ensure consistency.
type Time time.Time

// ToTime converts Time back to time.Time.
func (t Time) ToTime() time.Time {
	return time.Time(t)
}

// MarshalJSON implements the json.Marshaler interface.
// It encodes the time in time.RFC3339Nano format.
func (t Time) MarshalJSON() ([]byte, error) {
	s := time.Time(t).UTC().Format(time.RFC3339Nano)
	return []byte(`"` + s + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It decodes the time from time.RFC3339Nano format.
func (t *Time) UnmarshalJSON(data []byte) error {
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	parsed, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		return err
	}
	*t = Time(parsed.UTC())
	return nil
}

// ToTime parses a string in time.RFC3339Nano format and returns a time.Time in UTC.
func ToTime(s string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

// FromTime converts time.Time to UTC and
// formats time.Time to a string in time.RFC3339Nano format.
func FromTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
