POST http://example.invalid/documents
[MultipartFormData]
a: file,payloads/raw.txt; text/plain
b: file,payloads/empty.txt; text/plain
HTTP 200
