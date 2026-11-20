POST http://example.invalid/health
Content-Type: application/json
[Options]
variable: n=9007199254740993
variable: decimal=1.000000000000000001
{"n":{{n}},"decimal":{{decimal}}}
HTTP 200
