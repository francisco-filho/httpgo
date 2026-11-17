GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.slug=='pizza')].active" == true
[Captures]
id: jsonpath "$[?(@.slug=='pizza')].id"

GET http://example.invalid/health/{{id}}
HTTP 200
