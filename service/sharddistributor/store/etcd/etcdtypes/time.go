package etcdtypes

import "time"

// Time is a wrapper around time that implements JSON marshalling/unmarshalling
// in time.RFC3339Nano format to keep precision when storing in etcd.
// Convert to UTC before storing/parsing to ensure consistency.
type Time time.Time

// ToTimePtr converts *time.Time to *Time.
func ToTimePtr(t *time.Time) *Time {
	if t == nil {
		return nil
	}
	tt := Time(*t)
	return &tt
}

// ToTime converts Time back to time.Time.
func (t Time) ToTime() time.Time {
	return time.Time(t)
}

// ToTimePtr converts Time back to *time.Time.
func (t *Time) ToTimePtr() *time.Time {
	if t == nil {
		return nil
	}
	tt := time.Time(*t)
	return &tt
}

// MarshalJSON implements the json.Marshaler interface.
// It encodes the time in time.RFC3339Nano format.
func (t Time) MarshalJSON() ([]byte, error) {
	s := FormatTime(time.Time(t))
	return []byte(`"` + s + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It decodes the time from time.RFC3339Nano format.
func (t *Time) UnmarshalJSON(data []byte) error {
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	parsed, err := ParseTime(str)
	if err != nil {
		return err
	}
	*t = Time(parsed)
	return nil
}

// ParseTime parses a string in time.RFC3339Nano format and returns a time.Time in UTC.
func ParseTime(s string) (time.Time, error) {
	parsed, err := time.ParseInLocation(time.RFC3339Nano, s, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

// FormatTime converts time.Time to UTC and
// formats time.Time to a string in time.RFC3339Nano format.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
