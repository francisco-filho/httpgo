package config

import "testing"

func TestPlainNumbersBeyondMachineRangeKeepType(t *testing.T) {
	settings, err := decode("precision.yaml", []byte(`version: 1
variables:
  large: 1e400
  tiny: 1e-400
  integer: 1234567890123456789012345678901234567890
  array: [1e400, 1e-400, 1234567890123456789012345678901234567890]
  quoted: "1e400"
  dot: .
  sign: +
  underscore: _1
`))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"large": "1e400", "tiny": "1e-400", "integer": "1234567890123456789012345678901234567890", "array": "[1e400,1e-400,1234567890123456789012345678901234567890]", "quoted": `"1e400"`, "dot": `"."`, "sign": `"+"`, "underscore": `"_1"`} {
		if got := string(settings.Variables[name]); got != want {
			t.Errorf("%s got=%s want=%s", name, got, want)
		}
	}
}
