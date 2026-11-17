GET http://example.test/first
GET http://example.test/late
HTTP *
[Asserts]
jsonpath "$" isInteger 3
