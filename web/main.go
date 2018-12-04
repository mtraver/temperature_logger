package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"

	"google.golang.org/appengine"
	gaelog "google.golang.org/appengine/log"

	"github.com/mtraver/environmental-sensor/web/db"
	"github.com/mtraver/environmental-sensor/web/device"
	"github.com/mtraver/environmental-sensor/web/measurement"
)

// Data up to this many hours old will be plotted
const defaultDataDisplayAgeHours = 12

var (
	projectID = mustGetenv("GOOGLE_CLOUD_PROJECT")

	// This environment variable should be defined in app.yaml.
	iotcoreRegistry = mustGetenv("IOTCORE_REGISTRY")

	// Parse and cache all templates at startup instead of loading on each request
	templates = template.Must(template.New("index.html").Funcs(
		template.FuncMap{
			"millis": func(t time.Time) int64 {
				return t.Unix() * 1000
			},
			"RFC3339": func(t time.Time) string {
				return t.Format(time.RFC3339)
			},
		}).ParseGlob("templates/*"))
)

func mustGetenv(varName string) string {
	val := os.Getenv(varName)
	if val == "" {
		log.Fatalf("Environment variable must be set: %v\n", varName)
	}
	return val
}

// This is the structure of the JSON payload pushed to the endpoint by
// Cloud Pub/Sub. See https://cloud.google.com/pubsub/docs/push.
type pushRequest struct {
	Message struct {
		Attributes map[string]string
		Data       []byte
		ID         string `json:"message_id"`
	}
	Subscription string
}

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/_ah/push-handlers/telemetry", pushHandler)

	appengine.Main()
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure that we only serve the root.
	// From https://golang.org/pkg/net/http/#ServeMux:
	//   Note that since a pattern ending in a slash names a rooted subtree, the
	//   pattern "/" matches all paths not matched by other registered patterns,
	//   not just the URL with Path == "/".
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx := appengine.NewContext(r)

	// By default display data up to defaultDataDisplayAgeHours hours old
	hoursAgo := defaultDataDisplayAgeHours
	endTime := time.Now().UTC()
	startTime := endTime.Add(-time.Duration(hoursAgo) * time.Hour)

	// These control which HTML forms are auto-filled when the page loads, to
	// reflect the data that is being displayed
	fillRangeForm := false
	fillHoursAgoForm := true

	if r.Method == "POST" {
		switch formName := r.FormValue("form-name"); formName {
		case "range":
			var rangeErr error
			startTime, rangeErr = time.Parse(time.RFC3339Nano,
				r.FormValue("startdate-adjusted"))
			if rangeErr != nil {
				http.Error(w, fmt.Sprintf("Bad start time: %v", rangeErr),
					http.StatusBadRequest)
				return
			}

			endTime, rangeErr = time.Parse(time.RFC3339Nano,
				r.FormValue("enddate-adjusted"))
			if rangeErr != nil {
				http.Error(w, fmt.Sprintf("Bad end time: %v", rangeErr),
					http.StatusBadRequest)
				return
			}

			fillRangeForm = true
			fillHoursAgoForm = false
		case "hoursago":
			var hoursAgoErr error
			hoursAgo, hoursAgoErr = strconv.Atoi(r.FormValue("hoursago"))
			if hoursAgoErr != nil {
				http.Error(w, fmt.Sprintf("%v", hoursAgoErr), http.StatusBadRequest)
				return
			}

			if hoursAgo < 1 {
				http.Error(w, fmt.Sprintf("Hours ago must be >= 1"),
					http.StatusBadRequest)
				return
			}

			endTime = time.Now().UTC()
			startTime = endTime.Add(-time.Duration(hoursAgo) * time.Hour)

			fillRangeForm = false
			fillHoursAgoForm = true
		default:
			http.Error(w, fmt.Sprintf("Unknown form name"), http.StatusBadRequest)
			return
		}
	}

	database := db.NewDatastoreDB(projectID)

	// Get measurements and marshal to JSON for use in the template
	measurements, err := database.GetMeasurementsBetween(ctx, startTime, endTime)
	jsonBytes := []byte{}
	if err != nil {
		gaelog.Errorf(ctx, "Error fetching data: %v", err)
	} else {
		jsonBytes, err = measurement.MeasurementMapToJSON(measurements)
		if err != nil {
			gaelog.Errorf(ctx, "Error marshaling measurements to JSON: %v", err)
		}
	}

	// Get the latest measurement for each device
	var latest map[string]measurement.StorableMeasurement
	ids, latestErr := device.GetDeviceIDs(ctx, projectID, iotcoreRegistry)
	if latestErr != nil {
		gaelog.Errorf(ctx, "Error getting device IDs: %v", latestErr)
	} else {
		latest, latestErr = database.GetLatestMeasurements(ctx, ids)

		if latestErr != nil {
			gaelog.Errorf(ctx, "Error getting latest measurements: %v", latestErr)
		}
	}

	data := struct {
		Measurements     template.JS
		Error            error
		StartTime        time.Time
		EndTime          time.Time
		HoursAgo         int
		FillRangeForm    bool
		FillHoursAgoForm bool
		Latest           map[string]measurement.StorableMeasurement
		LatestError      error
	}{
		Measurements:     template.JS(jsonBytes),
		Error:            err,
		StartTime:        startTime,
		EndTime:          endTime,
		HoursAgo:         hoursAgo,
		FillRangeForm:    fillRangeForm,
		FillHoursAgoForm: fillHoursAgoForm,
		Latest:           latest,
		LatestError:      latestErr,
	}

	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		gaelog.Errorf(ctx, "Could not execute template: %v", err)
	}
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	msg := &pushRequest{}
	if err := json.NewDecoder(r.Body).Decode(msg); err != nil {
		gaelog.Criticalf(ctx, "Could not decode body: %v\n", err)
		http.Error(w, fmt.Sprintf("Could not decode body: %v", err),
			http.StatusBadRequest)
		return
	}

	m := &measurement.Measurement{}
	if err := proto.Unmarshal(msg.Message.Data, m); err != nil {
		gaelog.Criticalf(ctx, "Failed to unmarshal protobuf: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to unmarshal protobuf: %v", err),
			http.StatusBadRequest)
		return
	}

	if err := m.Validate(); err != nil {
		gaelog.Errorf(ctx, "%v", err)

		// Pub/Sub will only stop re-trying the message if it receives a status 200.
		// The docs say that any of 200, 201, 202, 204, or 102 will have this effect
		// (https://cloud.google.com/pubsub/docs/push), but the local emulator
		// doesn't respect anything other than 200, so return 200 just to be safe.
		// TODO(mtraver) I'd rather return e.g. 202 (http.StatusAccepted) to
		// indicate that it was successfully received but not that all is ok.
		w.WriteHeader(http.StatusOK)
		return
	}

	database := db.NewDatastoreDB(projectID)
	if err := database.Save(ctx, m); err != nil {
		gaelog.Errorf(ctx, "Failed to save measurement: %v\n", err)
	}

	w.WriteHeader(http.StatusOK)
}