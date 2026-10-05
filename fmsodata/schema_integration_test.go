//go:build filemaker

// Integration test of table and index changes against a real FileMaker
// Server. It runs only with
//
//	FMS_TEST_HOST=https://fms.example.com FMS_TEST_DATABASE=... \
//	FMS_TEST_USERNAME=... FMS_TEST_PASSWORD=... go test -tags filemaker ./fmsodata
//
// The account needs to create and delete tables (fmodata and schema
// privileges). It creates and deletes the table TestTable_Integration.
package fmsodata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSchemaModificationIntegration(t *testing.T) {
	config := ClientConfig{
		Host:     os.Getenv("FMS_TEST_HOST"),
		Database: os.Getenv("FMS_TEST_DATABASE"),
		Username: os.Getenv("FMS_TEST_USERNAME"),
		Password: os.Getenv("FMS_TEST_PASSWORD"),
		Timeout:  30 * time.Second,
	}
	if config.Host == "" || config.Database == "" || config.Username == "" {
		t.Skip("set FMS_TEST_HOST, FMS_TEST_DATABASE, FMS_TEST_USERNAME and FMS_TEST_PASSWORD to run")
	}

	client := NewClient(config)
	ctx := context.Background()

	tableName := "TestTable_Integration"

	// Clean up before test just in case
	_ = client.DeleteTable(ctx, tableName)

	// 1. Create Table
	t.Run("CreateTable", func(t *testing.T) {
		tableDef := TableDefinition{
			TableName: tableName,
			Fields: []FieldDefinition{
				{Name: "ID", Type: "NUMERIC", Primary: true, Unique: true},
				{Name: "Name", Type: "VARCHAR"},
				{Name: "Description", Type: "VARCHAR"},
			},
		}
		err := client.CreateTable(ctx, tableDef)
		assert.NoError(t, err, "Failed to create table")
	})

	// 2. Create Index
	t.Run("CreateIndex", func(t *testing.T) {
		err := client.CreateIndex(ctx, tableName, "Name")
		assert.NoError(t, err, "Failed to create index")
	})

	// 3. Verify Table Exists (by trying to create a record)
	t.Run("VerifyTableExists", func(t *testing.T) {
		record := map[string]interface{}{
			"ID":          1,
			"Name":        "Test Item",
			"Description": "This is a test item",
		}
		_, err := client.CreateRecord(ctx, tableName, record)
		assert.NoError(t, err, "Failed to create record in new table")
	})

	// 4. Delete Index
	t.Run("DeleteIndex", func(t *testing.T) {
		err := client.DeleteIndex(ctx, tableName, "Name")
		assert.NoError(t, err, "Failed to delete index")
	})

	// 5. Delete Table
	t.Run("DeleteTable", func(t *testing.T) {
		err := client.DeleteTable(ctx, tableName)
		assert.NoError(t, err, "Failed to delete table")
	})
}
