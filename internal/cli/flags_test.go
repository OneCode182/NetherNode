package cli

import (
	"reflect"
	"testing"
)

func TestExtractBoolFlag(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		flagName string
		wantFlag bool
		wantRest []string
	}{
		{"absent", []string{"difficulty", "hard"}, "apply", false, []string{"difficulty", "hard"}},
		{"trailing", []string{"difficulty", "hard", "--apply"}, "apply", true, []string{"difficulty", "hard"}},
		{"leading", []string{"--apply", "difficulty", "hard"}, "apply", true, []string{"difficulty", "hard"}},
		{"empty", nil, "apply", false, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFlag, gotRest := extractBoolFlag(tc.args, tc.flagName)
			if gotFlag != tc.wantFlag {
				t.Errorf("flag = %v, want %v", gotFlag, tc.wantFlag)
			}
			if !reflect.DeepEqual(gotRest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", gotRest, tc.wantRest)
			}
		})
	}
}

func TestExtractIntFlag(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		def      int
		wantVal  int
		wantRest []string
		wantErr  bool
	}{
		{"absent uses default", []string{"Steve"}, 4, 4, []string{"Steve"}, false},
		{"trailing space form", []string{"Steve", "--level", "3"}, 4, 3, []string{"Steve"}, false},
		{"leading space form", []string{"--level", "3", "Steve"}, 4, 3, []string{"Steve"}, false},
		{"equals form", []string{"Steve", "--level=2"}, 4, 2, []string{"Steve"}, false},
		{"missing value errors", []string{"Steve", "--level"}, 4, 0, nil, true},
		{"non-numeric value errors", []string{"Steve", "--level", "nope"}, 4, 0, nil, true},
		{"non-numeric equals value errors", []string{"Steve", "--level=nope"}, 4, 0, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVal, gotRest, err := extractIntFlag(tc.args, "level", tc.def)
			if tc.wantErr {
				if err == nil {
					t.Fatal("extractIntFlag() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("extractIntFlag() error = %v", err)
			}
			if gotVal != tc.wantVal {
				t.Errorf("val = %d, want %d", gotVal, tc.wantVal)
			}
			if !reflect.DeepEqual(gotRest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", gotRest, tc.wantRest)
			}
		})
	}
}
