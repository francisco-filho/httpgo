GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "id=(.*)" == "{{id}}"
[Captures]
id: regex "id=(.*)"
GET http://example.invalid/health
HTTP 200
