POST http://example.invalid/health
Content-Type: application/json
[Options]
variable: ref={{other}}
variable: env=${NAME}
variable: function=newUuid()
{"ref":"{{ref}}","env":"{{env}}","function":"{{function}}"}
HTTP 200
