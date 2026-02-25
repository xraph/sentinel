package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/xraph/grove/drivers/mongodriver/mongomigrate"
	"github.com/xraph/grove/migrate"
)

// Migrations is the grove migration group for the Sentinel mongo store.
var Migrations = migrate.NewGroup("sentinel")

func init() {
	Migrations.MustRegister(
		&migrate.Migration{
			Name:    "create_sentinel_suites_indexes",
			Version: "20240101000001",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				coll := mexec.DB().Collection(colSuites)

				indexes := []mongo.IndexModel{
					{
						Keys: bson.D{
							{Key: "app_id", Value: 1},
							{Key: "name", Value: 1},
						},
						Options: options.Index().SetUnique(true),
					},
					{
						Keys:    bson.D{{Key: "app_id", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_suites_app"),
					},
					{
						Keys:    bson.D{{Key: "created_at", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_suites_created_at"),
					},
				}

				_, err := coll.Indexes().CreateMany(ctx, indexes)
				if err != nil {
					return fmt.Errorf("create sentinel_suites indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				return mexec.DB().Collection(colSuites).Drop(ctx)
			},
		},
		&migrate.Migration{
			Name:    "create_sentinel_cases_indexes",
			Version: "20240101000002",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				coll := mexec.DB().Collection(colCases)

				indexes := []mongo.IndexModel{
					{
						Keys:    bson.D{{Key: "suite_id", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_cases_suite"),
					},
					{
						Keys: bson.D{
							{Key: "suite_id", Value: 1},
							{Key: "scenario_type", Value: 1},
						},
						Options: options.Index().SetName("idx_sentinel_cases_scenario"),
					},
					{
						Keys:    bson.D{{Key: "created_at", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_cases_created_at"),
					},
				}

				_, err := coll.Indexes().CreateMany(ctx, indexes)
				if err != nil {
					return fmt.Errorf("create sentinel_cases indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				return mexec.DB().Collection(colCases).Drop(ctx)
			},
		},
		&migrate.Migration{
			Name:    "create_sentinel_runs_indexes",
			Version: "20240101000003",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				coll := mexec.DB().Collection(colRuns)

				indexes := []mongo.IndexModel{
					{
						Keys: bson.D{
							{Key: "suite_id", Value: 1},
							{Key: "state", Value: 1},
						},
						Options: options.Index().SetName("idx_sentinel_runs_suite"),
					},
					{
						Keys:    bson.D{{Key: "app_id", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_runs_app"),
					},
					{
						Keys:    bson.D{{Key: "created_at", Value: -1}},
						Options: options.Index().SetName("idx_sentinel_runs_created_at"),
					},
				}

				_, err := coll.Indexes().CreateMany(ctx, indexes)
				if err != nil {
					return fmt.Errorf("create sentinel_runs indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				return mexec.DB().Collection(colRuns).Drop(ctx)
			},
		},
		&migrate.Migration{
			Name:    "create_sentinel_results_indexes",
			Version: "20240101000004",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				coll := mexec.DB().Collection(colResults)

				indexes := []mongo.IndexModel{
					{
						Keys:    bson.D{{Key: "run_id", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_results_run"),
					},
					{
						Keys: bson.D{
							{Key: "run_id", Value: 1},
							{Key: "case_id", Value: 1},
						},
						Options: options.Index().SetName("idx_sentinel_results_case"),
					},
					{
						Keys:    bson.D{{Key: "created_at", Value: 1}},
						Options: options.Index().SetName("idx_sentinel_results_created_at"),
					},
				}

				_, err := coll.Indexes().CreateMany(ctx, indexes)
				if err != nil {
					return fmt.Errorf("create sentinel_results indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				return mexec.DB().Collection(colResults).Drop(ctx)
			},
		},
		&migrate.Migration{
			Name:    "create_sentinel_baselines_indexes",
			Version: "20240101000005",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				coll := mexec.DB().Collection(colBaselines)

				indexes := []mongo.IndexModel{
					{
						Keys: bson.D{
							{Key: "suite_id", Value: 1},
							{Key: "is_current", Value: 1},
						},
						Options: options.Index().SetName("idx_sentinel_baselines_suite"),
					},
					{
						Keys:    bson.D{{Key: "created_at", Value: -1}},
						Options: options.Index().SetName("idx_sentinel_baselines_created_at"),
					},
				}

				_, err := coll.Indexes().CreateMany(ctx, indexes)
				if err != nil {
					return fmt.Errorf("create sentinel_baselines indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				return mexec.DB().Collection(colBaselines).Drop(ctx)
			},
		},
		&migrate.Migration{
			Name:    "create_sentinel_prompt_versions_indexes",
			Version: "20240101000006",
			Up: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				coll := mexec.DB().Collection(colPromptVersions)

				indexes := []mongo.IndexModel{
					{
						Keys: bson.D{
							{Key: "suite_id", Value: 1},
							{Key: "version", Value: 1},
						},
						Options: options.Index().SetUnique(true),
					},
					{
						Keys: bson.D{
							{Key: "suite_id", Value: 1},
							{Key: "is_current", Value: 1},
						},
						Options: options.Index().SetName("idx_sentinel_prompts_suite"),
					},
				}

				_, err := coll.Indexes().CreateMany(ctx, indexes)
				if err != nil {
					return fmt.Errorf("create sentinel_prompt_versions indexes: %w", err)
				}
				return nil
			},
			Down: func(ctx context.Context, exec migrate.Executor) error {
				mexec, ok := exec.(*mongomigrate.Executor)
				if !ok {
					return fmt.Errorf("expected mongomigrate executor, got %T", exec)
				}
				return mexec.DB().Collection(colPromptVersions).Drop(ctx)
			},
		},
	)
}
