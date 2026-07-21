package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"ysp.com/ncp/ncp/interfaces/dpcore/model"
)

func TestBuildManagedTableSummaryTreatsMissingSQLiteTableAsEmpty(t *testing.T) {
	summary, err := buildManagedTableSummary(managedTableQuery{
		sample:  model.CertInfo{},
		typeTag: "managed",
		countFn: func() (int64, error) {
			return 0, errors.New("no such table: cert_info")
		},
	})
	require.NoError(t, err)
	require.Equal(t, "cert_info", summary.Name)
	require.Equal(t, int64(0), summary.RowCount)
}

func TestBuildManagedTableSummaryReturnsOtherErrors(t *testing.T) {
	_, err := buildManagedTableSummary(managedTableQuery{
		sample:  model.CertInfo{},
		typeTag: "managed",
		countFn: func() (int64, error) {
			return 0, errors.New("database is locked")
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "database is locked")
}
