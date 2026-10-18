package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/mongodriver/mongomigrate"
	"github.com/xraph/grove/migrate"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
)

const colRecordedJobs = "sentinel_recorded_jobs"

type recordedJobModel struct {
	grove.BaseModel `grove:"table:sentinel_recorded_jobs"`
	ID              string `grove:"id,pk" bson:"_id"`
	JobKey          string `grove:"job_key" bson:"job_key"`
	AppID           string `grove:"app_id" bson:"app_id"`
	TenantID        string `grove:"tenant_id" bson:"tenant_id"`
	Revision        int64  `grove:"revision" bson:"revision"`
	State           string `grove:"state" bson:"state"`
	Payload         string `grove:"payload" bson:"payload"`
}

// CreateRecordedJob persists a request once per exact scope and idempotency key.
func (s *Store) CreateRecordedJob(ctx context.Context, j *recorded.Job) (*recorded.Job, error) {
	if err := j.ValidateNew(ctx); err != nil {
		return nil, err
	}
	data, err := recorded.Encode(ctx, j)
	if err != nil {
		return nil, err
	}
	m := &recordedJobModel{ID: j.ID.String(), JobKey: j.StorageKey(), AppID: j.AppID, TenantID: j.TenantID, Revision: j.Revision, State: string(j.State), Payload: string(data)}
	_, err = s.mdb.NewInsert(m).Exec(ctx)
	if err == nil {
		return recorded.Decode(ctx, data)
	}
	if !mongo.IsDuplicateKeyError(err) {
		return nil, err
	}
	existing := new(recordedJobModel)
	err = s.mdb.NewFind(existing).Filter(bson.M{"job_key": j.StorageKey(), "app_id": j.AppID, "tenant_id": j.TenantID}).Scan(ctx)
	if isNotFound(err) {
		return nil, recorded.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	old, err := recorded.Decode(ctx, []byte(existing.Payload))
	if err != nil {
		return nil, err
	}
	if old.RequestDigest != j.RequestDigest {
		return nil, recorded.ErrConflict
	}
	return old, nil
}

// GetRecordedJob reads only the exact app and tenant's evidence.
func (s *Store) GetRecordedJob(ctx context.Context, jobID id.ID) (*recorded.Job, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	m := new(recordedJobModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": jobID.String(), "app_id": sentinel.AppFromContext(ctx), "tenant_id": sentinel.TenantFromContext(ctx)}).Scan(ctx)
	if isNotFound(err) {
		return nil, recorded.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return recorded.Decode(ctx, []byte(m.Payload))
}

// ListRecordedJobs returns a stable ID page within the exact scope.
func (s *Store) ListRecordedJobs(ctx context.Context, query recorded.Query) ([]*recorded.Job, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	limit, err := query.PageLimit()
	if err != nil {
		return nil, err
	}
	var models []recordedJobModel
	filter := bson.M{"app_id": sentinel.AppFromContext(ctx), "tenant_id": sentinel.TenantFromContext(ctx), "_id": bson.M{"$gt": query.After}}
	if query.Active {
		filter["state"] = bson.M{"$in": bson.A{string(recorded.JobQueued), string(recorded.JobRunning)}}
	}
	err = s.mdb.NewFind(&models).Filter(filter).Sort(bson.D{{Key: "_id", Value: 1}}).Limit(int64(limit)).Scan(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make([]*recorded.Job, 0, len(models))
	for _, m := range models {
		j, decodeErr := recorded.Decode(ctx, []byte(m.Payload))
		if decodeErr != nil {
			return nil, decodeErr
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// UpdateRecordedJob atomically compares the revision before a fenced checkpoint.
func (s *Store) UpdateRecordedJob(ctx context.Context, expected int64, j *recorded.Job) error {
	data, err := recorded.Encode(ctx, j)
	if err != nil {
		return err
	}
	old, err := s.GetRecordedJob(ctx, j.ID)
	if err != nil {
		return err
	}
	if old.Revision != expected {
		return recorded.ErrConflict
	}
	if changeErr := recorded.ValidateChange(ctx, old, j, time.Now().UTC()); changeErr != nil {
		return changeErr
	}
	result, err := s.mdb.Collection(colRecordedJobs).UpdateOne(ctx, bson.M{"_id": j.ID.String(), "app_id": j.AppID, "tenant_id": j.TenantID, "revision": expected}, bson.M{"$set": bson.M{"payload": string(data), "revision": j.Revision, "state": string(j.State)}})
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return recorded.ErrConflict
	}
	return nil
}

func init() {
	Migrations.MustRegister(&migrate.Migration{Name: "create_recorded_jobs", Version: "20240101000008",
		Up: func(ctx context.Context, exec migrate.Executor) error {
			mexec, ok := exec.(*mongomigrate.Executor)
			if !ok {
				return fmt.Errorf("sentinel: expected mongo migration executor, got %T", exec)
			}
			if err := mexec.CreateCollection(ctx, (*recordedJobModel)(nil)); err != nil {
				return err
			}
			return mexec.CreateIndexes(ctx, colRecordedJobs, []mongo.IndexModel{
				{Keys: bson.D{{Key: "job_key", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "app_id", Value: 1}, {Key: "tenant_id", Value: 1}, {Key: "state", Value: 1}, {Key: "_id", Value: 1}}},
			})
		},
		Down: func(ctx context.Context, exec migrate.Executor) error {
			mexec, ok := exec.(*mongomigrate.Executor)
			if !ok {
				return fmt.Errorf("sentinel: expected mongo migration executor, got %T", exec)
			}
			return mexec.DropCollection(ctx, (*recordedJobModel)(nil))
		},
	})
}
