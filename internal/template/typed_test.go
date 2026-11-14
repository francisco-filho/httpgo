package template

import (
	"strings"
	"testing"
)

func TestTypedText(t *testing.T) {
	for _, tt := range []struct {
		input, want string
		rejected    bool
	}{
		{`"001"`, "001", false}, {`"a\n\"\\雪"`, "a\n\"\\雪", false},
		{"", "", false}, {"a=b=c", "a=b=c", false}, {"001", "001", false},
		{"true", "true", false}, {"null", "null", false}, {`"null"`, "null", false},
		{"9007199254740993123456789", "9007199254740993123456789", false},
		{"1.2300e+99", "1.2300e+99", false}, {" false ", "false", false},
		{`"{{missing}}"`, "{{missing}}", false},
		{`[1,9007199254740993123456789]`, "", true}, {`{"name":"Pizza"}`, "", true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			c, err := New([]string{"x=" + tt.input}, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Render("prefix{{x}}suffix")
			if tt.rejected {
				if err == nil || strings.Contains(err.Error(), tt.input) {
					t.Fatalf("unsafe/missing rejection: %v", err)
				}
				return
			}
			if err != nil || got.Value != "prefix"+tt.want+"suffix" {
				t.Fatalf("got=%q err=%v want=%q", got.Value, err, tt.want)
			}
		})
	}
}

func TestOpaqueObjectsAndConflicts(t *testing.T) {
	c, err := New([]string{`product={"name":"Pizza"}`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Render("{{product.name}}"); err == nil {
		t.Fatal("object properties became variables")
	}
	c, err = New([]string{"product.name=Pizza"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.Render("{{product.name}}"); err != nil || got.Value != "Pizza" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for _, args := range [][]string{
		{`product={"name":"Pizza"}`, "product.name=Pizza"},
		{"product.name=Pizza", `product={"name":"Pizza"}`},
		{"product=null", "product.name=Pizza"}, {"product=[]", "product.name=Pizza"},
	} {
		if _, err := New(args, nil); err == nil {
			t.Fatalf("accepted conflict %q", args)
		}
	}
}
