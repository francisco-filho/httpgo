GET http://example.invalid/capture
HTTP 200
[Captures]
signup_code: regex "verificação é: ([0-9]{6})"
[Asserts]
regex "verificação é: ([0-9]{6})" matches "^[0-9]{6}$"
POST http://example.invalid/health
Content-Type: application/json
{"code":"{{signup_code}}"}
HTTP 200
