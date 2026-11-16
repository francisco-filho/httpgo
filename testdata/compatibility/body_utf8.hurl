GET http://example.invalid/capture
HTTP 200
[Asserts]
body contains "ação 雪 �"
GET http://example.invalid/health
HTTP 200
