GET http://example.invalid/health
HTTP 200
GET http://example.invalid/health
[Options]
variable: name="bad\q"
variable: name=valid
HTTP 200
