package ini

import (
	"path/filepath"
	"testing"
)

func TestIni(t *testing.T) {
	tests := []struct {
		config      string
		credentials string
		profile     string
		key         string
		want        string
	}{
		{
			filepath.Join(testdataDir(t), "aws_config"),
			filepath.Join(testdataDir(t), "aws_credentials"),
			"default",
			"region",
			"us-east-1",
		},
		{
			filepath.Join(testdataDir(t), "aws_config"),
			filepath.Join(testdataDir(t), "aws_credentials"),
			"k1low",
			"role_arn",
			"arn:aws:iam::111111111111:role/admin-access",
		},
		{
			filepath.Join(testdataDir(t), "aws_config"),
			filepath.Join(testdataDir(t), "aws_credentials"),
			"unknown",
			"region",
			"us-east-1",
		},
		{
			"not_exist",
			filepath.Join(testdataDir(t), "aws_credentials"),
			"default",
			"aws_access_key_id",
			"DUMMYDEFAULT",
		},
		{
			filepath.Join(testdataDir(t), "aws_config"),
			"not_exist",
			"mycompany",
			"region",
			"us-east-1",
		},
	}
	for _, tt := range tests {
		t.Setenv("AWS_CONFIG_FILE", tt.config)
		t.Setenv("AWS_SHARED_CREDENTIALS_FILE", tt.credentials)
		i, err := New()
		if err != nil {
			t.Fatal(err)
		}
		got := i.GetKey(tt.profile, tt.key)
		if got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func testdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "testdata")
}
