GET http://example.invalid/health
HTTP 200
POST http://example.invalid/health
file,payloads/missing.txt;
HTTP 200
GET http://example.invalid/missing
