GET http://example.invalid/capture
HTTP 200
[Captures]
own: jsonpath "$"
[Asserts]
jsonpath "$" == {{own}}
GET http://example.invalid/health
HTTP 200
