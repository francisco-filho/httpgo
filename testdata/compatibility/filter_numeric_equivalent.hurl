GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==1.0)].id" count == 2

GET http://example.invalid/health
HTTP 200
