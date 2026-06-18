package worker_setting

import "testing"

func TestEffectiveResultAndReferenceStorageTypes(t *testing.T) {
	t.Setenv(DefaultResultLocalStoragePathEnv, "")
	t.Setenv(DefaultReferenceLocalStoragePathEnv, "")
	tests := []struct {
		name                 string
		cfg                  *WorkerSetting
		wantResultType       string
		wantReferenceType    string
		wantResultLocalPath  string
		wantReferenceLocPath string
	}{
		{
			name:                 "falls back to base storage when split fields are empty",
			cfg:                  &WorkerSetting{StorageType: "s3", LocalStoragePath: "/base"},
			wantResultType:       "s3",
			wantReferenceType:    "s3",
			wantResultLocalPath:  DefaultResultLocalStoragePath,
			wantReferenceLocPath: DefaultReferenceLocalStoragePath,
		},
		{
			name: "reference falls back to result fields when reference fields are empty",
			cfg: &WorkerSetting{
				ResultStorageType:      "local",
				ResultLocalStoragePath: "/result",
			},
			wantResultType:       "local",
			wantReferenceType:    "local",
			wantResultLocalPath:  DefaultResultLocalStoragePath,
			wantReferenceLocPath: DefaultReferenceLocalStoragePath,
		},
		{
			name: "reference fields override result fields when explicitly configured",
			cfg: &WorkerSetting{
				ResultStorageType:         "local",
				ResultLocalStoragePath:    "/result",
				ReferenceStorageType:      "s3",
				ReferenceLocalStoragePath: "/reference",
			},
			wantResultType:       "local",
			wantReferenceType:    "s3",
			wantResultLocalPath:  DefaultResultLocalStoragePath,
			wantReferenceLocPath: DefaultReferenceLocalStoragePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveResultStorageType(); got != tt.wantResultType {
				t.Fatalf("EffectiveResultStorageType() = %q, want %q", got, tt.wantResultType)
			}
			if got := tt.cfg.EffectiveReferenceStorageType(); got != tt.wantReferenceType {
				t.Fatalf("EffectiveReferenceStorageType() = %q, want %q", got, tt.wantReferenceType)
			}
			if got := tt.cfg.EffectiveResultLocalStoragePath(); got != tt.wantResultLocalPath {
				t.Fatalf("EffectiveResultLocalStoragePath() = %q, want %q", got, tt.wantResultLocalPath)
			}
			if got := tt.cfg.EffectiveReferenceLocalStoragePath(); got != tt.wantReferenceLocPath {
				t.Fatalf("EffectiveReferenceLocalStoragePath() = %q, want %q", got, tt.wantReferenceLocPath)
			}
		})
	}
}

func TestEffectiveLocalStoragePathsHonorEnvironmentOverrides(t *testing.T) {
	t.Setenv(DefaultResultLocalStoragePathEnv, "/tmp/custom-results")
	t.Setenv(DefaultReferenceLocalStoragePathEnv, "/tmp/custom-references")

	cfg := &WorkerSetting{}
	if got := cfg.EffectiveResultLocalStoragePath(); got != "/tmp/custom-results" {
		t.Fatalf("EffectiveResultLocalStoragePath() = %q, want %q", got, "/tmp/custom-results")
	}
	if got := cfg.EffectiveReferenceLocalStoragePath(); got != "/tmp/custom-references" {
		t.Fatalf("EffectiveReferenceLocalStoragePath() = %q, want %q", got, "/tmp/custom-references")
	}
}

func TestEffectiveCleanupIntervalHoursDefaultsTo24(t *testing.T) {
	cfg := &WorkerSetting{}
	if got := cfg.EffectiveCleanupIntervalHours(); got != 24 {
		t.Fatalf("EffectiveCleanupIntervalHours() = %d, want 24", got)
	}
	cfg.CleanupIntervalHours = 6
	if got := cfg.EffectiveCleanupIntervalHours(); got != 6 {
		t.Fatalf("EffectiveCleanupIntervalHours() = %d, want 6", got)
	}
}
