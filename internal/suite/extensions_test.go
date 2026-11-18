package suite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverScenarioExtensions(t *testing.T) {
	for _, names := range [][]string{{"01.httpgo", "nested/02.httpgo"}, {"same.httpgo", "same.hurl"}} {
		t.Run(names[0], func(t *testing.T) {
			dir := t.TempDir()
			var want []string
			for _, name := range names {
				want = append(want, fixture(t, dir, name, "GET http://example.test/health\nHTTP 200"))
			}
			fixture(t, dir, "helper.json", "not a scenario")
			fixture(t, dir, "helper.HTTPGO", "not a scenario")
			for _, paths := range [][]string{{dir}, {want[1], dir, want[0], filepath.Join(dir, ".")}} {
				files, err := New(nil).Discover(context.Background(), paths)
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, file := range files {
					got = append(got, file.Path)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("paths=%v got=%v want=%v", paths, got, want)
				}
			}
		})
	}
}
