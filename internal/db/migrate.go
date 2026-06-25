package db

import (
	"context"
	"fmt"

	"alt-bot/ent"
)

func MigrateSchema(ctx context.Context, client *ent.Client) error {
	if err := client.Schema.Create(ctx); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}
	return nil
}