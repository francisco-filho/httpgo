GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=={{v}})].id" == 1

GET http://example.invalid/health
HTTP 200
