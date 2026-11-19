GET http://example.invalid/capture
HTTP 200
[Captures]
otp_code: regex "verificação é: ([0-9]{6})"
POST http://example.invalid/health
Content-Type: application/json
{"code":"{{otp_code}}"}
HTTP 200
