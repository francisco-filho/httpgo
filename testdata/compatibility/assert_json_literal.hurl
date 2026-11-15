GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.x" == "{{name}}"
GET http://example.invalid/health
HTTP 200
