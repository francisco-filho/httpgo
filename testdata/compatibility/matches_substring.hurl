GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.name" matches "Pizza"
