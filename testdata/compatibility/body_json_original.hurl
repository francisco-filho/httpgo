GET http://example.invalid/capture
HTTP 200
[Asserts]
body contains "\\u0061"
body not contains "a"
GET http://example.invalid/health
HTTP 200
