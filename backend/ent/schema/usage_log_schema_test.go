package schema

import (
	"strings"
	"testing"

	"entgo.io/ent"
	sqlschema "entgo.io/ent/dialect/sql/schema"
	entmigrate "github.com/Wei-Shaw/sub2api/ent/migrate"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRequestIDAllowsWSBillingKeys(t *testing.T) {
	validators := requireStringFieldValidators(t, UsageLog{}.Fields(), "request_id")

	require.NoError(t, runStringValidators(validators, strings.Repeat("x", 255)))
	require.Error(t, runStringValidators(validators, strings.Repeat("x", 256)))
	require.Error(t, runStringValidators(validators, ""))

	require.Equal(t, 255, requireColumn(t, entmigrate.UsageLogsColumns, "request_id").Size)
}

func requireStringFieldValidators(t *testing.T, fields []ent.Field, name string) []func(string) error {
	t.Helper()

	for _, entField := range fields {
		descriptor := entField.Descriptor()
		if descriptor.Name != name {
			continue
		}
		require.NotEmpty(t, descriptor.Validators, "field %s should include validators", name)

		out := make([]func(string) error, 0, len(descriptor.Validators))
		for _, raw := range descriptor.Validators {
			validator, ok := raw.(func(string) error)
			require.True(t, ok, "field %s validator should be func(string) error", name)
			out = append(out, validator)
		}
		return out
	}

	require.Failf(t, "missing field validator", "schema should include field %s", name)
	return nil
}

func runStringValidators(validators []func(string) error, value string) error {
	for _, validator := range validators {
		if err := validator(value); err != nil {
			return err
		}
	}
	return nil
}

func requireColumn(t *testing.T, columns []*sqlschema.Column, name string) *sqlschema.Column {
	t.Helper()

	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}

	require.Failf(t, "missing column", "migrate schema should include column %s", name)
	return nil
}
