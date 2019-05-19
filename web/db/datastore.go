package db

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"github.com/mtraver/environmental-sensor/measurement"
	"github.com/mtraver/environmental-sensor/web/cache"
)

const (
	// Used for separating substrings in cache keys. The octothorpe is fine for this because
	// device IDs and timestamps, the two things most likely to be used in keys, can't contain it.
	keySep = "#"

	// Datastore queries are limited to this many entities, and multiple queries
	// are made to fetch all results.
	queryLimit = 1000
)

// cacheKeyLatest returns the cache key of the latest measurement for the given device ID.
func cacheKeyLatest(deviceID string) string {
	return strings.Join([]string{deviceID, "latest"}, keySep)
}

type datastoreDB struct {
	projectID string
	kind      string
	client    *datastore.Client
}

func NewDatastoreDB(projectID string, kind string) (*datastoreDB, error) {
	client, err := datastore.NewClient(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	return &datastoreDB{
		projectID: projectID,
		kind:      kind,
		client:    client,
	}, nil
}

// Save saves the given Measurement to the database. If the Measurement
// already exists in the database it makes no change to the database and
// returns nil as the error.
func (db *datastoreDB) Save(ctx context.Context, m *measurement.Measurement) error {
	sm, err := measurement.NewStorableMeasurement(m)
	if err != nil {
		return err
	}

	key := datastore.NameKey(db.kind, sm.DBKey(), nil)

	// Only store the measurement if it doesn't exist
	_, err = db.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var x measurement.StorableMeasurement
		if err := tx.Get(key, &x); err != datastore.ErrNoSuchEntity {
			return err
		}

		_, err := tx.Put(key, &sm)
		return err
	})

	// Each device has a cache entry for its latest value. Update it.
	if err == nil {
		cache.Set(ctx, cacheKeyLatest(sm.DeviceID), &sm)
	}

	return err
}

func (db *datastoreDB) executeQuery(ctx context.Context, q *datastore.Query) (map[string][]measurement.StorableMeasurement, error) {
	results := make(map[string][]measurement.StorableMeasurement)

	// Don't modify the original query. We'll continue to derive queries from it
	// using a cursor to break apart the whole query into multiple smaller ones.
	derivedQuery := q.Limit(queryLimit)

	for {
		processed := 0

		it := db.client.Run(ctx, derivedQuery)
		for {
			var m measurement.StorableMeasurement
			_, err := it.Next(&m)
			if err == iterator.Done {
				cursor, err := it.Cursor()
				if err != nil {
					return make(map[string][]measurement.StorableMeasurement), err
				}

				// The current query finished, so make a new one that starts
				// where it left off.
				derivedQuery = q.Start(cursor).Limit(queryLimit)
				break
			} else if err != nil {
				return make(map[string][]measurement.StorableMeasurement), err
			}

			if _, ok := results[m.DeviceID]; !ok {
				results[m.DeviceID] = []measurement.StorableMeasurement{}
			}
			results[m.DeviceID] = append(results[m.DeviceID], m)

			processed++
		}

		if processed < queryLimit {
			// The last query returned fewer results than the limit, meaning that a
			// subsequent query would return nothing, so we're done.
			break
		}
	}

	return results, nil
}

// GetMeasurementsSince gets all measurements with a timestamp greater than
// or equal to startTime. It returns a map of device ID (a string) to a
// StorableMeasurement slice, and an error.
func (db *datastoreDB) GetMeasurementsSince(ctx context.Context, startTime time.Time) (map[string][]measurement.StorableMeasurement, error) {
	// Don't need to filter by device ID here because building the map
	// has the effect of sorting by device ID.
	q := datastore.NewQuery(db.kind).Filter("timestamp >=", startTime).Order("timestamp")
	return db.executeQuery(ctx, q)
}

// GetDelayedMeasurementsSince gets all measurements with a non-nil upload timestamp greater than or equal
// to startTime. It returns a map of device ID (a string) to a StorableMeasurement slice, and an error.
func (db *datastoreDB) GetDelayedMeasurementsSince(ctx context.Context, startTime time.Time) (map[string][]measurement.StorableMeasurement, error) {
	// We don't need to filter by device ID here because building the map has the effect of sorting by device ID.
	q := datastore.NewQuery(db.kind).Filter("upload_timestamp >=", startTime).Order("upload_timestamp")
	return db.executeQuery(ctx, q)
}

// GetMeasurementsBetween gets all measurements with a timestamp greater than
// or equal to startTime and less than or equal to endTime. It returns a map
// of device ID (a string) to a StorableMeasurement slice, and an error.
func (db *datastoreDB) GetMeasurementsBetween(ctx context.Context, startTime time.Time, endTime time.Time) (map[string][]measurement.StorableMeasurement, error) {
	// Don't need to filter by device ID here because building the map
	// has the effect of sorting by device ID.
	q := datastore.NewQuery(db.kind).Filter("timestamp >=", startTime).Filter("timestamp <=", endTime).Order("timestamp")
	return db.executeQuery(ctx, q)
}

// GetLatestMeasurements gets the most recent measurement for each of the given
// device IDs. It returns a map of device ID to StorableMeasurement, and an
// error. If no measurement is found for a device ID then the returned map will
// not contain that device ID.
func (db *datastoreDB) GetLatestMeasurements(ctx context.Context, deviceIDs []string) (map[string]measurement.StorableMeasurement, error) {
	latest := make(map[string]measurement.StorableMeasurement)

	for _, id := range deviceIDs {
		if _, ok := latest[id]; ok {
			continue
		}

		cacheKey := cacheKeyLatest(id)

		// Try the cache
		var m measurement.StorableMeasurement
		err := cache.Get(ctx, cacheKey, &m)
		if err != nil && err != cache.ErrCacheMiss {
			return latest, err
		} else if err == nil {
			// Cache hit
			latest[id] = m
			continue
		}

		// Try the Datastore
		q := datastore.NewQuery(db.kind).Filter("device_id =", id).Order("-timestamp").Limit(1)
		it := db.client.Run(ctx, q)
		_, err = it.Next(&m)
		if err == iterator.Done {
			// Nothing found in the Datastore
			continue
		} else if err != nil {
			return latest, err
		}

		latest[id] = m
		cache.Add(ctx, cacheKey, &m)
	}

	return latest, nil
}
