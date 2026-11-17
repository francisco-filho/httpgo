GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.x[0]==1)]" exists

GET http://example.invalid/health
HTTP 200
