package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/mongodriver/mongomigrate"
	"github.com/xraph/grove/migrate"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/recorded"
)

const colRecordedCursors = "sentinel_recorded_cursors"

type recordedCursorModel struct {
	grove.BaseModel `grove:"table:sentinel_recorded_cursors"`
	ID              string `grove:"id,pk" bson:"_id"`
	AppID           string `grove:"app_id" bson:"app_id"`
	TenantID        string `grove:"tenant_id" bson:"tenant_id"`
	BindingID       string `grove:"binding_id" bson:"binding_id"`
	Revision        int64  `grove:"revision" bson:"revision"`
	Payload         string `grove:"payload" bson:"payload"`
}

// GetRecordedCursor reads only the exact app/tenant binding acknowledgment.
func (s *Store) GetRecordedCursor(ctx context.Context, binding string) (*recorded.SourceCursor, error) {
	if sentinel.AppFromContext(ctx) == "" {
		return nil, recorded.ErrNotFound
	}
	m := new(recordedCursorModel)
	err := s.mdb.NewFind(m).Filter(bson.M{"_id": recorded.CursorStorageKey(ctx, binding), "app_id": sentinel.AppFromContext(ctx), "tenant_id": sentinel.TenantFromContext(ctx), "binding_id": binding}).Scan(ctx)
	if isNotFound(err) {
		return nil, recorded.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return recorded.DecodeCursor(ctx, []byte(m.Payload))
}

// AdvanceRecordedCursor requires a persisted job and an atomic cursor revision.
func (s *Store) AdvanceRecordedCursor(ctx context.Context, req recorded.CursorAdvance) (*recorded.SourceCursor, error) {
	j, err := s.GetRecordedJob(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	current, err := s.GetRecordedCursor(ctx, req.BindingID)
	if errors.Is(err, recorded.ErrNotFound) {
		current = nil
	} else if err != nil {
		return nil, err
	}
	next, err := recorded.NextCursor(ctx, current, j, req, time.Now())
	if err != nil {
		return nil, err
	}
	if current != nil && next.Revision == current.Revision {
		return current, nil
	}
	data, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	m := &recordedCursorModel{ID: recorded.CursorStorageKey(ctx, req.BindingID), AppID: next.AppID, TenantID: next.TenantID, BindingID: next.BindingID, Revision: next.Revision, Payload: string(data)}
	if current == nil {
		_, insertErr := s.mdb.NewInsert(m).Exec(ctx)
		if mongo.IsDuplicateKeyError(insertErr) {
			return nil, recorded.ErrConflict
		}
		if insertErr != nil {
			return nil, insertErr
		}
	} else {
		result, updateErr := s.mdb.Collection(colRecordedCursors).UpdateOne(ctx, bson.M{"_id": m.ID, "app_id": next.AppID, "tenant_id": next.TenantID, "revision": current.Revision}, bson.M{"$set": bson.M{"payload": string(data), "revision": next.Revision}})
		if updateErr != nil {
			return nil, updateErr
		}
		if result.ModifiedCount == 0 {
			return nil, recorded.ErrConflict
		}
	}
	return next, nil
}

func init() {
	Migrations.MustRegister(&migrate.Migration{Name: "create_recorded_cursors", Version: "20240101000009",
		Up: func(ctx context.Context, exec migrate.Executor) error {
			mexec, ok := exec.(*mongomigrate.Executor)
			if !ok {
				return fmt.Errorf("sentinel: expected mongo migration executor, got %T", exec)
			}
			return mexec.CreateCollection(ctx, (*recordedCursorModel)(nil))
		},
		Down: func(ctx context.Context, exec migrate.Executor) error {
			mexec, ok := exec.(*mongomigrate.Executor)
			if !ok {
				return fmt.Errorf("sentinel: expected mongo migration executor, got %T", exec)
			}
			return mexec.DropCollection(ctx, (*recordedCursorModel)(nil))
		},
	})
}
