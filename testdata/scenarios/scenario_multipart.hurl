# Token fictício; não forneça credenciais reais em argumentos.
POST {{base_url}}/documents
Authorization: Bearer {{token}}
[MultipartFormData]
description: Documento de teste
category: {{category}}
document: file,../payloads/document.pdf; application/pdf
HTTP 201
