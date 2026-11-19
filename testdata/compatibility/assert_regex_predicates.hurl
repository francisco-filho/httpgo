GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "id=(.*)" == "123456"
regex "id=(.*)" contains "234"
regex "id=(.*)" not contains "789"
regex "id=(.*)" matches "^[0-9]{6}$"
regex "id=(.*)" exists
GET http://example.invalid/health
HTTP 200
