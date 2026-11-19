GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(missing)" exists
[Captures]
x: regex "(missing)"
GET http://example.invalid/health
HTTP 200
