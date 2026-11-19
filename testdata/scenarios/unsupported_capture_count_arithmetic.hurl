GET http://example.invalid/first
HTTP 200

GET http://example.invalid/late
HTTP 200
[Captures]
items: jsonpath "$.items" count + 1
