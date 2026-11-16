GET http://example.invalid/capture
HTTP 200
[Asserts]
body contains "a b"
GET http://example.invalid/health
HTTP 200
