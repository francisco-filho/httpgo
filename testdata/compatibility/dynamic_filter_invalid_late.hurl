GET http://example.invalid/health
HTTP 200
GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=='{{broken')].id" exists

GET http://example.invalid/health
HTTP 200
