GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.x==1)].v" count == 1

GET http://example.invalid/health
HTTP 200
