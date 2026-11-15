package query

import "testing"

func TestDetailedSelection(t *testing.T) {
	for _, tc := range []struct {
		body, path       string
		present, invalid bool
	}{
		{`null`, `$`, true, false}, {`{"x":null}`, `$.x`, true, false},
		{`{"items":[{"x":""}]}`, `$.items[0].x`, true, false},
		{`{}`, `$.x`, false, false}, {`[]`, `$[0]`, false, false},
		{`1`, `$.x`, false, true}, {`"x"`, `$.x`, false, true},
		{`true`, `$.x`, false, true}, {`null`, `$.x`, false, true},
		{`[]`, `$.x`, false, true}, {`{}`, `$[0]`, false, true},
	} {
		t.Run(tc.body+tc.path, func(t *testing.T) {
			p, err := Parse(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			document, err := Decode([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			_, present, err := p.SelectValue(document)
			if present != tc.present || (err != nil) != tc.invalid {
				t.Fatalf("present=%v err=%v, want %v/%v", present, err, tc.present, tc.invalid)
			}
			_, capturePresent, err := p.Select([]byte(tc.body))
			if err != nil || capturePresent != tc.present {
				t.Fatalf("capture present=%v err=%v", capturePresent, err)
			}
		})
	}
	for _, source := range []string{"", " ", "{", "{} []", "null null"} {
		if _, err := Decode([]byte(source)); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
