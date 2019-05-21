package measurement

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	mpb "github.com/mtraver/environmental-sensor/measurementpb"
)

var (
	testTimestamp = time.Date(2018, time.March, 25, 0, 0, 0, 0, time.UTC)
	pbTimestamp   = mustTimestampProto(testTimestamp)

	testTimestamp2 = time.Date(2018, time.March, 25, 14, 40, 0, 0, time.UTC)
	pbTimestamp2   = mustTimestampProto(testTimestamp2)

	// These cases are used to test conversion in both directions between the generated
	// Measurement type and StorableMeasurement.
	conversionCases = []struct {
		name  string
		m     mpb.Measurement
		sm    StorableMeasurement
		valid bool
	}{
		{"valid_no_upload_timestamp",
			mpb.Measurement{
				DeviceId:  "foo",
				Timestamp: pbTimestamp,
				Temp:      18.5,
			},
			StorableMeasurement{
				DeviceID:  "foo",
				Timestamp: testTimestamp,
				Temp:      18.5,
			},
			true,
		},
		{"valid_with_upload_timestamp",
			mpb.Measurement{
				DeviceId:        "foo",
				Timestamp:       pbTimestamp,
				UploadTimestamp: pbTimestamp2,
				Temp:            18.5,
			},
			StorableMeasurement{
				DeviceID:        "foo",
				Timestamp:       testTimestamp,
				UploadTimestamp: testTimestamp2,
				Temp:            18.5,
			},
			true,
		},
		{"nil_timestamp",
			mpb.Measurement{
				DeviceId:  "foo",
				Timestamp: nil,
				Temp:      18.5,
			},
			StorableMeasurement{},
			false,
		},
	}
)

func mustTimestampProto(t time.Time) *timestamp.Timestamp {
	pbts, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}

	return pbts
}

func TestStorableMeasurementString(t *testing.T) {
	cases := []struct {
		name string
		sm   StorableMeasurement
		want string
	}{
		{"empty", StorableMeasurement{}, " 0.000°C 0001-01-01T00:00:00Z"},
		{"no_upload_timestamp",
			StorableMeasurement{
				DeviceID:  "foo",
				Timestamp: testTimestamp,
				Temp:      18.3748,
			},
			"foo 18.375°C 2018-03-25T00:00:00Z",
		},
		{"upload_timestamp",
			StorableMeasurement{
				DeviceID:        "foo",
				Timestamp:       testTimestamp,
				UploadTimestamp: testTimestamp2,
				Temp:            18.3748,
			},
			"foo 18.375°C 2018-03-25T00:00:00Z (14h40m0s upload delay)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fmt.Sprintf("%v", c.sm)
			if got != c.want {
				t.Errorf("Got %q, want %q", got, c.want)
			}
		})
	}
}

func TestNewStorableMeasurement(t *testing.T) {
	for _, c := range conversionCases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewStorableMeasurement(&c.m)
			if err != nil && c.valid {
				t.Errorf("Unexpected error: %v", err)
				return
			} else if err == nil && !c.valid {
				t.Errorf("Expected error, got no error")
				return
			} else if err != nil && !c.valid {
				// For this case the test has passed. We don't enforce any contract on the first
				// return value of NewStorableMeasurement when the error is non-nil.
				return
			}

			if !reflect.DeepEqual(got, c.sm) {
				t.Errorf("Got %v, want %v", got, c.sm)
			}
		})
	}
}

func TestNewMeasurement(t *testing.T) {
	for _, c := range conversionCases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewMeasurement(&c.sm)
			if err != nil && c.valid {
				t.Errorf("Unexpected error: %v", err)
				return
			} else if err == nil && !c.valid {
				t.Errorf("Expected error, got no error")
				return
			} else if err != nil && !c.valid {
				// For this case the test has passed. We don't enforce any contract on the first
				// return value of NewMeasurement when the error is non-nil.
				return
			}

			if !reflect.DeepEqual(got, c.m) {
				t.Errorf("Got %v, want %v", got, c.m)
			}
		})
	}
}

func TestDBKey(t *testing.T) {
	m := StorableMeasurement{
		DeviceID:  "foo",
		Timestamp: time.Date(2018, time.March, 25, 0, 0, 0, 0, time.UTC),
		Temp:      18.5,
	}

	expected := "foo#2018-03-25T00:00:00Z"
	key := m.DBKey()
	if key != expected {
		t.Errorf("Incorrect DB key. Expected %q, got %q", expected, key)
	}
}
