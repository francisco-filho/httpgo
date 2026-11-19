GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=='it\\'s pizza')].id" == "selected"

GET http://example.invalid/health
HTTP 200
