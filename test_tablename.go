package main

import (
	"fmt"
)

// Simple test to verify table name handling
func testTableName() {
	// Test with different database types and table names
	testCases := []struct {
		dbtype    string
		schema    string
		tableName string
		expected  string
	}{
		{"postgres", "public", "actions", "actions"},                // lowercase, public schema
		{"postgres", "public", "Actions", "\"Actions\""},            // uppercase, public schema
		{"postgres", "myschema", "actions", "myschema.actions"},     // lowercase, custom schema
		{"postgres", "myschema", "Actions", "myschema.\"Actions\""}, // uppercase, custom schema
		{"mysql", "", "Actions", "Actions"},                         // MySQL should not quote
		{"sqlite3", "", "Actions", "Actions"},                       // SQLite should not quote
	}

	for _, tc := range testCases {
		config := Config{
			Dbtype: tc.dbtype,
			Schema: tc.schema,
		}

		result := config.GetTableName(tc.tableName)
		fmt.Printf("DB: %s, Schema: %s, Table: %s -> Result: %s (Expected: %s) %s\n",
			tc.dbtype, tc.schema, tc.tableName, result, tc.expected,
			map[bool]string{true: "✓", false: "✗"}[result == tc.expected])
	}
}
