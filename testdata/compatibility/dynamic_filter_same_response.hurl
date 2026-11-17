GET http://example.invalid/capture
HTTP 200
[Captures]
own: jsonpath "$[0].v"
[Asserts]
jsonpath "$[?(@.v=={{own}})].id" == 1
GET http://example.invalid/health
HTTP 200
