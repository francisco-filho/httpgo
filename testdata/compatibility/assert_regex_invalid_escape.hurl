GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "\d" exists
GET http://example.invalid/health
HTTP 200
